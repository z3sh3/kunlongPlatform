package datasetmarket

import (
	"embed"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"

	"github.com/gin-gonic/gin"
)

// Bundled demonstration datasets for the data marketplace.
//
// Three listings, each covering a different capability dimension (domain
// knowledge, code/function calling, multi-step reasoning). The files ship inside
// the binary so an administrator can publish them with one request on a fresh
// deployment, and they are served from an exact-key allowlist below so the model
// training console can reference them by URL.
//
//go:embed demo/manifest.json demo/*.jsonl
var demoDataFS embed.FS

// demoFileRoutePrefix is stored as the listing's file URL path.
const demoFileRoutePrefix = "/api/data-market/demo-datasets/"

// demoMarketDataset is one bundled listing description.
type demoMarketDataset struct {
	Key         string `json:"key"`
	File        string `json:"file"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Tags        string `json:"tags"`
	Rows        int    `json:"rows"`
	SizeBytes   int64  `json:"size_bytes"`
}

// demoMarketState reports whether one bundled listing is already published.
type demoMarketState struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Tags     string `json:"tags"`
	Rows     int    `json:"rows"`
	Exists   bool   `json:"exists"`
}

type demoSeedResult struct {
	Created  []Dataset `json:"created"`
	Existing []string  `json:"existing"`
}

var (
	demoManifestOnce sync.Once
	demoManifest     []demoMarketDataset
	demoManifestErr  error
)

// loadDemoManifest parses the embedded catalogue exactly once.
func loadDemoManifest() ([]demoMarketDataset, error) {
	demoManifestOnce.Do(func() {
		raw, err := demoDataFS.ReadFile("demo/manifest.json")
		if err != nil {
			demoManifestErr = err
			return
		}
		var parsed []demoMarketDataset
		if err := common.Unmarshal(raw, &parsed); err != nil {
			demoManifestErr = err
			return
		}
		demoManifest = parsed
	})
	return demoManifest, demoManifestErr
}

// ListDemoDatasets reports the bundled listings and whether each is published.
func ListDemoDatasets(c *gin.Context) {
	entries, err := loadDemoManifest()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	published, err := publishedDemoNames(entries)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]demoMarketState, 0, len(entries))
	for _, entry := range entries {
		items = append(items, demoMarketState{
			Key:      entry.Key,
			Name:     entry.Name,
			Category: entry.Category,
			Tags:     entry.Tags,
			Rows:     entry.Rows,
			Exists:   published[entry.Name],
		})
	}
	common.ApiSuccess(c, items)
}

// SeedDemoDatasets publishes the bundled listings. It is idempotent: a listing
// whose name already exists is reported and left untouched.
func SeedDemoDatasets(c *gin.Context) {
	entries, err := loadDemoManifest()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	published, err := publishedDemoNames(entries)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	baseURL := demoPublicBaseURL(c)
	result := demoSeedResult{Created: []Dataset{}, Existing: []string{}}

	pending := make([]Dataset, 0, len(entries))
	for _, entry := range entries {
		if published[entry.Name] {
			result.Existing = append(result.Existing, entry.Key)
			continue
		}
		pending = append(pending, Dataset{
			UserId:      c.GetInt("id"),
			Name:        entry.Name,
			Description: entry.Description,
			Category:    entry.Category,
			Format:      "jsonl",
			Tags:        entry.Tags,
			FileUrl:     baseURL + demoFileRoutePrefix + entry.Key + "/file",
			SizeBytes:   entry.SizeBytes,
			PriceMode:   shared.PriceModeFree,
			Status:      shared.PublishStatusPublished,
		})
	}
	if len(pending) > 0 {
		if err := model.DB.Create(&pending).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		result.Created = pending
	}
	common.ApiSuccess(c, result)
}

// ServeDemoDatasetFile streams one bundled file. It is intentionally reachable
// without a session: the training service downloads listings by URL. Only exact
// keys from the embedded manifest are served, so no path can escape the binary.
func ServeDemoDatasetFile(c *gin.Context) {
	entries, err := loadDemoManifest()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	key := c.Param("key")
	for _, entry := range entries {
		if entry.Key != key {
			continue
		}
		data, readErr := demoDataFS.ReadFile("demo/" + entry.File)
		if readErr != nil {
			common.ApiError(c, readErr)
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+entry.File+`"`)
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "demo dataset not found"})
}

// publishedDemoNames returns the names already used by bundled listings.
func publishedDemoNames(entries []demoMarketDataset) (map[string]bool, error) {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	var found []string
	if err := model.DB.Model(&Dataset{}).Where("name IN ?", names).Pluck("name", &found).Error; err != nil {
		return nil, err
	}
	published := make(map[string]bool, len(found))
	for _, name := range found {
		published[name] = true
	}
	return published, nil
}

// demoPublicBaseURL builds the absolute origin stored in the listing file URL.
// A relative path would be rejected by the training console, which requires an
// absolute http(s) URL for remote datasets.
func demoPublicBaseURL(c *gin.Context) string {
	common.OptionMapRWMutex.RLock()
	serverAddress := strings.TrimRight(strings.TrimSpace(common.OptionMap["ServerAddress"]), "/")
	common.OptionMapRWMutex.RUnlock()
	if serverAddress != "" {
		return serverAddress
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
