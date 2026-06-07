// Package server wires the Apprise-compatible HTTP API.
package server

import (
	"net/http"

	"git.fogtype.com/nebel/apprize/internal/notify"
	"git.fogtype.com/nebel/apprize/internal/store"
)

// Deps holds everything a server instance needs. Tests construct it directly
// with a fake Notifier and an in-memory Store.
type Deps struct {
	Notifier notify.Notifier
	Store    store.Store

	// Behaviour toggles (mirrors the relevant apprise-api env vars).
	StatelessURLs   []string // APPRIZE_STATELESS_URLS
	ConfigLock      bool     // APPRIZE_CONFIG_LOCK
	Admin           bool     // APPRIZE_ADMIN
	RecursionMax    int      // APPRIZE_RECURSION_MAX
	DenyServices    []string // APPRIZE_DENY_SERVICES
	AllowServices   []string // APPRIZE_ALLOW_SERVICES
	APIKey          string   // optional simple API secret (empty = no auth)
	MaxBodyBytes    int64    // request body limit; 0 = sensible default
	DefaultConfigID string   // APPRIZE_DEFAULT_CONFIG_ID

	Version string
}

// server is the concrete handler holding dependencies.
type server struct {
	Deps
}

// New returns the HTTP handler implementing the Apprise API.
func New(d Deps) http.Handler {
	s := &server{Deps: d}
	mux := http.NewServeMux()

	// Meta — P1
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /details", s.handleDetails)

	// Stateless — P2+
	mux.HandleFunc("POST /notify", s.handleNotify)

	// Persistent — P4-P5
	mux.HandleFunc("GET /cfg", s.handleListConfigs)
	mux.HandleFunc("POST /cfg", s.handleGetConfig)
	mux.HandleFunc("POST /add", s.handleAddConfig)
	mux.HandleFunc("POST /del", s.handleDeleteConfig)
	mux.HandleFunc("POST /get", s.handleGetConfig)
	mux.HandleFunc("POST /add/{key}", s.handleAddConfig)
	mux.HandleFunc("POST /del/{key}", s.handleDeleteConfig)
	mux.HandleFunc("POST /get/{key}", s.handleGetConfig)
	mux.HandleFunc("POST /cfg/{key}", s.handleGetConfig)
	mux.HandleFunc("POST /notify/{key}", s.handleNotifyByKey)
	mux.HandleFunc("GET /json/urls/{key}", s.handleJSONURLs)

	return s.withMiddleware(mux)
}

// withMiddleware composes the cross-cutting middleware chain.
// P1 wires the full stack: requestLog → recover → apiKey → recursion → bodyLimit → handler.
func (s *server) withMiddleware(h http.Handler) http.Handler {
	return withMiddlewareChain(s, h)
}
