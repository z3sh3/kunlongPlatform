package modelsquare

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"path"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// demoDataFS ships the bundled demonstration datasets inside the binary so the
// console can install them without any file drop or out-of-band script. The
// training-service demo script reads the same manifest, which keeps a single
// source of truth for names, formats and suggested pipelines.
//
//go:embed demo/manifest.json demo/sft demo/raw
var demoDataFS embed.FS

// demoDataset is one bundled demonstration dataset.
type demoDataset struct {
	Key               string `json:"key"`
	File              string `json:"file"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	SuggestedPipeline string `json:"suggested_pipeline"`
	Format            string `json:"format"`
	Rows              int    `json:"rows"`
}

// demoDatasetInfo adds the per-user installation state.
type demoDatasetInfo struct {
	Key               string `json:"key"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	SuggestedPipeline string `json:"suggested_pipeline"`
	Format            string `json:"format"`
	Rows              int    `json:"rows"`
	Installed         bool   `json:"installed"`
}

type installDemoRequest struct {
	// Keys selects a subset; an empty list installs everything.
	Keys []string `json:"keys"`
}

type demoInstallFailure struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

type demoInstallResult struct {
	Installed []TrainingDataset    `json:"installed"`
	Skipped   []string             `json:"skipped"`
	Failed    []demoInstallFailure `json:"failed"`
}

var (
	demoManifestOnce sync.Once
	demoManifest     []demoDataset
	demoManifestErr  error
)

// loadDemoManifest parses the embedded catalogue exactly once.
func loadDemoManifest() ([]demoDataset, error) {
	demoManifestOnce.Do(func() {
		raw, err := demoDataFS.ReadFile("demo/manifest.json")
		if err != nil {
			demoManifestErr = err
			return
		}
		var parsed []demoDataset
		if err := common.Unmarshal(raw, &parsed); err != nil {
			demoManifestErr = err
			return
		}
		demoManifest = parsed
	})
	return demoManifest, demoManifestErr
}

// demoCatalogue reports the bundled datasets together with whether installing
// them is possible right now. The console uses `configured` to explain the
// missing training service instead of letting the user hit a bare error.
type demoCatalogue struct {
	Configured bool              `json:"configured"`
	Datasets   []demoDatasetInfo `json:"datasets"`
}

// ListDemoDatasets returns the bundled datasets and whether the caller already
// installed each of them. It never talks to the training service, so the console
// can show the catalogue (and the reason it cannot be installed yet) even before
// the integration is configured.
func ListDemoDatasets(c *gin.Context) {
	entries, err := loadDemoManifest()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	installedNames, err := installedDatasetNames(c.GetInt("id"), entries)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]demoDatasetInfo, 0, len(entries))
	for _, entry := range entries {
		items = append(items, demoDatasetInfo{
			Key:               entry.Key,
			Name:              entry.Name,
			Description:       entry.Description,
			SuggestedPipeline: entry.SuggestedPipeline,
			Format:            entry.Format,
			Rows:              entry.Rows,
			Installed:         installedNames[entry.Name],
		})
	}
	cfg, cfgErr := loadTrainingConfig()
	common.ApiSuccess(c, demoCatalogue{
		Configured: cfgErr == nil && cfg.IsEnabled,
		Datasets:   items,
	})
}

// installedDatasetNames reports which of the demo datasets the user already has.
func installedDatasetNames(userId int, entries []demoDataset) (map[string]bool, error) {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	var found []string
	if err := model.DB.Model(&TrainingDataset{}).
		Where("user_id = ? AND name IN ?", userId, names).
		Pluck("name", &found).Error; err != nil {
		return nil, err
	}
	installed := make(map[string]bool, len(found))
	for _, name := range found {
		installed[name] = true
	}
	return installed, nil
}

// InstallDemoDatasets uploads the selected bundled datasets to the training
// service and records them for the caller. Re-running it is safe: datasets that
// already exist are skipped, and one bad file does not abort the rest.
func InstallDemoDatasets(c *gin.Context) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	entries, err := loadDemoManifest()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req installDemoRequest
	// The body is optional: an empty request installs the whole catalogue.
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		common.ApiError(c, bindErr)
		return
	}
	selected, selectErr := selectDemoDatasets(entries, req.Keys)
	if selectErr != nil {
		common.ApiError(c, selectErr)
		return
	}

	installedNames, err := installedDatasetNames(c.GetInt("id"), entries)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	client := newTrainingClient(cfg)
	result := demoInstallResult{
		Installed: []TrainingDataset{},
		Skipped:   []string{},
		Failed:    []demoInstallFailure{},
	}
	for _, entry := range selected {
		if installedNames[entry.Name] {
			result.Skipped = append(result.Skipped, entry.Key)
			continue
		}
		data, readErr := demoDataFS.ReadFile("demo/" + entry.File)
		if readErr != nil {
			result.Failed = append(result.Failed, demoInstallFailure{Key: entry.Key, Message: "bundled file is missing"})
			continue
		}
		dataset, uploadErr := client.uploadDataset(
			c.Request.Context(), entry.Name, path.Base(entry.File), bytes.NewReader(data),
		)
		if uploadErr != nil {
			result.Failed = append(result.Failed, demoInstallFailure{Key: entry.Key, Message: uploadErr.Error()})
			continue
		}
		// The console owns the display name: do not depend on the service echoing
		// the multipart `name` field, and keep names unique per user so re-running
		// the installer recognises what is already there.
		dataset.Name = entry.Name
		record, createErr := storeDatasetRecord(c.GetInt("id"), dataset, DatasetSourceUpload, "demo:"+entry.Key)
		if createErr != nil {
			result.Failed = append(result.Failed, demoInstallFailure{Key: entry.Key, Message: createErr.Error()})
			continue
		}
		result.Installed = append(result.Installed, record)
	}
	common.ApiSuccess(c, result)
}

// selectDemoDatasets resolves the requested keys, rejecting unknown ones instead
// of silently ignoring a typo.
func selectDemoDatasets(entries []demoDataset, keys []string) ([]demoDataset, error) {
	if len(keys) == 0 {
		return entries, nil
	}
	byKey := make(map[string]demoDataset, len(entries))
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	selected := make([]demoDataset, 0, len(keys))
	for _, key := range keys {
		entry, ok := byKey[key]
		if !ok {
			return nil, errors.New("unknown demo dataset: " + key)
		}
		selected = append(selected, entry)
	}
	return selected, nil
}
