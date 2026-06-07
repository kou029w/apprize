package server_test

// This file is the conformance test harness. It treats testdata/swagger.yaml as
// the single source of truth for the API contract and verifies, for every
// documented operation, that:
//   - the route is reachable (not 404/405),
//   - each documented status code can be produced,
//   - content negotiation returns the documented media types,
//   - JSON response bodies validate against the swagger schema definitions.
//
// TestSpecCoverage is a meta-test asserting the case table covers the whole
// spec.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.fogtype.com/nebel/apprize/internal/notify"
	"git.fogtype.com/nebel/apprize/internal/server"
	"git.fogtype.com/nebel/apprize/internal/store"
	"github.com/getkin/kin-openapi/openapi3"
)

const specFile = "../../testdata/swagger.yaml"

// specDoc is the parsed swagger document shared by all tests.
var specDoc = mustLoadSpec()

func mustLoadSpec() *openapi3.T {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specFile)
	if err != nil {
		panic("loading swagger: " + err.Error())
	}
	return doc
}

// ---------------------------------------------------------------------------
// Fake backends
// ---------------------------------------------------------------------------

// notifierFunc adapts a function to the notify.Notifier interface.
type notifierFunc func(context.Context, []string, string, notify.Options) ([]notify.Result, error)

func (f notifierFunc) Notify(ctx context.Context, urls []string, body string, opts notify.Options) ([]notify.Result, error) {
	return f(ctx, urls, body, opts)
}

// okNotifier reports success for every URL.
func okNotifier() notify.Notifier {
	return notifierFunc(func(_ context.Context, urls []string, _ string, _ notify.Options) ([]notify.Result, error) {
		res := make([]notify.Result, len(urls))
		for i, u := range urls {
			res[i] = notify.Result{URL: u, Service: "test", OK: true, Message: "Sent"}
		}
		return res, nil
	})
}

// failNotifier reports failure for every URL (drives the 424 path).
func failNotifier() notify.Notifier {
	return notifierFunc(func(_ context.Context, urls []string, _ string, _ notify.Options) ([]notify.Result, error) {
		res := make([]notify.Result, len(urls))
		for i, u := range urls {
			res[i] = notify.Result{URL: u, Service: "test", OK: false, Message: "boom"}
		}
		return res, nil
	})
}

var errBoom = errors.New("boom")

// errStore fails every operation (drives the 500 path).
type errStore struct{}

func (errStore) Get(context.Context, string) (store.Entry, bool, error) {
	return store.Entry{}, false, errBoom
}
func (errStore) Put(context.Context, store.Entry) error       { return errBoom }
func (errStore) Delete(context.Context, string) (bool, error) { return false, errBoom }
func (errStore) List(context.Context) ([]string, error)       { return nil, errBoom }
func (errStore) Close() error                                 { return nil }

// ---------------------------------------------------------------------------
// Server construction
// ---------------------------------------------------------------------------

// baseDeps returns sane defaults for a test server.
func baseDeps() server.Deps {
	return server.Deps{
		Notifier:     okNotifier(),
		Store:        store.NewMemory(),
		RecursionMax: 1,
		Version:      "test",
	}
}

// newServer builds a handler, applying the given Deps mutators in order.
func newServer(mods ...func(*server.Deps)) http.Handler {
	d := baseDeps()
	for _, m := range mods {
		m(&d)
	}
	return server.New(d)
}

// seed stores an entry in the given store, failing the test on error.
func seed(t *testing.T, st store.Store, e store.Entry) {
	t.Helper()
	if err := st.Put(context.Background(), e); err != nil {
		t.Fatalf("seeding store: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

type req struct {
	method      string
	path        string
	contentType string
	accept      string
	body        []byte
	headers     map[string]string
}

func (r req) do(h http.Handler) *httptest.ResponseRecorder {
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	if r.contentType != "" {
		hr.Header.Set("Content-Type", r.contentType)
	}
	if r.accept != "" {
		hr.Header.Set("Accept", r.accept)
	}
	for k, v := range r.headers {
		hr.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, hr)
	return rec
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func multipartBody(t *testing.T, fields map[string]string, files map[string][]byte) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	for name, content := range files {
		fw, err := w.CreateFormFile("attach", name)
		if err != nil {
			t.Fatalf("create file: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return w.FormDataContentType(), buf.Bytes()
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Errorf("status = %d, want %d (body: %q)", rec.Code, code, truncate(rec.Body.String(), 200))
	}
}

func wantContentType(t *testing.T, rec *httptest.ResponseRecorder, prefix string) {
	t.Helper()
	got := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("Content-Type = %q, want prefix %q", got, prefix)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// validateSchema validates a JSON body against the swagger response schema for
// the given operation/status/content-type. Skips silently if no schema is
// documented for that combination.
func validateSchema(t *testing.T, specPath, method, status, contentType string, body []byte) {
	t.Helper()
	schema := responseSchema(specPath, method, status, contentType)
	if schema == nil {
		return
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Errorf("response is not valid JSON: %v (body: %q)", err, truncate(string(body), 200))
		return
	}
	if err := schema.VisitJSON(v); err != nil {
		t.Errorf("response does not match swagger schema %s %s %s: %v", method, specPath, status, err)
	}
}

func responseSchema(specPath, method, status, contentType string) *openapi3.Schema {
	pi := specDoc.Paths.Find(specPath)
	if pi == nil {
		return nil
	}
	op := pi.GetOperation(strings.ToUpper(method))
	if op == nil || op.Responses == nil {
		return nil
	}
	rr := op.Responses.Map()[status]
	if rr == nil || rr.Value == nil {
		return nil
	}
	mt := rr.Value.Content[contentType]
	if mt == nil || mt.Schema == nil {
		return nil
	}
	return mt.Schema.Value
}

// ---------------------------------------------------------------------------
// Spec coverage meta-test
// ---------------------------------------------------------------------------

// TestSpecCoverage asserts the conformance case table exercises every
// (path, method) documented in swagger.yaml, and contains no operation that is
// absent from the spec.
func TestSpecCoverage(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range conformanceCases {
		covered[c.method+" "+c.specPath] = true
	}

	documented := map[string]bool{}
	for _, p := range specDoc.Paths.InMatchingOrder() {
		pi := specDoc.Paths.Find(p)
		for method := range pi.Operations() {
			documented[method+" "+p] = true
		}
	}

	for op := range documented {
		if !covered[op] {
			t.Errorf("operation %q is documented in swagger but not covered by any conformance case", op)
		}
	}
	for op := range covered {
		if !documented[op] {
			t.Errorf("conformance case targets %q which is not in swagger", op)
		}
	}
}

// TestConformance runs every conformance case as a subtest.
func TestConformance(t *testing.T) {
	for _, c := range conformanceCases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t)
		})
	}
}
