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
	"git.fogtype.com/nebel/apprize/internal/store"
)

// version is overridable at build time via -ldflags.
var version = "dev"

func main() {
	cfg := config.FromEnv()

	flag.StringVar(&cfg.Bind, "bind", cfg.Bind, "listen address (env APPRIZE_BIND)")
	flag.StringVar(&cfg.DBPath, "db", cfg.DBPath, "sqlite database path, :memory: for ephemeral (env APPRIZE_DB_PATH)")
	flag.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "optional API secret (env APPRIZE_API_KEY)")
	flag.Int64Var(&cfg.ConfigMaxKB, "config-max-length", cfg.ConfigMaxKB, "request body limit in KB (env APPRIZE_CONFIG_MAX_LENGTH)")
	flag.StringVar(&cfg.DefaultConfigID, "default-config-id", cfg.DefaultConfigID, "default key for keyless persistent routes (env APPRIZE_DEFAULT_CONFIG_ID)")
	flag.Parse()

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config.Config) error {
	var st store.Store
	if cfg.DBPath == "" || cfg.DBPath == ":memory:" {
		st = store.NewMemory()
	} else {
		s, err := store.OpenSQLite(cfg.DBPath)
		if err != nil {
			return err
		}
		st = s
	}
	defer st.Close()

	h := server.New(server.Deps{
		Notifier:        notify.NewApprise(),
		Store:           st,
		StatelessURLs:   cfg.StatelessURLs,
		ConfigLock:      cfg.ConfigLock,
		Admin:           cfg.Admin,
		RecursionMax:    cfg.RecursionMax,
		DenyServices:    cfg.DenyServices,
		AllowServices:   cfg.AllowServices,
		APIKey:          cfg.APIKey,
		MaxBodyBytes:    cfg.ConfigMaxKB * 1024,
		DefaultConfigID: cfg.DefaultConfigID,
		Version:         version,
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
