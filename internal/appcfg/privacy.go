package appcfg

import (
	"net/url"
	"strings"
)

// MaskURL redacts credentials and common secret query/path segments.
func MaskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return maskPathToken(raw)
	}
	if u.User != nil {
		username := u.User.Username()
		if username == "" {
			username = "****"
		}
		u.User = url.UserPassword(username, "****")
	}
	q := u.Query()
	for key := range q {
		lk := strings.ToLower(key)
		if strings.Contains(lk, "token") || strings.Contains(lk, "key") || strings.Contains(lk, "secret") || strings.Contains(lk, "pass") {
			q.Set(key, "****")
		}
	}
	u.RawQuery = q.Encode()
	return maskPathToken(u.String())
}

func maskPathToken(raw string) string {
	i := strings.Index(raw, "://")
	if i < 0 {
		return raw
	}
	head := raw[:i+3]
	rest := raw[i+3:]
	j := strings.Index(rest, "/")
	if j < 0 {
		return raw
	}
	host := rest[:j+1]
	path := rest[j+1:]
	if path == "" {
		return raw
	}
	return head + host + "****"
}
