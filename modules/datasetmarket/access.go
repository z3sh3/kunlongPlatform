package datasetmarket

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"

	"github.com/gin-gonic/gin"
)

// Dataset access and preview.
//
// Applying for a dataset issues a grant carrying its own access key. The holder
// of that key can download the data from a stable URL without a console session,
// which is what a training job or a script needs. Preview lets a browser show the
// actual rows of a bundled dataset before applying.

const (
	// demoPreviewRowLimit bounds how many rows a preview returns.
	demoPreviewRowLimit = 20
	// accessKeyPattern keeps the key a safe, single path segment.
	accessKeyPattern = `^[A-Za-z0-9_-]{16,128}$`
	// accessFileRoutePrefix is the path component of the issued access URL.
	accessFileRoutePrefix = "/api/data-market/access/"
)

var (
	errGrantNotApproved = errors.New("access has not been approved")
	errGrantExpired     = errors.New("access has expired")
	errGrantNotFound    = errors.New("access key not found")
	accessKeyRegexp     = regexp.MustCompile(accessKeyPattern)
)

// datasetPreview is the payload behind the preview dialog.
type datasetPreview struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason,omitempty"`
	Columns   []string         `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	TotalRows int64            `json:"total_rows"`
	Truncated bool             `json:"truncated"`
}

// issueAccessKey generates a key that is not used by any other grant yet.
func issueAccessKey() (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		key, err := common.GenerateKey()
		if err != nil {
			return "", err
		}
		var count int64
		if err := model.DB.Model(&DatasetGrant{}).Where("access_key = ?", key).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return key, nil
		}
	}
	return "", errors.New("could not issue a unique access key")
}

// grantAccessURL is the download URL presented to the user after approval.
func grantAccessURL(c *gin.Context, key string) string {
	if key == "" {
		return ""
	}
	return demoPublicBaseURL(c) + accessFileRoutePrefix + key + "/file"
}

// ServeGrantFile hands out a dataset to the holder of an access key.
//
// No console session is required: the key is the credential, so a training job
// or an external script can fetch the data directly. Bundled demo datasets are
// served from the binary; any other dataset redirects to its stored file URL
// once the grant has been validated.
func ServeGrantFile(c *gin.Context) {
	key := c.Param("key")
	if !accessKeyRegexp.MatchString(key) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "access key not found"})
		return
	}

	var grant DatasetGrant
	err := model.DB.Preload("Dataset").Where("access_key = ?", key).First(&grant).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": errGrantNotFound.Error()})
		return
	}
	if grant.Status != shared.ReviewStatusApproved {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": errGrantNotApproved.Error()})
		return
	}
	if grant.ExpiresAt > 0 && grant.ExpiresAt < time.Now().Unix() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": errGrantExpired.Error()})
		return
	}

	if demoKey, ok := demoKeyFromFileURL(grant.Dataset.FileUrl); ok {
		fileName, data, ok := readDemoFile(demoKey)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "dataset file is not available"})
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}
	if strings.TrimSpace(grant.Dataset.FileUrl) == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "dataset has no file to serve"})
		return
	}
	c.Redirect(http.StatusFound, grant.Dataset.FileUrl)
}

// PreviewDataset returns the first rows of a dataset so the console can show the
// real content instead of only metadata.
func PreviewDataset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid dataset id")
		return
	}
	var dataset Dataset
	if err := model.DB.First(&dataset, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	isOwner := dataset.UserId == c.GetInt("id")
	if dataset.Status != shared.PublishStatusPublished && !isOwner && c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorMsg(c, "no permission to preview this dataset")
		return
	}

	demoKey, ok := demoKeyFromFileURL(dataset.FileUrl)
	if !ok {
		common.ApiSuccess(c, datasetPreview{
			Available: false,
			Reason:    "preview is available for bundled demo datasets",
			Columns:   []string{},
			Rows:      []map[string]any{},
		})
		return
	}

	rows, total, err := readDemoRows(demoKey, demoPreviewRowLimit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	columns := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, row := range rows {
		for key := range row {
			if !seen[key] {
				seen[key] = true
				columns = append(columns, key)
			}
		}
	}
	common.ApiSuccess(c, datasetPreview{
		Available: true,
		Columns:   columns,
		Rows:      rows,
		TotalRows: total,
		Truncated: total > int64(len(rows)),
	})
}

// demoKeyFromFileURL extracts the bundled dataset key from a listing file URL.
func demoKeyFromFileURL(fileURL string) (string, bool) {
	index := strings.Index(fileURL, demoFileRoutePrefix)
	if index < 0 {
		return "", false
	}
	rest := fileURL[index+len(demoFileRoutePrefix):]
	key, _, _ := strings.Cut(rest, "/")
	if key == "" {
		return "", false
	}
	return key, true
}

// readDemoFile returns the file name and bytes of one bundled dataset.
func readDemoFile(demoKey string) (string, []byte, bool) {
	entries, err := loadDemoManifest()
	if err != nil {
		return "", nil, false
	}
	for _, entry := range entries {
		if entry.Key != demoKey {
			continue
		}
		data, readErr := demoDataFS.ReadFile("demo/" + entry.File)
		if readErr != nil {
			return "", nil, false
		}
		return entry.File, data, true
	}
	return "", nil, false
}

// readDemoRows parses the first `limit` JSONL rows and counts the rest.
func readDemoRows(demoKey string, limit int) ([]map[string]any, int64, error) {
	_, data, ok := readDemoFile(demoKey)
	if !ok {
		return nil, 0, errors.New("bundled dataset not found")
	}
	var rows []map[string]any
	var total int64
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		total++
		if len(rows) >= limit {
			continue
		}
		var row map[string]any
		if err := common.UnmarshalJsonStr(trimmed, &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return rows, total, nil
}
