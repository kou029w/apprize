package notify

import (
	"context"
	"errors"
	"strings"

	apprise "github.com/unraid/apprise-go"
)

// Apprise is the production Notifier backed by github.com/unraid/apprise-go.
type Apprise struct{}

// NewApprise returns a Notifier that delivers via apprise-go.
func NewApprise() *Apprise { return &Apprise{} }

// SupportedSchemas returns the notification URL schemas this build can deliver.
//
// It is a static, network-free capability list (not tied to any Notifier
// instance), so meta endpoints can report it without depending on apprise-go
// directly — the dependency stays confined to this package.
func SupportedSchemas() []string { return apprise.SupportedSchemas() }

// Notify builds a fresh apprise client for the given URLs and sends body.
//
// apprise-go's Send returns errors.Join(*apprise.TargetError...) on partial or
// total failure; we unwrap that to report a per-URL Result.
func (a *Apprise) Notify(_ context.Context, urls []string, body string, opts Options) ([]Result, error) {
	client := apprise.New()
	added := make([]string, 0, len(urls))
	results := make([]Result, 0, len(urls))
	for _, u := range urls {
		if err := client.Add(u); err != nil {
			results = append(results, Result{
				URL:     u,
				Service: schemeOf(u),
				OK:      false,
				Message: err.Error(),
			})
			continue
		}
		added = append(added, u)
	}

	if len(added) == 0 {
		return results, nil
	}

	var sendOpts []apprise.Option
	if opts.Title != "" {
		sendOpts = append(sendOpts, apprise.WithTitle(opts.Title))
	}
	if opts.Type != "" {
		if t, ok := apprise.ParseNotifyType(opts.Type); ok {
			sendOpts = append(sendOpts, apprise.WithNotifyType(t))
		}
	}
	if opts.InputFormat != "" {
		sendOpts = append(sendOpts, apprise.WithInputFormat(opts.InputFormat))
	}

	err := client.Send(body, sendOpts...)
	failed := failuresByURL(err)
	for _, u := range added {
		if msg, bad := failed[u]; bad {
			results = append(results, Result{URL: u, Service: schemeOf(u), OK: false, Message: msg})
		} else if err != nil {
			results = append(results, Result{URL: u, Service: schemeOf(u), OK: false, Message: err.Error()})
		} else {
			results = append(results, Result{URL: u, Service: schemeOf(u), OK: true, Message: "Sent"})
		}
	}
	return results, nil
}

// failuresByURL extracts per-target failures from the (possibly joined) error
// returned by apprise-go's Send.
func failuresByURL(err error) map[string]string {
	out := map[string]string{}
	if err == nil {
		return out
	}
	var targets []error
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		targets = joined.Unwrap()
	} else {
		targets = []error{err}
	}
	for _, e := range targets {
		var te *apprise.TargetError
		if errors.As(e, &te) {
			out[te.URL] = te.Err.Error()
		}
	}
	return out
}

// schemeOf returns the URL scheme (best effort) used as a fallback service name.
func schemeOf(rawURL string) string {
	if i := strings.Index(rawURL, "://"); i > 0 {
		return rawURL[:i]
	}
	if i := strings.Index(rawURL, ":"); i > 0 {
		return rawURL[:i]
	}
	return rawURL
}
