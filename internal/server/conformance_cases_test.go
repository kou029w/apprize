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
	{"notify/204-no-urls", "/notify", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify", contentType: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 204)
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
	// Persistent endpoints — intentionally omitted, 404 via the default mux.
	// -------------------------------------------------------------------
	{"cfg/404", "/cfg", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/cfg"}.do(newServer()), 404)
	}},
	{"add/404", "/add/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/add/mykey"}.do(newServer()), 404)
	}},
	{"del/404", "/del/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/del/mykey"}.do(newServer()), 404)
	}},
	{"get/404", "/get/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/get/mykey"}.do(newServer()), 404)
	}},
	{"cfgkey/404", "/cfg/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/cfg/mykey"}.do(newServer()), 404)
	}},
	{"notifykey/404", "/notify/{key}", "POST", func(t *testing.T) {
		wantStatus(t, req{method: "POST", path: "/notify/mykey"}.do(newServer()), 404)
	}},
	{"jsonurls/404", "/json/urls/{key}", "GET", func(t *testing.T) {
		wantStatus(t, req{method: "GET", path: "/json/urls/mykey"}.do(newServer()), 404)
	}},
}
