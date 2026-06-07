package server

import (
	"bytes"
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// withMiddlewareChain composes the cross-cutting middleware stack.
// Execution order (outer → inner): requestLog → recover → apiKey → recursion → bodyLimit → handler.
func withMiddlewareChain(s *server, h http.Handler) http.Handler {
	h = bodyLimitMiddleware(s.maxBodyBytes())(h)
	h = recursionMiddleware(s.RecursionMax)(h)
	if s.APIKey != "" {
		h = apiKeyMiddleware(s.APIKey)(h)
	}
	h = recoverMiddleware(h)
	h = requestLogMiddleware(h)
	return h
}

// maxBodyBytes returns the effective request-body limit. A zero Deps value
// gets a sensible default so middleware never runs unbounded.
func (s *server) maxBodyBytes() int64 {
	if s.MaxBodyBytes > 0 {
		return s.MaxBodyBytes
	}
	return 512 * 1024 // 512 KB, matching APPRIZE_CONFIG_MAX_LENGTH default
}

// recoverMiddleware converts panics into 500 responses so a single bad request
// cannot take down the server.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps a ResponseWriter so we can capture the status code
// for request logging without interfering with the real response.
type responseRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rr *responseRecorder) WriteHeader(status int) {
	if !rr.wrote {
		rr.status = status
		rr.wrote = true
		rr.ResponseWriter.WriteHeader(status)
	}
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if !rr.wrote {
		rr.WriteHeader(http.StatusOK)
	}
	return rr.ResponseWriter.Write(b)
}

// Flush delegates to the wrapped ResponseWriter if it implements http.Flusher.
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// requestLogMiddleware logs every request with method, path, status and duration.
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rr, r)
		status := rr.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), status, time.Since(start))
	})
}

// bodyLimitMiddleware rejects requests whose body exceeds limit bytes with 431.
// It reads the body up-front so downstream handlers see a fully populated r.Body.
func bodyLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit <= 0 || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			// Allow an extra byte so we can tell "exactly at limit" from "over limit".
			body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
			_ = r.Body.Close()
			if err != nil {
				http.Error(w, "unable to read request body", http.StatusBadRequest)
				return
			}
			if int64(len(body)) > limit {
				http.Error(w, "request body too large", http.StatusRequestHeaderFieldsTooLarge)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			next.ServeHTTP(w, r)
		})
	}
}

// apiKeyMiddleware enforces a simple shared secret when APIKey is non-empty.
// Clients may present the key via `Authorization: Bearer <key>` or `X-API-Key: <key>`.
func apiKeyMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if provided == "" {
				auth := r.Header.Get("Authorization")
				const prefix = "Bearer "
				if strings.HasPrefix(auth, prefix) {
					provided = strings.TrimSpace(auth[len(prefix):])
				}
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// recursionMiddleware blocks requests whose X-Apprise-Recursion-Count header
// exceeds the configured maximum, returning 406 Not Acceptable per swagger.
func recursionMiddleware(max int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := 0
			if h := r.Header.Get("X-Apprise-Recursion-Count"); h != "" {
				if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
					count = n
				}
			}
			if count > max {
				http.Error(w, "recursion limit reached", http.StatusNotAcceptable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
