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
	"git.fogtype.com/nebel/apprize/internal/store"
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
	{"status/417-text-store-error", "/status", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/status", accept: ctText}.
			do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 417)
		wantContentType(t, rec, ctText)
		if !strings.Contains(rec.Body.String(), "CONFIG_PERMISSION_ISSUE") {
			t.Errorf("text status body = %q, want to contain CONFIG_PERMISSION_ISSUE", rec.Body.String())
		}
	}},
	{"status/417-json-store-error", "/status", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/status", accept: ctJSON}.
			do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 417)
		wantContentType(t, rec, ctJSON)
		var obj map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
			t.Fatalf("status body is not JSON: %v", err)
		}
		status, _ := obj["status"].(map[string]any)
		if ok, _ := status["persistent_storage"].(bool); ok {
			t.Fatalf("status.persistent_storage = %v, want false", status["persistent_storage"])
		}
		if can, _ := status["can_write_config"].(bool); can {
			t.Fatalf("status.can_write_config = %v, want false", status["can_write_config"])
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
	// GET /cfg
	// -------------------------------------------------------------------
	{"cfg/403-no-admin", "/cfg", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/cfg"}.do(newServer(func(d *server.Deps) { d.Admin = false }))
		wantStatus(t, rec, 403)
	}},
	{"cfg/200-admin-json", "/cfg", "GET", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		rec := req{method: "GET", path: "/cfg", accept: ctJSON}.
			do(newServer(func(d *server.Deps) { d.Admin = true; d.Store = st }))
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctJSON)
		validateSchema(t, "/cfg", "GET", "200", ctJSON, rec.Body.Bytes())
		var keys []string
		_ = json.Unmarshal(rec.Body.Bytes(), &keys)
		if len(keys) == 0 || keys[0] != "mykey" {
			t.Errorf("keys = %v, want [mykey]", keys)
		}
	}},
	{"cfg/200-admin-html", "/cfg", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/cfg", accept: ctHTML}.do(newServer(func(d *server.Deps) { d.Admin = true }))
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctHTML)
	}},

	// -------------------------------------------------------------------
	// POST /add/{key}
	// -------------------------------------------------------------------
	{"add/200-json-config", "/add/{key}", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"config": "gotify://host/token", "format": "text"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctJSON, accept: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 200)
		validateSchema(t, "/add/{key}", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"add/200-form-urls", "/add/{key}", "POST", func(t *testing.T) {
		body := formBody(map[string]string{"urls": "gotify://host/token"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctForm, accept: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 200)
	}},
	{"add/400-invalid", "/add/{key}", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"config": "urls: [unterminated", "format": "yaml"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 400)
	}},
	{"add/403-lock", "/add/{key}", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"config": "gotify://host/token", "format": "text"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.ConfigLock = true }))
		wantStatus(t, rec, 403)
	}},
	{"add/431-large", "/add/{key}", "POST", func(t *testing.T) {
		big := strings.Repeat("gotify://host/token\n", 1000)
		body := jsonBody(t, map[string]any{"config": big, "format": "text"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.MaxBodyBytes = 64 }))
		wantStatus(t, rec, 431)
	}},
	{"add/500-store-error", "/add/{key}", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"config": "gotify://host/token", "format": "text"})
		rec := req{method: "POST", path: "/add/mykey", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 500)
	}},

	// -------------------------------------------------------------------
	// POST /del/{key}
	// -------------------------------------------------------------------
	{"del/200-exists", "/del/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		rec := req{method: "POST", path: "/del/mykey"}.do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
	}},
	{"del/204-absent", "/del/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/del/missing"}.do(newServer())
		wantStatus(t, rec, 204)
	}},
	{"del/403-lock", "/del/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/del/mykey"}.do(newServer(func(d *server.Deps) { d.ConfigLock = true }))
		wantStatus(t, rec, 403)
	}},
	{"del/500-error", "/del/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/del/mykey"}.do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 500)
	}},

	// -------------------------------------------------------------------
	// POST /get/{key}
	// -------------------------------------------------------------------
	{"get/200-json", "/get/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		rec := req{method: "POST", path: "/get/mykey", accept: ctJSON}.do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctJSON)
		validateSchema(t, "/get/{key}", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"get/200-text", "/get/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		rec := req{method: "POST", path: "/get/mykey", accept: ctText}.do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctText)
		if !strings.Contains(rec.Body.String(), "gotify://host/token") {
			t.Errorf("text config body = %q", rec.Body.String())
		}
	}},
	{"get/204-absent", "/get/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/get/missing"}.do(newServer())
		wantStatus(t, rec, 204)
	}},
	{"get/403-lock", "/get/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/get/mykey"}.do(newServer(func(d *server.Deps) { d.ConfigLock = true }))
		wantStatus(t, rec, 403)
	}},
	{"get/500-error", "/get/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/get/mykey"}.do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 500)
	}},

	// -------------------------------------------------------------------
	// POST /cfg/{key} (alias for /get/{key})
	// -------------------------------------------------------------------
	{"cfgkey/200-json", "/cfg/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		rec := req{method: "POST", path: "/cfg/mykey", accept: ctJSON}.do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		validateSchema(t, "/cfg/{key}", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"cfgkey/204-absent", "/cfg/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/cfg/missing"}.do(newServer())
		wantStatus(t, rec, 204)
	}},
	{"cfgkey/403-lock", "/cfg/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/cfg/mykey"}.do(newServer(func(d *server.Deps) { d.ConfigLock = true }))
		wantStatus(t, rec, 403)
	}},
	{"cfgkey/500-error", "/cfg/{key}", "POST", func(t *testing.T) {
		rec := req{method: "POST", path: "/cfg/mykey"}.do(newServer(func(d *server.Deps) { d.Store = errStore{} }))
		wantStatus(t, rec, 500)
	}},

	// -------------------------------------------------------------------
	// POST /notify/{key}
	// -------------------------------------------------------------------
	{"notifykey/200", "/notify/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify/mykey", contentType: ctJSON, accept: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		validateSchema(t, "/notify/{key}", "POST", "200", ctJSON, rec.Body.Bytes())
	}},
	{"notifykey/204-absent", "/notify/{key}", "POST", func(t *testing.T) {
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify/missing", contentType: ctJSON, body: body}.do(newServer())
		wantStatus(t, rec, 204)
	}},
	{"notifykey/406-recursion", "/notify/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify/mykey", contentType: ctJSON, body: body,
			headers: map[string]string{"X-Apprise-Recursion-Count": "5"}}.
			do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 406)
	}},
	{"notifykey/424-failure", "/notify/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		body := jsonBody(t, map[string]any{"body": "hi"})
		rec := req{method: "POST", path: "/notify/mykey", contentType: ctJSON, accept: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Store = st; d.Notifier = failNotifier() }))
		wantStatus(t, rec, 424)
	}},
	{"notifykey/431-large", "/notify/{key}", "POST", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/token"})
		big := strings.Repeat("x", 4096)
		body := jsonBody(t, map[string]any{"body": big})
		rec := req{method: "POST", path: "/notify/mykey", contentType: ctJSON, body: body}.
			do(newServer(func(d *server.Deps) { d.Store = st; d.MaxBodyBytes = 64 }))
		wantStatus(t, rec, 431)
	}},

	// -------------------------------------------------------------------
	// GET /json/urls/{key}
	// -------------------------------------------------------------------
	{"jsonurls/200", "/json/urls/{key}", "GET", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "alerts=gotify://host/token"})
		rec := req{method: "GET", path: "/json/urls/mykey", accept: ctJSON}.do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		wantContentType(t, rec, ctJSON)
		validateSchema(t, "/json/urls/{key}", "GET", "200", ctJSON, rec.Body.Bytes())
	}},
	{"jsonurls/200-privacy", "/json/urls/{key}", "GET", func(t *testing.T) {
		st := store.NewMemory()
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: "gotify://host/SECRETTOKEN"})
		rec := req{method: "GET", path: "/json/urls/mykey?privacy=1", accept: ctJSON}.
			do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		if strings.Contains(rec.Body.String(), "SECRETTOKEN") {
			t.Error("privacy=1 should redact secrets, but SECRETTOKEN is present")
		}
	}},
	{"jsonurls/200-tagfilter", "/json/urls/{key}", "GET", func(t *testing.T) {
		st := store.NewMemory()
		cfg := "alerts=gotify://host/token1\nteam=discord://id/token2"
		seed(t, st, store.Entry{Key: "mykey", Format: "text", Config: cfg})
		rec := req{method: "GET", path: "/json/urls/mykey?tag=alerts", accept: ctJSON}.
			do(newServer(func(d *server.Deps) { d.Store = st }))
		wantStatus(t, rec, 200)
		body := rec.Body.String()
		if !strings.Contains(body, "token1") || strings.Contains(body, "token2") {
			t.Errorf("tag=alerts should return only the alerts URL; body=%q", truncate(body, 300))
		}
	}},
	{"jsonurls/204-absent", "/json/urls/{key}", "GET", func(t *testing.T) {
		rec := req{method: "GET", path: "/json/urls/missing", accept: ctJSON}.do(newServer())
		wantStatus(t, rec, 204)
	}},
}
