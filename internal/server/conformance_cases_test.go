package server_test

// conformanceCases enumerates one or more cases per swagger operation. Each case
// targets a (method, specPath) so TestSpecCoverage can prove the suite covers
// the whole contract. Cases assert documented status codes, content negotiation
// and (for JSON bodies) schema conformance.

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"git.fogtype.com/nebel/apprize/internal/server"
)

const (
	ctJSON = "application/json"
	ctForm = "application/x-www-form-urlencoded"
	ctText = "text/plain"
	ctHTML = "text/html"
)

type opCase struct {
	name     string
	specPath string
	method   string
	run      func(t *testing.T)
}

// logResponse mirrors the swagger LogResponse for decoding.
type logResponse struct {
	Error   *string    `json:"error"`
	Details [][]string `json:"details"`
}

func formBody(vals map[string]string) []byte {
	v := url.Values{}
	for k, val := range vals {
		v.Set(k, val)
	}
	return []byte(v.Encode())
}

func decodeLog(t *testing.T, body []byte) logResponse {
	t.Helper()
	var lr logResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		t.Fatalf("decode LogResponse: %v (body %q)", err, truncate(string(body), 200))
	}
	return lr
}

var conformanceCases = []opCase{
	// -------------------------------------------------------------------
	// GET /status
	// -------------------------------------------------------------------
	{"status/200-json", "/status", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/status", accept: ctJSON}.do(newServer())
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctJSON)
		validateSchema(t, "/status", "GET", "200", ctJSON, rec.Body.Bytes())
	}},
	{"status/200-json-attach-locked", "/status", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/status", accept: ctJSON}.do(newServer())
		wantStatus(t, rec, 200)
		var obj map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
			t.Fatalf("status body is not JSON: %v", err)
		}
		if lock, ok := obj["attach_lock"].(bool); !ok || !lock {
			t.Fatalf("attach_lock = %#v, want true", obj["attach_lock"])
		}
		status, _ := obj["status"].(map[string]any)
		if can, ok := status["can_write_attach"].(bool); !ok || can {
			t.Fatalf("status.can_write_attach = %#v, want false", status["can_write_attach"])
		}
		if obj["stateless_enabled"] != true || obj["stateful_enabled"] != false || obj["privilege"] != "admin" {
			t.Fatalf("modes = %v/%v/%v, want stateless only, admin", obj["stateless_enabled"], obj["stateful_enabled"], obj["privilege"])
		}
	}},
	{"status/200-text", "/status", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/status", accept: ctText}.do(newServer())
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctText)
		if !strings.Contains(rec.Body.String(), "OK") {
			t.Errorf("text status body = %q, want to contain OK", rec.Body.String())
		}
	}},

	// -------------------------------------------------------------------
	// GET /details
	// -------------------------------------------------------------------
	{"details/200-json", "/details", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/details?all=yes", accept: ctJSON}.do(newServer())
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctJSON)
		var obj map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
			t.Errorf("details body is not a JSON object: %v", err)
		}
	}},
	{"details/200-html", "/details", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/details", accept: ctHTML}.do(newServer())
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctHTML)
	}},

	// -------------------------------------------------------------------
	// POST /notify
	// -------------------------------------------------------------------
	{"notify/200-json", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, accept: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 200)
		validateSchema(t, "/notify", "POST", "200", ctJSON, rec.Body.Bytes())
		if lr := decodeLog(t, rec.Body.Bytes()); lr.Error != nil {
			t.Errorf("expected error null on success, got %q", *lr.Error)
		}
	}},
	{"notify/200-form", "/notify", "POST", func(t *testing.T) {
		body := formBody(map[string]string{"urls": "gotify://host/token", "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctForm, accept: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 200)
		validateSchema(t, "/notify", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"notify/200-multipart", "/notify", "POST", func(t *testing.T) {
		ct, body := multipartBody(t, map[string]string{"urls": "gotify://host/token", "body": "hi"}, nil)
		rec := req{method: "POST", path: "/notify", contentType: ct, accept: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 200)
		validateSchema(t, "/notify", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"notify/200-format-omitted-passes-through", "/notify", "POST", func(t *testing.T) {
		var c captured
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Notifier = captureNotifier(&c) }))
		wantStatus(t, rec, 200)
		if c.opts.InputFormat != "" {
			t.Errorf("format = %q, want pass-through", c.opts.InputFormat)
		}
	}},
	{"notify/200-format-omitted-uses-default", "/notify", "POST", func(t *testing.T) {
		var c captured
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Notifier = captureNotifier(&c); d.DefaultFormat = "markdown" }))
		wantStatus(t, rec, 200)
		if c.opts.InputFormat != "markdown" {
			t.Errorf("format = %q, want markdown", c.opts.InputFormat)
		}
	}},
	{"notify/200-format-null-overrides-default", "/notify", "POST", func(t *testing.T) {
		var c captured
		body := []byte(`{"urls":["gotify://host/token"],"body":"hi","format":null}`)
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Notifier = captureNotifier(&c); d.DefaultFormat = "markdown" }))
		wantStatus(t, rec, 200)
		if c.opts.InputFormat != "" {
			t.Errorf("format = %q, want pass-through", c.opts.InputFormat)
		}
	}},
	{"notify/200-text-plain-body", "/notify", "POST", func(t *testing.T) {
		var c captured
		rec := req{method: "POST", path: "/notify", contentType: "text/plain; charset=utf-8", body: []byte("hello\nworld")}.
			do(newServer(func(d *server.Deps) {
				d.Notifier = captureNotifier(&c)
				d.StatelessURLs = []string{"gotify://host/token"}
			}))
		wantStatus(t, rec, 200)
		if c.body != "hello\nworld" || c.opts.InputFormat != "text" {
			t.Errorf("body/format = %q/%q, want whole body as text", c.body, c.opts.InputFormat)
		}
	}},
	{"notify/200-text-markdown-format-query", "/notify", "POST", func(t *testing.T) {
		var c captured
		rec := req{method: "POST", path: "/notify?format=html", contentType: "text/markdown", body: []byte("<b>hi</b>")}.
			do(newServer(func(d *server.Deps) {
				d.Notifier = captureNotifier(&c)
				d.StatelessURLs = []string{"gotify://host/token"}
			}))
		wantStatus(t, rec, 200)
		if c.opts.InputFormat != "html" {
			t.Errorf("format = %q, want html from ?format=", c.opts.InputFormat)
		}
	}},
	{"notify/200-header-and-query-precedence", "/notify", "POST", func(t *testing.T) {
		var c captured
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi", "title": "payload"})
		rec := req{method: "POST", path: "/notify?title=query&type=failure", contentType: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Title": "header", "X-Apprise-Type": "warning"}}.
			do(newServer(func(d *server.Deps) { d.Notifier = captureNotifier(&c) }))
		wantStatus(t, rec, 200)
		if c.opts.Title != "payload" || c.opts.Type != "warning" {
			t.Errorf("title/type = %q/%q, want payload/warning", c.opts.Title, c.opts.Type)
		}
	}},
	{"notify/200-query-fallback", "/notify", "POST", func(t *testing.T) {
		var c captured
		rec := req{method: "POST", path: "/notify?title=query&type=success", contentType: ctText, body: []byte("hi")}.
			do(newServer(func(d *server.Deps) {
				d.Notifier = captureNotifier(&c)
				d.StatelessURLs = []string{"gotify://host/token"}
			}))
		wantStatus(t, rec, 200)
		if c.opts.Title != "query" || c.opts.Type != "success" {
			t.Errorf("title/type = %q/%q, want query/success", c.opts.Title, c.opts.Type)
		}
	}},
	{"notify/200-log-level-filters", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, accept: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Log-Level": "error"}}.do(newServer())
		wantStatus(t, rec, 200)
		if lr := decodeLog(t, rec.Body.Bytes()); len(lr.Details) != 0 {
			t.Errorf("details = %v, want info entries filtered out", lr.Details)
		}
	}},
	{"notify/400-invalid-type", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi", "type": "bogus"})
		wantStatus(t, req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer()), 400)
	}},
	{"notify/400-invalid-format", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi", "format": "rtf"})
		wantStatus(t, req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer()), 400)
	}},
	{"notify/400-invalid-template-header", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		wantStatus(t, req{method: "POST", path: "/notify", contentType: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Template": "room=%zz"}}.do(newServer()), 400)
	}},
	{"notify/400-invalid-template-value", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi", "template": map[string]any{"room": []string{"x"}}})
		wantStatus(t, req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer()), 400)
	}},
	{"notify/400-invalid-log-level", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		wantStatus(t, req{method: "POST", path: "/notify", contentType: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Log-Level": "loud"}}.do(newServer()), 400)
	}},
	{"notify/400-no-urls", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 400)
	}},
	{"notify/400-missing-body", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 400)
	}},
	{"notify/406-recursion", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Recursion-Count": "5"}}.do(newServer())
		wantStatus(t, rec, 406)
	}},
	{"notify/424-failure", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, accept: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Notifier = failNotifier() }))
		wantStatus(t, rec, 424)
		if lr := decodeLog(t, rec.Body.Bytes()); lr.Error == nil {
			t.Error("expected non-null error on failure")
		}
	}},
	{"notify/431-too-large", "/notify", "POST", func(t *testing.T) {
		big := strings.Repeat("x", 4096)
		body := jsonBody(t, map[string]any{"urls": []string{"gotify://host/token"}, "body": big})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.MaxBodyBytes = 64 }))
		wantStatus(t, rec, 431)
	}},

	// -------------------------------------------------------------------
	// Endpoints apprize does not implement. The default mux returns 404.
	// -------------------------------------------------------------------
	{"cfg/404", "/cfg", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/cfg"}.do(newServer()), 404)
	}},
	{"add/404", "/add/", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/add/"}.do(newServer()), 404)
	}},
	{"addkey/404", "/add/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/add/mykey"}.do(newServer()), 404)
	}},
	{"del/404", "/del/", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/del/"}.do(newServer()), 404)
	}},
	{"delkey/404", "/del/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/del/mykey"}.do(newServer()), 404)
	}},
	{"get/404", "/get/", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/get/"}.do(newServer()), 404)
	}},
	{"getkey/404", "/get/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/get/mykey"}.do(newServer()), 404)
	}},
	{"cfgkey/404", "/cfg/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/cfg/mykey"}.do(newServer()), 404)
	}},
	{"notifykey/404", "/notify/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/notify/mykey"}.do(newServer()), 404)
	}},
	{"statuskey/404", "/status/{key}", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/status/mykey"}.do(newServer()), 404)
	}},
	{"jsonurls/404", "/json/urls/", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/json/urls/"}.do(newServer()), 404)
	}},
	{"jsonurlskey/404", "/json/urls/{key}", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/json/urls/mykey"}.do(newServer()), 404)
	}},
	{"move/404", "/move/", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/move/"}.do(newServer()), 404)
	}},
	{"movekey/404", "/move/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/move/mykey"}.do(newServer()), 404)
	}},
	{"qr/404", "/qr/", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/qr/"}.do(newServer()), 404)
	}},
	{"qrkey/404", "/qr/{key}", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/qr/mykey"}.do(newServer()), 404)
	}},
	{"auth-get/404", "/auth/", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/auth/"}.do(newServer()), 404)
	}},
	{"auth-post/404", "/auth/", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/auth/"}.do(newServer()), 404)
	}},
	{"auth-delete/404", "/auth/", "DELETE", func(t *testing.T) {
		wantStatus(t, req{method: "DELETE", path: "/auth/"}.do(newServer()), 404)
	}},
	{"authkey-get/404", "/auth/{key}", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/auth/mykey"}.do(newServer()), 404)
	}},
	{"authkey-post/404", "/auth/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/auth/mykey"}.do(newServer()), 404)
	}},
	{"authkey-delete/404", "/auth/{key}", "DELETE", func(t *testing.T) {
		wantStatus(t, req{method: "DELETE", path: "/auth/mykey"}.do(newServer()), 404)
	}},
	{"login-get/404", "/login", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/login"}.do(newServer()), 404)
	}},
	{"login-post/404", "/login", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/login"}.do(newServer()), 404)
	}},
	{"logout-get/404", "/logout", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/logout"}.do(newServer()), 404)
	}},
	{"logout-post/404", "/logout", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/logout"}.do(newServer()), 404)
	}},
	{"metrics/404", "/metrics", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/metrics"}.do(newServer()), 404)
	}},
}
