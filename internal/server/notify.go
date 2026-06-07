package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"git.fogtype.com/nebel/apprize/internal/appcfg"
	"git.fogtype.com/nebel/apprize/internal/notify"
)

type notifyPayload struct {
	Body   string `json:"body"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Format string `json:"format"`
	Tag    string `json:"tag"`
	URLs   any    `json:"urls"`
}

func (s *server) handleNotify(w http.ResponseWriter, r *http.Request) {
	payload, err := parseNotifyPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Body) == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	urls := normalizeURLs(payload.URLs)
	if len(urls) == 0 {
		urls = append(urls, s.StatelessURLs...)
	}
	urls = s.filterURLs(urls)
	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.respondNotify(w, r, payload, urls)
}

func (s *server) respondNotify(w http.ResponseWriter, r *http.Request, payload notifyPayload, urls []string) {
	opts := notify.Options{
		Title:       strings.TrimSpace(payload.Title),
		Type:        strings.TrimSpace(payload.Type),
		InputFormat: strings.TrimSpace(payload.Format),
	}
	results, err := s.Notifier.Notify(r.Context(), urls, payload.Body, opts)
	if err != nil {
		results = append(results, notify.Result{
			URL:     "",
			Service: "",
			OK:      false,
			Message: err.Error(),
		})
	}

	now := time.Now().Format("2006-01-02 15:04:05,000")

	details := make([][]string, 0, len(results))
	failed := false
	for _, res := range results {
		if res.OK {
			details = append(details, []string{"info", now, "Sent " + res.URL})
			continue
		}
		failed = true
		msg := strings.TrimSpace(res.Message)
		if msg == "" {
			msg = "failed"
		}
		if res.URL != "" {
			msg = "Failed " + res.URL + ": " + msg
		}
		details = append(details, []string{"error", now, msg})
	}

	resp := map[string]any{
		"error":   nil,
		"details": details,
	}
	status := http.StatusOK
	if failed {
		status = http.StatusFailedDependency
		resp["error"] = "One or more notifications could not be sent"
	}

	mt := negotiate(r, mediaJSON, mediaText, mediaHTML)
	switch mt {
	case mediaText:
		lines := make([]string, 0, len(details))
		for _, d := range details {
			if len(d) >= 3 {
				lines = append(lines, fmt.Sprintf("%s - %s - %s", d[1], strings.ToUpper(d[0]), d[2]))
			} else {
				lines = append(lines, strings.Join(d, ": "))
			}
		}
		writeText(w, status, strings.Join(lines, "\n"))
	case mediaHTML:
		var b strings.Builder
		b.WriteString("<html><body><ul>")
		for _, d := range details {
			if len(d) >= 3 {
				fmt.Fprintf(&b, "<li>%s - %s - %s</li>", html.EscapeString(d[1]), html.EscapeString(strings.ToUpper(d[0])), html.EscapeString(d[2]))
			} else {
				fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(strings.Join(d, ": ")))
			}
		}
		b.WriteString("</ul></body></html>")
		writeHTML(w, status, b.String())
	default:
		writeJSON(w, status, resp)
	}
}

func parseNotifyPayload(r *http.Request) (notifyPayload, error) {
	var p notifyPayload
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(ct, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			return p, fmt.Errorf("invalid json")
		}
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"), strings.HasPrefix(ct, "multipart/form-data"):
		var err error
		if strings.HasPrefix(ct, "multipart/form-data") {
			err = r.ParseMultipartForm(32 << 20)
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			return p, fmt.Errorf("invalid form")
		}
		p.Body = r.FormValue("body")
		p.Title = r.FormValue("title")
		p.Type = r.FormValue("type")
		p.Format = r.FormValue("format")
		p.Tag = r.FormValue("tag")
		p.URLs = r.FormValue("urls")
	default:
		return p, fmt.Errorf("unsupported content type")
	}
	return p, nil
}

func normalizeURLs(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			out = append(out, splitList(s)...)
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			if s, ok := it.(string); ok {
				out = append(out, splitList(s)...)
			}
		}
		return out
	case string:
		return splitList(v)
	default:
		return nil
	}
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *server) filterURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	if len(s.allowSet) == 0 && len(s.denySet) == 0 {
		return urls
	}
	filtered := make([]string, 0, len(urls))
	for _, raw := range urls {
		sc := appcfg.Scheme(raw)
		if len(s.allowSet) > 0 {
			if _, ok := s.allowSet[sc]; !ok {
				continue
			}
		}
		if _, blocked := s.denySet[sc]; blocked {
			continue
		}
		filtered = append(filtered, raw)
	}
	return filtered
}
