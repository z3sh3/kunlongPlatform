package objectstorage

import (
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Option keys of the object storage integration. ObjectStorageSecretKey ends with
// "Key" on purpose: controller.GetOptions filters such keys, which keeps the
// secret write-only.
const (
	OptionProvider      = "ObjectStorageProvider"
	OptionRegion        = "ObjectStorageRegion"
	OptionBucket        = "ObjectStorageBucket"
	OptionScheme        = "ObjectStorageScheme"
	OptionSecretId      = "ObjectStorageSecretId"
	OptionSecretKey     = "ObjectStorageSecretKey"
	OptionPublicBaseURL = "ObjectStoragePublicBaseURL"
	OptionMaxMB         = "ObjectStorageMaxMB"
)

// LoadConfig resolves the configuration from the option map, falling back to
// environment variables so an operator can keep credentials out of the database.
func LoadConfig() Config {
	common.OptionMapRWMutex.RLock()
	cfg := Config{
		Provider:      optionOrDefault(OptionProvider, "tencent"),
		Region:        strings.TrimSpace(common.OptionMap[OptionRegion]),
		Bucket:        optionOrDefault(OptionBucket, ""),
		Scheme:        optionOrDefault(OptionScheme, "https"),
		SecretId:      optionOrDefault(OptionSecretId, ""),
		SecretKey:     optionOrDefault(OptionSecretKey, ""),
		PublicBaseURL: optionOrDefault(OptionPublicBaseURL, ""),
		MaxBytes:      optionMaxBytes(OptionMaxMB),
	}
	common.OptionMapRWMutex.RUnlock()

	if cfg.SecretId == "" {
		cfg.SecretId = firstEnv("OBJECT_STORAGE_SECRET_ID", "COS_SECRET_ID")
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = firstEnv("OBJECT_STORAGE_SECRET_KEY", "COS_SECRET_KEY")
	}
	if cfg.Bucket == "" {
		cfg.Bucket = firstEnv("OBJECT_STORAGE_BUCKET", "COS_BUCKET")
	}
	if cfg.Region == "" {
		cfg.Region = firstEnv("OBJECT_STORAGE_REGION", "COS_REGION")
	}
	if cfg.Region == "" {
		cfg.Region = "ap-nanjing"
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "https"
	}
	return cfg
}

func optionOrDefault(key, fallback string) string {
	value := strings.TrimSpace(common.OptionMap[key])
	if value == "" {
		return fallback
	}
	return value
}

func optionMaxBytes(key string) int64 {
	raw := strings.TrimSpace(common.OptionMap[key])
	if raw == "" {
		return DefaultMaxBytes
	}
	megabytes, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || megabytes <= 0 {
		return DefaultMaxBytes
	}
	return megabytes << 20
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
