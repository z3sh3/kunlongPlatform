package controller

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/objectstorage"

	"github.com/gin-gonic/gin"
)

// allowedMediaTypes maps a permitted extension to the content-type prefix a
// browser must see, so an uploaded file cannot claim to be an image and be
// served as a script.
var allowedMediaTypes = map[string]string{
	".jpg":  "image/",
	".jpeg": "image/",
	".png":  "image/",
	".webp": "image/",
	".gif":  "image/",
	".mp3":  "audio/",
	".wav":  "audio/",
	".m4a":  "audio/",
	".mp4":  "video/",
	".webm": "video/",
	".mov":  "video/",
}

// UploadMedia stores one uploaded file in the object store and returns its URL.
//
// Objects are laid out as `<module>/<yyyyMM>/<random>.<ext>`, which keeps every
// module's media in its own month-partitioned prefix.
func UploadMedia(c *gin.Context) {
	cfg := objectstorage.LoadConfig()
	if err := cfg.Validate(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		common.ApiErrorMsg(c, "cannot parse the upload")
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		common.ApiErrorMsg(c, "a file is required")
		return
	}
	header := files[0]
	if header.Size <= 0 {
		common.ApiErrorMsg(c, "the uploaded file is empty")
		return
	}
	if header.Size > cfg.MaxBytes {
		common.ApiErrorMsg(c, "the file exceeds the configured upload limit")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	requiredPrefix, ok := allowedMediaTypes[ext]
	if !ok {
		common.ApiErrorMsg(c, "unsupported file type")
		return
	}

	file, err := header.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()

	// Sniff the real content so a renamed script cannot pass as an image.
	head := make([]byte, 512)
	read, err := io.ReadFull(file, head)
	if err != nil && read == 0 {
		common.ApiErrorMsg(c, "the uploaded file is unreadable")
		return
	}
	detected := http.DetectContentType(head[:read])
	if !strings.HasPrefix(detected, requiredPrefix) {
		common.ApiErrorMsg(c, "the file content does not match its extension")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		common.ApiError(c, err)
		return
	}

	module := strings.TrimSpace(c.PostForm("module"))
	key, err := objectstorage.BuildObjectKey(module, randomName(ext))
	if err != nil {
		common.ApiErrorMsg(c, "invalid storage module")
		return
	}

	client := objectstorage.New(cfg)
	if err := client.Put(c.Request.Context(), key, detected, file, header.Size); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"key":          key,
		"url":          cfg.PublicURL(key),
		"size":         header.Size,
		"content_type": detected,
	})
}

// randomName returns a random hex file name, keeping uploads unguessable.
func randomName(ext string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "upload" + ext
	}
	return hex.EncodeToString(buf) + ext
}

// GetObjectStorageConfig reports whether uploads are possible, without exposing
// the secret key.
func GetObjectStorageConfig(c *gin.Context) {
	cfg := objectstorage.LoadConfig()
	common.ApiSuccess(c, gin.H{
		"configured":      cfg.Validate() == nil,
		"provider":        cfg.Provider,
		"region":          cfg.Region,
		"bucket":          cfg.Bucket,
		"scheme":          cfg.Scheme,
		"public_base_url": cfg.PublicBaseURL,
		"max_mb":          cfg.MaxBytes >> 20,
		"secret_id_set":   cfg.SecretId != "",
		"secret_key_set":  cfg.SecretKey != "",
		"allowed_types":   allowedMediaExtensions(),
	})
}

// VerifyObjectStorage checks the credentials against the bucket.
func VerifyObjectStorage(c *gin.Context) {
	cfg := objectstorage.LoadConfig()
	if err := cfg.Validate(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := objectstorage.New(cfg).HeadBucket(c.Request.Context()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"ok": true, "bucket": cfg.Bucket, "region": cfg.Region})
}

// allowedMediaExtensions lists the accepted extensions for the console.
func allowedMediaExtensions() []string {
	extensions := make([]string, 0, len(allowedMediaTypes))
	for ext := range allowedMediaTypes {
		extensions = append(extensions, ext)
	}
	slices.Sort(extensions)
	return extensions
}
