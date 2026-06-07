package server_test

import (
	"strings"
	"testing"

	"git.fogtype.com/nebel/apprize/internal/server"
)

func TestDefaultConfigID_KeylessPersistentRoutes(t *testing.T) {
	body := jsonBody(t, map[string]any{"config": "gotify://host/token", "format": "text"})
	h := newServer(func(d *server.Deps) {
		d.DefaultConfigID = "mydefault"
	})

	addRec := req{method: "POST", path: "/add", contentType: ctJSON, accept: ctJSON, body: body}.do(h)
	wantStatus(t, addRec, 200)

	getRec := req{method: "POST", path: "/get", accept: ctText}.do(h)
	wantStatus(t, getRec, 200)
	if !strings.Contains(getRec.Body.String(), "gotify://host/token") {
		t.Fatalf("keyless /get response = %q", getRec.Body.String())
	}

	cfgRec := req{method: "POST", path: "/cfg", accept: ctText}.do(h)
	wantStatus(t, cfgRec, 200)
	if !strings.Contains(cfgRec.Body.String(), "gotify://host/token") {
		t.Fatalf("keyless /cfg response = %q", cfgRec.Body.String())
	}

	delRec := req{method: "POST", path: "/del"}.do(h)
	wantStatus(t, delRec, 200)

	getAfterDelete := req{method: "POST", path: "/get"}.do(h)
	wantStatus(t, getAfterDelete, 204)
}

func TestDefaultConfigID_UsesFallbackWhenUnset(t *testing.T) {
	body := jsonBody(t, map[string]any{"config": "gotify://host/token", "format": "text"})
	h := newServer(func(d *server.Deps) {
		d.DefaultConfigID = ""
	})

	addRec := req{method: "POST", path: "/add", contentType: ctJSON, body: body}.do(h)
	wantStatus(t, addRec, 200)

	getRec := req{method: "POST", path: "/get", accept: ctText}.do(h)
	wantStatus(t, getRec, 200)
	if !strings.Contains(getRec.Body.String(), "gotify://host/token") {
		t.Fatalf("keyless /get response = %q", getRec.Body.String())
	}
}
