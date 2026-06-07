package server

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeText writes a plain text response.
func writeText(w http.ResponseWriter, code int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(text))
}

// writeHTML writes an HTML response.
func writeHTML(w http.ResponseWriter, code int, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(html))
}
