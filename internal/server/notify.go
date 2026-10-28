package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"git.fogtype.com/nebel/apprize/internal/appcfg"
	"git.fogtype.com/nebel/apprize/internal/notify"
)

type notifyPayload struct {
	Body  string
	Title string
	Type  string
	Tag   string
	// Format is nil when the request omits it. An empty string, or JSON
	// null, forces pass-through even when DefaultFormat is set.
	Format *string
	URLs   any
}

type jsonNotifyPayload struct {
	Body     string                     `json:"body"`
	Title    string                     `json:"title"`
	Type     string                     `json:"type"`
	Format   json.RawMessage            `json:"format"`
	Tag      string                     `json:"tag"`
	Tags     string                     `json:"tags"`
	Template map[string]json.RawMessage `json:"template"`
	URLs     any                        `json:"urls"`
}

const maxTemplateValueLen = 1024

var (
	validNotifyTypes   = map[string]bool{"info": true, "success": true, "warning": true, "failure": true}
	validNotifyFormats = map[string]bool{"": true, "text": true, "markdown": true, "html": true}
)

func (s *server) handleNotify(w http.ResponseWriter, r *http.Request) {
	nr, err := s.parseNotifyRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.respondNotify(w, r, nr)
}

type notifyRequest struct {
	body     string
	opts     notify.Options
	urls     []string
	minLevel int
}

// parseNotifyRequest validates r. The caller answers any error with 400.
func (s *server) parseNotifyRequest(r *http.Request) (notifyRequest, error) {
	payload, err := parseNotifyPayload(r)
	if err != nil {
		return notifyRequest{}, err
	}
	if err := applyHeaderAndQuery(r, &payload); err != nil {
		return notifyRequest{}, err
	}
	if strings.TrimSpace(payload.Body) == "" {
		return notifyRequest{}, errors.New("body is required")
	}
	opts, err := s.notifyOptions(payload)
	if err != nil {
		return notifyRequest{}, err
	}
	minLevel, err := parseLogLevel(r.Header.Get("X-Apprise-Log-Level"))
	if err != nil {
		return notifyRequest{}, err
	}

	urls := normalizeURLs(payload.URLs)
	if len(urls) == 0 {
		urls = s.StatelessURLs
	}
	urls = s.filterURLs(urls)
	if len(urls) == 0 {
		return notifyRequest{}, errors.New("no valid urls to notify")
	}
	return notifyRequest{body: payload.Body, opts: opts, urls: urls, minLevel: minLevel}, nil
}

func (s *server) notifyOptions(p notifyPayload) (notify.Options, error) {
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	if typ != "" && !validNotifyTypes[typ] {
		return notify.Options{}, fmt.Errorf("invalid type: %s", p.Type)
	}
	format := s.DefaultFormat
	if p.Format != nil {
		format = strings.ToLower(strings.TrimSpace(*p.Format))
	}
	if !validNotifyFormats[format] {
		return notify.Options{}, fmt.Errorf("invalid format: %s", format)
	}
	return notify.Options{
		Title:       strings.TrimSpace(p.Title),
		Type:        typ,
		InputFormat: format,
	}, nil
}

// applyHeaderAndQuery fills empty title, type and tag from the X-Apprise-*
// headers, then from the query string.
func applyHeaderAndQuery(r *http.Request, p *notifyPayload) error {
	q := r.URL.Query()
	fill := func(dst *string, header, query string) {
		if *dst == "" {
			*dst = cmp.Or(r.Header.Get(header), q.Get(query))
		}
	}
	fill(&p.Title, "X-Apprise-Title", "title")
	fill(&p.Type, "X-Apprise-Type", "type")
	fill(&p.Tag, "X-Apprise-Tag", "tag")

	// Explicit URLs ignore template values, but the spec still wants 400 for
	// a malformed header.
	h := r.Header.Get("X-Apprise-Template")
	if h == "" {
		return nil
	}
	vals, err := url.ParseQuery(h)
	if err != nil {
		return errors.New("invalid X-Apprise-Template header")
	}
	for name, vs := range vals {
		if name == "" {
			return errors.New("invalid X-Apprise-Template header")
		}
		if err := checkTemplateValues(name, vs...); err != nil {
			return err
		}
	}
	return nil
}

func parseNotifyPayload(r *http.Request) (notifyPayload, error) {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mt {
	case "application/json":
		return parseJSONPayload(r)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		return parseFormPayload(r, mt)
	case "text/plain", "text/html", "text/markdown":
		return parseTextPayload(r, mt)
	default:
		return notifyPayload{}, errors.New("unsupported content type")
	}
}

func parseJSONPayload(r *http.Request) (notifyPayload, error) {
	var jp jsonNotifyPayload
	if err := json.NewDecoder(r.Body).Decode(&jp); err != nil {
		return notifyPayload{}, errors.New("invalid json")
	}
	if err := validateJSONTemplate(jp.Template); err != nil {
		return notifyPayload{}, err
	}
	p := notifyPayload{
		Body:  jp.Body,
		Title: jp.Title,
		Type:  jp.Type,
		Tag:   cmp.Or(jp.Tag, jp.Tags),
		URLs:  jp.URLs,
	}
	if len(jp.Format) > 0 {
		var f *string
		if err := json.Unmarshal(jp.Format, &f); err != nil {
			return notifyPayload{}, errors.New("invalid format")
		}
		if f == nil {
			f = new(string)
		}
		p.Format = f
	}
	return p, nil
}

func parseFormPayload(r *http.Request, mt string) (notifyPayload, error) {
	var err error
	if mt == "multipart/form-data" {
		err = r.ParseMultipartForm(32 << 20)
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		return notifyPayload{}, errors.New("invalid form")
	}
	for k, vs := range r.PostForm {
		if strings.HasPrefix(k, "template[") {
			if err := checkTemplateValues(k, vs...); err != nil {
				return notifyPayload{}, err
			}
		}
	}
	p := notifyPayload{
		// body and urls fall back to the query string. title, type and tag
		// don't, so applyHeaderAndQuery can let headers win over the query.
		Body:  r.FormValue("body"),
		URLs:  r.FormValue("urls"),
		Title: r.PostFormValue("title"),
		Type:  r.PostFormValue("type"),
		Tag:   cmp.Or(r.PostFormValue("tag"), r.PostFormValue("tags")),
	}
	if r.PostForm.Has("format") {
		f := r.PostFormValue("format")
		p.Format = &f
	}
	return p, nil
}

// parseTextPayload treats the whole body as the message, in the format named
// by the content type unless ?format= names another.
func parseTextPayload(r *http.Request, mt string) (notifyPayload, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return notifyPayload{}, errors.New("unable to read body")
	}
	f := strings.TrimPrefix(mt, "text/")
	if f == "plain" {
		f = "text"
	}
	if q := r.URL.Query(); q.Has("format") {
		f = q.Get("format")
	}
	return notifyPayload{Body: string(b), Format: &f}, nil
}

func checkTemplateValues(name string, vals ...string) error {
	for _, v := range vals {
		if utf8.RuneCountInString(v) > maxTemplateValueLen {
			return fmt.Errorf("template value too long: %s", name)
		}
	}
	return nil
}

func validateJSONTemplate(tmpl map[string]json.RawMessage) error {
	for name, raw := range tmpl {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("invalid template value: %s", name)
		}
		switch tv := v.(type) {
		case string:
			if err := checkTemplateValues(name, tv); err != nil {
				return err
			}
		case float64, bool:
		default:
			return fmt.Errorf("invalid template value: %s", name)
		}
	}
	return nil
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
