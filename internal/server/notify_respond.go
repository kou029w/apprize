package server

import (
	"cmp"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"
	"time"

	"git.fogtype.com/nebel/apprize/internal/notify"
)

var logLevels = map[string]int{
	"trace":    0,
	"debug":    1,
	"info":     2,
	"warning":  3,
	"error":    4,
	"critical": 5,
}

func parseLogLevel(raw string) (int, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return logLevels["info"], nil
	}
	lv, ok := logLevels[raw]
	if !ok {
		return 0, fmt.Errorf("invalid X-Apprise-Log-Level: %s", raw)
	}
	return lv, nil
}

type logEntry struct {
	level   string // logLevels key
	asctime string
	message string
}

func (e logEntry) String() string {
	return fmt.Sprintf("%s - %s - %s", e.asctime, strings.ToUpper(e.level), e.message)
}

func (s *server) respondNotify(w http.ResponseWriter, r *http.Request, nr notifyRequest) {
	results, err := s.Notifier.Notify(r.Context(), nr.urls, nr.body, nr.opts)
	entries, failed := notifyLog(results, err)
	entries = filterLog(entries, nr.minLevel)

	status := http.StatusOK
	if failed {
		status = http.StatusFailedDependency
	}

	switch negotiate(r, mediaJSON, mediaText, mediaHTML) {
	case mediaText:
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = e.String()
		}
		writeText(w, status, strings.Join(lines, "\n"))
	case mediaHTML:
		var b strings.Builder
		b.WriteString("<html><body><ul>")
		for _, e := range entries {
			fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(e.String()))
		}
		b.WriteString("</ul></body></html>")
		writeHTML(w, status, b.String())
	default:
		details := make([][]string, len(entries))
		for i, e := range entries {
			details[i] = []string{e.level, e.asctime, e.message}
		}
		var errMsg any
		if failed {
			errMsg = "One or more notifications could not be sent"
		}
		writeJSON(w, status, map[string]any{"error": errMsg, "details": details})
	}
}

func notifyLog(results []notify.Result, err error) (entries []logEntry, failed bool) {
	now := time.Now().Format("2006-01-02 15:04:05,000")
	for _, res := range results {
		if res.OK {
			entries = append(entries, logEntry{"info", now, "Sent " + res.URL})
			continue
		}
		failed = true
		msg := cmp.Or(strings.TrimSpace(res.Message), "failed")
		if res.URL != "" {
			msg = "Failed " + res.URL + ": " + msg
		}
		entries = append(entries, logEntry{"error", now, msg})
	}
	if err != nil {
		failed = true
		entries = append(entries, logEntry{"error", now, err.Error()})
	}
	return entries, failed
}

func filterLog(entries []logEntry, minLevel int) []logEntry {
	return slices.DeleteFunc(entries, func(e logEntry) bool {
		return logLevels[e.level] < minLevel
	})
}
