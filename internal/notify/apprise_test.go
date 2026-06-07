package notify

import (
	"errors"
	"testing"

	apprise "github.com/unraid/apprise-go"
)

// TestFailuresByURL checks that the per-target failures are recovered from the
// joined error apprise-go returns, while successes and non-target errors are
// handled gracefully.
func TestFailuresByURL(t *testing.T) {
	t.Run("nil error has no failures", func(t *testing.T) {
		if got := failuresByURL(nil); len(got) != 0 {
			t.Fatalf("failures = %v, want empty", got)
		}
	})

	t.Run("joined target errors map per URL", func(t *testing.T) {
		joined := errors.Join(
			&apprise.TargetError{URL: "discord://a/b", Err: errors.New("401")},
			&apprise.TargetError{URL: "mailto://x@y", Err: errors.New("timeout")},
		)
		got := failuresByURL(joined)
		if got["discord://a/b"] != "401" || got["mailto://x@y"] != "timeout" {
			t.Fatalf("failures = %v", got)
		}
	})

	t.Run("single target error", func(t *testing.T) {
		got := failuresByURL(&apprise.TargetError{URL: "gotify://h/t", Err: errors.New("boom")})
		if got["gotify://h/t"] != "boom" {
			t.Fatalf("failures = %v", got)
		}
	})
}

func TestSchemeOf(t *testing.T) {
	cases := map[string]string{
		"discord://id/token": "discord",
		"mailto://u@h":       "mailto",
		"weird":              "weird",
	}
	for in, want := range cases {
		if got := schemeOf(in); got != want {
			t.Errorf("schemeOf(%q) = %q, want %q", in, got, want)
		}
	}
}
