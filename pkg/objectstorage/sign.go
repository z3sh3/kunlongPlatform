package objectstorage

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// sign implements COS's sha1 signing: the key time seeds a signing key, and the
// signature covers the canonical HTTP string as described in the COS docs.
func sign(secretKey, keyTime, httpString, _ string) string {
	signKey := hmacSHA1Hex(secretKey, keyTime)
	stringToSign := "sha1\n" + keyTime + "\n" + sha1Hex(httpString) + "\n"
	return hmacSHA1Hex(signKey, stringToSign)
}

func hmacSHA1Hex(key, data string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func sha1Hex(data string) string {
	sum := sha1.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}

// urlEncode percent-encodes a header value the way COS canonicalises it.
func urlEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func sortStrings(values []string) {
	sort.Strings(values)
}
