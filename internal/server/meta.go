package server

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"git.fogtype.com/nebel/apprize/internal/notify"
)

// GET /status
func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	mt := negotiate(r, mediaJSON, mediaText)

	// Build status response. Attachments are always unsupported in this build,
	// so attach_lock is effectively always true regardless of external config.
	attachLock := true
	persistentStorage := true
	canWriteConfig := !s.ConfigLock
	canWriteAttach := false

	// Check if store is accessible (simple check - try listing)
	var storeErr error
	if s.Store == nil {
		storeErr = fmt.Errorf("no store configured")
	} else {
		_, storeErr = s.Store.List(r.Context())
	}
	if storeErr != nil {
		err := storeErr
		if mt == mediaText {
			writeText(w, http.StatusExpectationFailed, "CONFIG_PERMISSION_ISSUE")
		} else {
			writeJSON(w, http.StatusExpectationFailed, map[string]any{
				"attach_lock": attachLock,
				"config_lock": s.ConfigLock,
				"status": map[string]any{
					"persistent_storage": false,
					"can_write_config":   false,
					"can_write_attach":   false,
					"details":            []string{"store error: " + err.Error()},
				},
			})
		}
		return
	}

	if mt == mediaText {
		writeText(w, http.StatusOK, "OK")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"attach_lock": attachLock,
		"config_lock": s.ConfigLock,
		"status": map[string]any{
			"persistent_storage": persistentStorage,
			"can_write_config":   canWriteConfig,
			"can_write_attach":   canWriteAttach,
			"details":            []string{fmt.Sprintf("apprize %s", s.Version), fmt.Sprintf("schemas: %d", len(notify.SupportedSchemas()))},
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
		for _, schema := range notify.SupportedSchemas() {
			fmt.Fprintf(&b, "<li>%s</li>\n", html.EscapeString(schema))
		}
		b.WriteString("</ul></body></html>")
		writeHTML(w, http.StatusOK, b.String())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.Version,
		"schemas": notify.SupportedSchemas(),
	})
}
