// Package objectstorage stores user-uploaded media (cover images, audio, video)
// in an object store and returns a public URL.
//
// Only the Tencent COS provider is implemented. The request signing below follows
// COS's documented algorithm and is written against the standard library so the
// gateway does not depend on a vendor SDK.
package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ProviderTencent identifies Tencent Cloud COS.
const ProviderTencent = "tencent"

// DefaultMaxBytes caps one uploaded object (10 MiB).
const DefaultMaxBytes int64 = 10 << 20

// modulePattern keeps the module segment a single safe path component, so an
// object key can never escape its module prefix.
var modulePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

// ErrNotConfigured is returned while the bucket or credentials are missing.
var ErrNotConfigured = errors.New("object storage is not configured")

// Config is the resolved object storage configuration.
type Config struct {
	Provider      string
	Region        string
	Bucket        string
	Scheme        string
	SecretId      string
	SecretKey     string
	PublicBaseURL string
	MaxBytes      int64
}

// Validate reports whether uploads can be attempted.
func (c Config) Validate() error {
	if c.Provider != ProviderTencent {
		return fmt.Errorf("%w: unsupported provider %q", ErrNotConfigured, c.Provider)
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return fmt.Errorf("%w: bucket is empty", ErrNotConfigured)
	}
	if strings.TrimSpace(c.Region) == "" {
		return fmt.Errorf("%w: region is empty", ErrNotConfigured)
	}
	if strings.TrimSpace(c.SecretId) == "" || strings.TrimSpace(c.SecretKey) == "" {
		return fmt.Errorf("%w: credentials are missing", ErrNotConfigured)
	}
	return nil
}

// BuildObjectKey lays out an object as `<module>/<yyyyMM>/<name>`.
//
// The month segment keeps buckets browsable without unbounded flat prefixes, and
// the caller supplies only a module name and an extension, so the key cannot
// contain traversal segments.
func BuildObjectKey(module, name string) (string, error) {
	if !modulePattern.MatchString(module) {
		return "", fmt.Errorf("invalid module %q", module)
	}
	clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "."))
	if clean == "" {
		return "", errors.New("empty file name")
	}
	return fmt.Sprintf("%s/%s/%s", module, time.Now().Format("200601"), clean), nil
}

// PublicURL returns the address a browser should load for a stored object.
func (c Config) PublicURL(key string) string {
	base := strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	if base != "" {
		return base + "/" + escapeKey(key)
	}
	return fmt.Sprintf("%s://%s.cos.%s.myqcloud.com/%s", c.Scheme, c.Bucket, c.Region, escapeKey(key))
}

// escapeKey percent-encodes each path segment but keeps the separators.
func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(url.PathEscape(part), "+", "%20")
	}
	return strings.Join(parts, "/")
}

// TencentCOS talks to one COS bucket.
type TencentCOS struct {
	cfg    Config
	client *http.Client
}

// New builds a client for the given configuration.
func New(cfg Config) *TencentCOS {
	if cfg.Scheme == "" {
		cfg.Scheme = "https"
	}
	return &TencentCOS{cfg: cfg, client: &http.Client{Timeout: 60 * time.Second}}
}

// host is the bucket endpoint for the configured region.
func (c *TencentCOS) host() string {
	return fmt.Sprintf("%s.cos.%s.myqcloud.com", c.cfg.Bucket, c.cfg.Region)
}

// Put uploads one object and returns its key.
func (c *TencentCOS) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	escaped := "/" + escapeKey(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s://%s%s", c.cfg.Scheme, c.host(), escaped), body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Host", c.host())
	req.Header.Set("Authorization", c.authorization(http.MethodPut, escaped, req.Header))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("object storage rejected the upload: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

// HeadBucket verifies the credentials and bucket without writing anything.
func (c *TencentCOS) HeadBucket(ctx context.Context) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead,
		fmt.Sprintf("%s://%s/", c.cfg.Scheme, c.host()), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Host", c.host())
	req.Header.Set("Authorization", c.authorization(http.MethodHead, "/", req.Header))
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach object storage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("object storage rejected the credentials: HTTP %d", resp.StatusCode)
	}
	return nil
}

// authorization builds the COS `q-sign-algorithm=sha1` header.
func (c *TencentCOS) authorization(method, pathname string, header http.Header) string {
	now := time.Now()
	keyTime := fmt.Sprintf("%d;%d", now.Add(-60*time.Second).Unix(), now.Add(time.Hour).Unix())

	names := make([]string, 0, len(header))
	for name := range header {
		lower := strings.ToLower(name)
		if lower == "authorization" || lower == "content-length" {
			continue
		}
		names = append(names, lower)
	}
	sortStrings(names)

	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+urlEncode(strings.ToLower(header.Get(name))))
	}
	headerList := strings.Join(names, ";")
	canonicalHeaders := strings.Join(pairs, "&")

	httpString := strings.ToLower(method) + "\n" + pathname + "\n" + "" + "\n" + canonicalHeaders + "\n"
	signature := sign(c.cfg.SecretKey, keyTime, httpString, headerList)

	return "q-sign-algorithm=sha1" +
		"&q-ak=" + c.cfg.SecretId +
		"&q-sign-time=" + keyTime +
		"&q-key-time=" + keyTime +
		"&q-header-list=" + headerList +
		"&q-url-param-list=" +
		"&q-signature=" + signature
}
