package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"git.fogtype.com/nebel/apprize/internal/appcfg"
	"git.fogtype.com/nebel/apprize/internal/notify"
	"git.fogtype.com/nebel/apprize/internal/store"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type configPayload struct {
	Config string `json:"config"`
	Format string `json:"format"`
	URLs   any    `json:"urls"`
}

type notifyPayload struct {
	Body   string `json:"body"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Format string `json:"format"`
	Tag    string `json:"tag"`
	URLs   any    `json:"urls"`
}

type parsedConfigEntry struct {
	URL  string
	Tags []string
}

func (s *server) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	if !s.Admin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	keys, err := s.Store.List(r.Context())
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if negotiate(r, mediaJSON, mediaHTML) == mediaHTML {
		var b strings.Builder
		b.WriteString("<html><body><ul>")
		for _, k := range keys {
			fmt.Fprintf(&b, "<li>%s</li>", html.EscapeString(k))
		}
		b.WriteString("</ul></body></html>")
		writeHTML(w, http.StatusOK, b.String())
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *server) handleAddConfig(w http.ResponseWriter, r *http.Request) {
	if s.ConfigLock {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	key := s.configKey(r)
	if !keyPattern.MatchString(key) {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}

	cfg, format, err := parseAddPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateConfig(format, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Store.Put(r.Context(), store.Entry{
		Key:       key,
		Format:    format,
		Config:    cfg,
		UpdatedAt: time.Now().Unix(),
	}); err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}

	if negotiate(r, mediaJSON, mediaText) == mediaJSON {
		writeJSON(w, http.StatusOK, map[string]any{"error": nil})
		return
	}
	writeText(w, http.StatusOK, "Successfully saved configuration")
}

func (s *server) handleDeleteConfig(w http.ResponseWriter, r *http.Request) {
	if s.ConfigLock {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ok, err := s.Store.Delete(r.Context(), s.configKey(r))
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if s.ConfigLock {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	entry, ok, err := s.Store.Get(r.Context(), s.configKey(r))
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if !ok || strings.TrimSpace(entry.Config) == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	mt := negotiate(r, mediaJSON, mediaText)
	if mt == mediaText {
		if strings.EqualFold(entry.Format, "yaml") {
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(entry.Config))
			return
		}
		writeText(w, http.StatusOK, entry.Config)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"format": entry.Format,
		"config": entry.Config,
	})
}

func (s *server) handleNotifyByKey(w http.ResponseWriter, r *http.Request) {
	payload, err := parseNotifyPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Body) == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	entry, ok, err := s.Store.Get(r.Context(), s.configKey(r))
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	entries, err := parseStoredConfig(entry.Format, entry.Config)
	if err != nil {
		http.Error(w, "invalid stored config", http.StatusInternalServerError)
		return
	}
	tagExpr := payload.Tag
	if strings.TrimSpace(tagExpr) == "" {
		tagExpr = "all"
	}
	urls := make([]string, 0, len(entries))
	for _, e := range entries {
		if matchTagExpr(tagExpr, e.Tags) {
			urls = append(urls, e.URL)
		}
	}
	urls = s.filterURLs(urls)
	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.respondNotify(w, r, payload, urls)
}

func (s *server) handleJSONURLs(w http.ResponseWriter, r *http.Request) {
	entry, ok, err := s.Store.Get(r.Context(), s.configKey(r))
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	entries, err := parseStoredConfig(entry.Format, entry.Config)
	if err != nil || len(entries) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tagExpr := r.URL.Query().Get("tag")
	if strings.TrimSpace(tagExpr) == "" {
		tagExpr = "all"
	}
	privacy := r.URL.Query().Get("privacy") == "1"

	filtered := make([]parsedConfigEntry, 0, len(entries))
	tagSet := map[string]struct{}{}
	for _, e := range entries {
		if !matchTagExpr(tagExpr, e.Tags) {
			continue
		}
		filtered = append(filtered, e)
		for _, t := range e.Tags {
			tagSet[t] = struct{}{}
		}
	}
	if len(filtered) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	urlRows := make([]map[string]any, 0, len(filtered))
	for _, e := range filtered {
		u := e.URL
		if privacy {
			u = maskURL(u)
		}
		tagDetails := make([]map[string]string, 0, len(e.Tags))
		for _, t := range e.Tags {
			tagDetails = append(tagDetails, map[string]string{"name": t})
		}
		urlRows = append(urlRows, map[string]any{
			"id":           appcfg.URLID(e.URL),
			"service_name": appcfg.ServiceName(e.URL),
			"enabled":      true,
			"url":          u,
			"tags":         e.Tags,
			"tag_details":  tagDetails,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tags": tags,
		"urls": urlRows,
	})
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

func parseAddPayload(r *http.Request) (cfg string, format string, err error) {
	format = "text"
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(ct, "application/json"):
		var p configPayload
		if err = json.NewDecoder(r.Body).Decode(&p); err != nil {
			return "", "", fmt.Errorf("invalid json")
		}
		cfg = strings.TrimSpace(p.Config)
		format = normalizeFormat(p.Format)
		if cfg == "" {
			cfg = urlsToConfig(normalizeURLs(p.URLs))
		}
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"), strings.HasPrefix(ct, "multipart/form-data"):
		if strings.HasPrefix(ct, "multipart/form-data") {
			err = r.ParseMultipartForm(32 << 20)
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			return "", "", fmt.Errorf("invalid form")
		}
		cfg = strings.TrimSpace(r.FormValue("config"))
		format = normalizeFormat(r.FormValue("format"))
		if cfg == "" {
			cfg = urlsToConfig(normalizeURLs(r.FormValue("urls")))
		}
	default:
		return "", "", fmt.Errorf("unsupported content type")
	}
	if cfg == "" {
		return "", "", fmt.Errorf("config is required")
	}
	if format == "auto" {
		format = detectFormat(cfg)
	}
	if format == "" {
		format = "text"
	}
	return cfg, format, nil
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
		return splitList(strings.Join(v, ","))
	case []any:
		items := make([]string, 0, len(v))
		for _, it := range v {
			if s, ok := it.(string); ok {
				items = append(items, s)
			}
		}
		return splitList(strings.Join(items, ","))
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

func urlsToConfig(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	return strings.Join(urls, "\n")
}

func normalizeFormat(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yaml", "text", "auto":
		return strings.ToLower(strings.TrimSpace(s))
	case "":
		return ""
	default:
		return ""
	}
}

func detectFormat(cfg string) string {
	for _, ln := range strings.Split(cfg, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.Contains(ln, ":") && !strings.Contains(ln, "://") {
			return "yaml"
		}
		break
	}
	return "text"
}

func validateConfig(format, cfg string) error {
	if format != "text" && format != "yaml" {
		return fmt.Errorf("invalid format")
	}
	entries, err := appcfg.Parse(format, cfg)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("invalid %s", format)
	}
	return nil
}

func parseStoredConfig(format, cfg string) ([]parsedConfigEntry, error) {
	entries, err := appcfg.Parse(format, cfg)
	if err != nil {
		return nil, err
	}
	out := make([]parsedConfigEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, parsedConfigEntry{URL: e.URL, Tags: e.Tags})
	}
	return out, nil
}

func matchTagExpr(expr string, tags []string) bool {
	return appcfg.Match(appcfg.ParseTagExpr(expr), tags)
}

func maskURL(raw string) string {
	return appcfg.MaskURL(raw)
}

func (s *server) filterURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	allow := make(map[string]struct{}, len(s.AllowServices))
	for _, sc := range s.AllowServices {
		allow[strings.ToLower(strings.TrimSpace(sc))] = struct{}{}
	}
	deny := make(map[string]struct{}, len(s.DenyServices))
	for _, sc := range s.DenyServices {
		deny[strings.ToLower(strings.TrimSpace(sc))] = struct{}{}
	}

	filtered := make([]string, 0, len(urls))
	for _, raw := range urls {
		sc := appcfg.Scheme(raw)
		if len(allow) > 0 {
			if _, ok := allow[sc]; !ok {
				continue
			}
		}
		if _, blocked := deny[sc]; blocked {
			continue
		}
		filtered = append(filtered, raw)
	}
	return filtered
}

func (s *server) configKey(r *http.Request) string {
	key := strings.TrimSpace(r.PathValue("key"))
	if key != "" {
		return key
	}
	key = strings.TrimSpace(s.DefaultConfigID)
	if key == "" {
		return "apprise"
	}
	return key
}
