package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okHandler writes 200 + "ok" and is used as the inner handler for middleware
// unit tests.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
}

func TestBodyLimitMiddleware_AllowsUnderLimit(t *testing.T) {
	h := bodyLimitMiddleware(100)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBodyLimitMiddleware_RejectsOverLimit(t *testing.T) {
	h := bodyLimitMiddleware(4)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader("hello world"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestHeaderFieldsTooLarge)
	}
}

func TestRecursionMiddleware_AllowsUnderLimit(t *testing.T) {
	h := recursionMiddleware(5)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Apprise-Recursion-Count", "3")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRecursionMiddleware_RejectsOverLimit(t *testing.T) {
	h := recursionMiddleware(1)(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Apprise-Recursion-Count", "5")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotAcceptable)
	}
}

func TestAPIKeyMiddleware_SkipsWhenEmpty(t *testing.T) {
	h := apiKeyMiddleware("")(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_AcceptsXAPIKey(t *testing.T) {
	h := apiKeyMiddleware("secret")(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_AcceptsBearer(t *testing.T) {
	h := apiKeyMiddleware("secret")(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_RejectsBadKey(t *testing.T) {
	h := apiKeyMiddleware("secret")(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestWithMiddlewareChain_LogsPanicAs500(t *testing.T) {
	var buf bytes.Buffer
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	}()

	s := &server{Deps: Deps{RecursionMax: 3}}
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	h := withMiddlewareChain(s, panicHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	logs := buf.String()
	if !strings.Contains(logs, "panic: boom") {
		t.Fatalf("expected panic log, got %q", logs)
	}
	if !strings.Contains(logs, "GET /panic 500 ") {
		t.Fatalf("expected request log with 500 status, got %q", logs)
	}
}
