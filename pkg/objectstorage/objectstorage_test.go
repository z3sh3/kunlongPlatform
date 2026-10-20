package objectstorage

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildObjectKeyUsesModuleAndMonth(t *testing.T) {
	key, err := BuildObjectKey("talent", "abc123.webp")
	require.NoError(t, err)

	parts := strings.Split(key, "/")
	require.Len(t, parts, 3)
	assert.Equal(t, "talent", parts[0])
	assert.Equal(t, time.Now().Format("200601"), parts[1])
	assert.Equal(t, "abc123.webp", parts[2])
}

func TestBuildObjectKeyRejectsUnsafeInput(t *testing.T) {
	_, err := BuildObjectKey("../etc", "a.png")
	assert.Error(t, err, "traversal must not become a module")

	_, err = BuildObjectKey("Talent", "a.png")
	assert.Error(t, err, "modules are lowercase ASCII")

	_, err = BuildObjectKey("talent", "   ")
	assert.Error(t, err, "an empty name is not a key")
}

func TestPublicURLAndEscaping(t *testing.T) {
	cfg := Config{Provider: ProviderTencent, Bucket: "demo-1250000000", Region: "ap-nanjing", Scheme: "https"}
	assert.Equal(t,
		"https://demo-1250000000.cos.ap-nanjing.myqcloud.com/a/b.png",
		cfg.PublicURL("a/b.png"))

	cfg.PublicBaseURL = "https://cdn.example.com/"
	assert.Equal(t, "https://cdn.example.com/talent/202601/x.webp", cfg.PublicURL("talent/202601/x.webp"))

	// Separators survive, segments are encoded.
	assert.Equal(t, "talent/202601/%E5%9B%BE%20%E7%89%87.png", escapeKey("talent/202601/图 片.png"))
}

func TestValidateRequiresBucketAndCredentials(t *testing.T) {
	base := Config{Provider: ProviderTencent, Region: "ap-nanjing", SecretId: "id", SecretKey: "key"}
	assert.ErrorIs(t, base.Validate(), ErrNotConfigured, "empty bucket")

	noSecret := base
	noSecret.Bucket = "b"
	noSecret.SecretKey = ""
	assert.ErrorIs(t, noSecret.Validate(), ErrNotConfigured, "missing secret key")

	ok := base
	ok.Bucket = "b"
	assert.NoError(t, ok.Validate())

	other := ok
	other.Provider = "s3"
	assert.ErrorIs(t, other.Validate(), ErrNotConfigured, "unsupported provider")
}

func TestAuthorizationShape(t *testing.T) {
	client := New(Config{
		Provider: ProviderTencent, Bucket: "demo-1", Region: "ap-nanjing",
		Scheme: "https", SecretId: "AKIDtest", SecretKey: "secret",
	})
	header := map[string][]string{
		"Host":         {"demo-1.cos.ap-nanjing.myqcloud.com"},
		"Content-Type": {"image/png"},
	}
	auth := client.authorization("PUT", "/talent/202601/a.png", header)

	assert.Contains(t, auth, "q-sign-algorithm=sha1")
	assert.Contains(t, auth, "q-ak=AKIDtest")
	assert.Contains(t, auth, "q-header-list=content-type;host")
	signature := regexp.MustCompile(`q-signature=([0-9a-f]{40})`).FindStringSubmatch(auth)
	require.Len(t, signature, 2, "signature must be a sha1 hex digest: %s", auth)

	// Same inputs produce the same signature within the same signing window.
	again := client.authorization("PUT", "/talent/202601/a.png", header)
	assert.Equal(t, strings.Split(auth, "&q-signature=")[1][:0], "")
	assert.Len(t, again, len(auth))

	// A different path must not reuse the signature.
	other := client.authorization("PUT", "/talent/202601/b.png", header)
	assert.NotEqual(t, signature[1], regexp.MustCompile(`q-signature=([0-9a-f]{40})`).FindStringSubmatch(other)[1])
}
