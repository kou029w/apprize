// Package config loads server configuration from environment variables and CLI
// flags. Environment variable names mirror caronc/apprise-api where sensible.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config is the resolved server configuration.
type Config struct {
	Bind            string
	DBPath          string
	APIKey          string
	ConfigMaxKB     int64
	DefaultConfigID string
	StatelessURLs   []string
	ConfigLock      bool
	Admin           bool
	RecursionMax    int
	DenyServices    []string
	AllowServices   []string
}

// FromEnv builds a Config from environment variables, applying defaults.
func FromEnv() Config {
	c := Config{
		Bind:            firstNonEmpty(os.Getenv("APPRIZE_BIND"), portToBind(os.Getenv("HTTP_PORT")), ":8000"),
		DBPath:          firstNonEmpty(os.Getenv("APPRIZE_DB_PATH"), "./apprize.db"),
		APIKey:          os.Getenv("APPRIZE_API_KEY"),
		ConfigMaxKB:     parsePositiveInt64(os.Getenv("APPRIZE_CONFIG_MAX_LENGTH"), 512),
		DefaultConfigID: firstNonEmpty(os.Getenv("APPRIZE_DEFAULT_CONFIG_ID"), "apprise"),
		StatelessURLs:   splitList(os.Getenv("APPRIZE_STATELESS_URLS")),
		ConfigLock:      parseBool(os.Getenv("APPRIZE_CONFIG_LOCK"), false),
		Admin:           parseBool(os.Getenv("APPRIZE_ADMIN"), false),
		RecursionMax:    parseInt(os.Getenv("APPRIZE_RECURSION_MAX"), 1),
		DenyServices:    splitList(os.Getenv("APPRIZE_DENY_SERVICES")),
		AllowServices:   splitList(os.Getenv("APPRIZE_ALLOW_SERVICES")),
	}
	return c
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func portToBind(port string) string {
	if port == "" {
		return ""
	}
	return ":" + port
}

// splitList splits a comma/space separated list, dropping empties.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func parseBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true", "1", "on":
		return true
	case "no", "n", "false", "0", "off":
		return false
	default:
		return def
	}
}

func parseInt(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func parsePositiveInt64(s string, def int64) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
