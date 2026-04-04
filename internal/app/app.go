package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/fetcher"
	"github.com/batkiz/rss-gateway/internal/httpapi"
	"github.com/batkiz/rss-gateway/internal/llm"
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

	log.Printf("initializing feed fetcher timeout=%s", (30 * time.Second).String())
	fetch := fetcher.New(30 * time.Second)

	var processor llm.Processor
	switch cfg.LLM.Provider {
	case "openai":
		log.Printf("initializing llm provider=%s model=%s base_url=%s", cfg.LLM.Provider, cfg.LLM.Model, cfg.LLM.BaseURL)
		processor, err = llm.NewOpenAIProcessor(cfg.LLM)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.LLM.Provider)
	}

	service := pipeline.NewService(cfg, fetch, processor, store)
	handler := httpapi.New(service)
	log.Printf("http handler initialized sources=%d", len(service.Sources()))
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

func (a *App) Router() http.Handler {
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
	tickers := make(map[string]*time.Ticker)
	for id, source := range a.service.Sources() {
		if !source.Enabled {
			log.Printf("scheduler skip source=%s enabled=false", id)
			continue
		}
		ticker := time.NewTicker(source.RefreshInterval)
		tickers[id] = ticker
		log.Printf("scheduler registered source=%s interval=%s", id, source.RefreshInterval)
		go func(sourceID string, ticker *time.Ticker) {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					log.Printf("scheduler worker stopping source=%s", sourceID)
					return
				case <-ticker.C:
					log.Printf("scheduled refresh start source=%s", sourceID)
					refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
					err := a.service.RefreshSource(refreshCtx, sourceID)
					cancel()
					if err != nil {
						log.Printf("scheduled refresh %s: %v", sourceID, err)
					} else {
						log.Printf("scheduled refresh complete source=%s", sourceID)
					}
				}
			}
		}(id, ticker)
	}

	<-ctx.Done()
	log.Printf("scheduler shutting down")
	for _, ticker := range tickers {
		ticker.Stop()
	}
}
