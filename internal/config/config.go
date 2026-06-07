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
	Bind          string
	APIKey        string
	StatelessURLs []string
	RecursionMax  int
	DenyServices  []string
	AllowServices []string
}

// FromEnv builds a Config from environment variables, applying defaults.
func FromEnv() Config {
	return Config{
		Bind:          firstNonEmpty(os.Getenv("APPRIZE_BIND"), portToBind(os.Getenv("HTTP_PORT")), ":8000"),
		APIKey:        os.Getenv("APPRIZE_API_KEY"),
		StatelessURLs: splitList(os.Getenv("APPRIZE_STATELESS_URLS")),
		RecursionMax:  parseInt(os.Getenv("APPRIZE_RECURSION_MAX"), 1),
		DenyServices:  splitList(os.Getenv("APPRIZE_DENY_SERVICES")),
		AllowServices: splitList(os.Getenv("APPRIZE_ALLOW_SERVICES")),
	}
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

func parseInt(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}
