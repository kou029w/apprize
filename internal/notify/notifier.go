// Package notify abstracts the notification backend so that HTTP handlers can
// be tested without performing real network calls.
package notify

import "context"

// Options carries the per-notification settings shared by every backend.
type Options struct {
	Title       string // optional message title
	Type        string // info|success|warning|failure (empty = info)
	InputFormat string // text|markdown|html (empty = text)
}

// Result describes the outcome of delivering to a single target URL.
type Result struct {
	URL     string // the (unmasked) target URL
	Service string // service name derived from the URL scheme
	OK      bool   // whether delivery succeeded
	Message string // human readable detail (error text on failure)
}

// Notifier delivers a notification body to a set of Apprise URLs.
//
// Implementations must be safe for concurrent use. The real implementation
// wraps github.com/unraid/apprise-go; tests inject a fake.
type Notifier interface {
	Notify(ctx context.Context, urls []string, body string, opts Options) ([]Result, error)
}
