package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/batkiz/rss-gateway/internal/app"
	"github.com/batkiz/rss-gateway/internal/config"
)

func main() {
	configPath := flag.String("config", "configs/config.example.toml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("config loaded path=%s sources=%d provider=%s storage=%s", *configPath, len(cfg.Sources), cfg.LLM.Provider, cfg.Storage.Path)

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}
	defer application.Close()
	log.Printf("application initialized")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Start(ctx); err != nil {
		log.Fatalf("start app: %v", err)
	}
	logStartupInfo(cfg)

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           application.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("rss-gateway listening on %s", cfg.Server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("http server stopped")
}

func logStartupInfo(cfg config.Config) {
	log.Printf("http endpoints health=%s sources=%s admin=%s status=%s", "/healthz", "/sources", "/admin", "/admin/status")

	sourceIDs := make([]string, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	sort.Strings(sourceIDs)
	for _, id := range sourceIDs {
		log.Printf("feed endpoint source=%s path=/feeds/%s.rss", id, id)
	}
}
