package shared

import "strings"

// Card presentation helpers.
//
// Course and program cards show a cover image and a short introduction. Both
// modules persist the same two presentation fields, so the sanitising rules live
// here instead of being duplicated (console modules must not import each other).

const (
	// CardSummaryLimit is the longest card summary accepted for storage.
	CardSummaryLimit = 500
	// CardCoverUrlLimit is the longest cover URL accepted for storage.
	CardCoverUrlLimit = 512
)

// NormalizeCoverUrl trims a cover image address and keeps only usable values:
// an absolute http(s) URL or a site-relative path. Anything else is dropped
// rather than rejected, because a bad image must not block saving content.
func NormalizeCoverUrl(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if len(value) > CardCoverUrlLimit {
		return ""
	}
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return value
	}
	// Site-relative asset, e.g. /static/covers/course.svg
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value
	}
	return ""
}

// NormalizeSummary trims the card introduction and caps its length so a card
// layout cannot be broken by an oversized value.
func NormalizeSummary(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > CardSummaryLimit {
		return string(runes[:CardSummaryLimit])
	}
	return value
}
