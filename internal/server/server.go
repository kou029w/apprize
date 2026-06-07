// Package server wires the Apprise-compatible HTTP API.
package server

import (
	"net/http"
	"strings"

	"git.fogtype.com/nebel/apprize/internal/notify"
)

// Deps holds everything a server instance needs. Tests construct it directly
// with a fake Notifier.
type Deps struct {
	Notifier notify.Notifier

	// Behaviour toggles (mirrors the relevant apprise-api env vars).
	StatelessURLs []string // APPRIZE_STATELESS_URLS
	RecursionMax  int      // APPRIZE_RECURSION_MAX
	DenyServices  []string // APPRIZE_DENY_SERVICES
	AllowServices []string // APPRIZE_ALLOW_SERVICES
	APIKey        string   // optional simple API secret (empty = no auth)
	MaxBodyBytes  int64    // request body limit; 0 = sensible default; test-only

	Version string
}

// server is the concrete handler holding dependencies.
type server struct {
	Deps
	allowSet map[string]struct{}
	denySet  map[string]struct{}
	schemas  []string
}

// New returns the HTTP handler implementing the Apprise API.
func New(d Deps) http.Handler {
	s := &server{
		Deps:     d,
		allowSet: buildServiceSet(d.AllowServices),
		denySet:  buildServiceSet(d.DenyServices),
		schemas:  notify.SupportedSchemas(),
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /details", s.handleDetails)
	mux.HandleFunc("POST /notify", s.handleNotify)

	return withMiddlewareChain(s, mux)
}

func buildServiceSet(services []string) map[string]struct{} {
	if len(services) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(services))
	for _, sc := range services {
		m[strings.ToLower(strings.TrimSpace(sc))] = struct{}{}
	}
	return m
}
