package appcfg

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ServiceName maps URL schemes to display names.
func ServiceName(rawURL string) string {
	scheme := rawURL
	if i := strings.Index(rawURL, "://"); i > 0 {
		scheme = rawURL[:i]
	}
	switch strings.ToLower(scheme) {
	case "tgram":
		return "Telegram"
	case "discord":
		return "Discord"
	case "gotify":
		return "Gotify"
	default:
		return scheme
	}
}

// URLID returns a stable short id from the URL.
func URLID(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:4])
}

// Scheme extracts lower-cased scheme from a URL-like string.
func Scheme(rawURL string) string {
	if i := strings.Index(rawURL, "://"); i > 0 {
		return strings.ToLower(rawURL[:i])
	}
	return strings.ToLower(rawURL)
}
