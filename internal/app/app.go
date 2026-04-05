package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/fetcher"
	"github.com/batkiz/rss-gateway/internal/httpapi"
	"github.com/batkiz/rss-gateway/internal/pipeline"
	"github.com/batkiz/rss-gateway/internal/storage"
)

type App struct {
	store    *storage.SQLiteStore
	service  *pipeline.Service
	handler  *httpapi.Handler
	cancelFn context.CancelFunc
}

func New(cfg config.Config) (*App, error) {
	log.Printf("opening sqlite storage path=%s", cfg.Storage.Path)
	store, err := storage.NewSQLiteStore(cfg.Storage.Path)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}
	if err := store.SeedRuntimeConfig(context.Background(), cfg); err != nil {
		return nil, fmt.Errorf("seed runtime config: %w", err)
	}
	log.Printf("runtime config checked from toml seed sources=%d modes=%d", len(cfg.Sources), len(cfg.Modes))

	log.Printf("initializing feed fetcher timeout=%s", (30 * time.Second).String())
	fetch := fetcher.New(30 * time.Second)

	settings, err := store.GetLLMSettings(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load llm settings: %w", err)
	}
	log.Printf("runtime llm settings provider=%s model=%s base_url=%s", settings.Provider, settings.Model, settings.BaseURL)

	service := pipeline.NewService(fetch, store)
	handler := httpapi.New(service)
	sources, err := service.ListSources(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	log.Printf("http handler initialized sources=%d", len(sources))
	return &App{
		store:   store,
		service: service,
		handler: handler,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	a.cancelFn = cancel

	log.Printf("starting background scheduler")
	go a.runScheduler(runCtx)

	log.Printf("queueing initial refresh for enabled sources")
	go func() {
		refreshCtx, refreshCancel := context.WithTimeout(runCtx, 3*time.Minute)
		defer refreshCancel()
		log.Printf("initial refresh started")
		if err := a.service.RefreshAll(refreshCtx); err != nil {
			log.Printf("initial refresh failed: %v", err)
			return
		}
		log.Printf("initial refresh complete")
	}()
	return nil
}

func (a *App) Router() chi.Router {
	return a.handler.Router()
}

func (a *App) Close() error {
	if a.cancelFn != nil {
		a.cancelFn()
	}
	log.Printf("closing storage")
	return a.store.Close()
}

func (a *App) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	log.Printf("scheduler polling loop started interval=1m")

	for {
		select {
		case <-ctx.Done():
			log.Printf("scheduler shutting down")
			return
		case <-ticker.C:
			a.runScheduledRefreshes(ctx)
		}
	}
}

func (a *App) runScheduledRefreshes(ctx context.Context) {
	sources, err := a.service.ListSources(ctx)
	if err != nil {
		log.Printf("scheduler list sources: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		state, err := a.service.FeedState(ctx, source.ID)
		if err != nil {
			log.Printf("scheduler state source=%s: %v", source.ID, err)
			continue
		}
		if !state.LastSuccessAt.IsZero() && now.Sub(state.LastSuccessAt) < source.RefreshInterval {
			continue
		}
		log.Printf("scheduled refresh start source=%s interval=%s", source.ID, source.RefreshInterval)
		refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		err = a.service.RefreshSource(refreshCtx, source.ID)
		cancel()
		if err != nil {
			log.Printf("scheduled refresh %s: %v", source.ID, err)
			continue
		}
		log.Printf("scheduled refresh complete source=%s", source.ID)
	}
}
