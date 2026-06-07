package server

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

// GET /status
func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	mt := negotiate(r, mediaJSON, mediaText)

	if mt == mediaText {
		writeText(w, http.StatusOK, "OK")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"attach_lock": true,
		"config_lock": false,
		"status": map[string]any{
			"persistent_storage": false,
			"can_write_config":   false,
			"can_write_attach":   false,
			"details":            []string{fmt.Sprintf("apprize %s", s.Version), fmt.Sprintf("schemas: %d", len(s.schemas))},
		},
	})
}

// GET /details
func (s *server) handleDetails(w http.ResponseWriter, r *http.Request) {
	mt := negotiate(r, mediaJSON, mediaHTML)

	if mt == mediaHTML {
		var b strings.Builder
		b.WriteString("<html><head><title>Apprise Details</title></head><body>")
		b.WriteString("<h1>Supported Notification Services</h1><ul>")
		for _, schema := range s.schemas {
			fmt.Fprintf(&b, "<li>%s</li>\n", html.EscapeString(schema))
		}
		b.WriteString("</ul></body></html>")
		writeHTML(w, http.StatusOK, b.String())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.Version,
		"schemas": s.schemas,
	})
}
