// Command apprize runs an Apprise-API-compatible notification server.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"git.fogtype.com/nebel/apprize/internal/config"
	"git.fogtype.com/nebel/apprize/internal/notify"
	"git.fogtype.com/nebel/apprize/internal/server"
)

// version is overridable at build time via -ldflags.
var version = "dev"

func main() {
	cfg := config.FromEnv()

	flag.StringVar(&cfg.Bind, "bind", cfg.Bind, "listen address (env APPRIZE_BIND)")
	flag.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "optional API secret (env APPRIZE_API_KEY)")
	flag.Parse()

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config.Config) error {
	h := server.New(server.Deps{
		Notifier:      notify.NewApprise(),
		StatelessURLs: cfg.StatelessURLs,
		RecursionMax:  cfg.RecursionMax,
		DenyServices:  cfg.DenyServices,
		AllowServices: cfg.AllowServices,
		APIKey:        cfg.APIKey,
		Version:       version,
	})

	srv := &http.Server{
		Addr:              cfg.Bind,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("apprize %s listening on %s", version, cfg.Bind)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
