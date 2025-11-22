package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/neo/kokoa-proxy/control-plane/internal/api"
	"github.com/neo/kokoa-proxy/control-plane/internal/db"
)

type config struct {
	ListenAddr     string
	DBPath         string
	BootstrapToken string
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := loadConfig()
	logger := log.New(os.Stdout, "kokoa-cp ", log.LstdFlags|log.LUTC)

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		logger.Fatalf("failed to create data dir: %v", err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		logger.Fatalf("failed to migrate database: %v", err)
	}

	server := api.NewServer(api.ServerConfig{
		Store:          store,
		BootstrapToken: cfg.BootstrapToken,
		Logger:         logger,
	})

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      server.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Printf("control plane listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func loadConfig() config {
	return config{
		ListenAddr:     envDefault("CP_LISTEN_ADDR", ":8080"),
		DBPath:         envDefault("CP_DB_PATH", filepath.Join("data", "kokoa.db")),
		BootstrapToken: envDefault("CP_BOOTSTRAP_TOKEN", ""),
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
