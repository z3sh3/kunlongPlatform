package shared

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeCoverUrl(t *testing.T) {
	cases := map[string]string{
		"":                               "",
		"   ":                            "",
		"https://cdn.example.com/a.webp": "https://cdn.example.com/a.webp",
		" http://cdn.example.com/b.png ": "http://cdn.example.com/b.png",
		"/static/covers/course.svg":      "/static/covers/course.svg",
		// Rejected: not an image address a browser can load safely.
		"ftp://cdn.example.com/a.png": "",
		"//cdn.example.com/a.png":     "",
		"javascript:alert(1)":         "",
		"data:image/png;base64,AAAA":  "",
	}
	for input, expected := range cases {
		assert.Equal(t, expected, NormalizeCoverUrl(input), "input %q", input)
	}
	assert.Equal(t, "", NormalizeCoverUrl("https://x.test/"+strings.Repeat("a", CardCoverUrlLimit)))
}

func TestNormalizeSummary(t *testing.T) {
	assert.Equal(t, "", NormalizeSummary("   "))
	assert.Equal(t, "面向企业的认证培训", NormalizeSummary("  面向企业的认证培训  "))

	long := strings.Repeat("简", CardSummaryLimit+50)
	capped := NormalizeSummary(long)
	assert.Equal(t, CardSummaryLimit, len([]rune(capped)))
	assert.True(t, strings.HasPrefix(capped, "简简"))
}
