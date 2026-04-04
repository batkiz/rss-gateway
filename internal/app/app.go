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
	store, err := storage.NewSQLiteStore(cfg.Storage.Path)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	fetch := fetcher.New(30 * time.Second)

	var processor llm.Processor
	switch cfg.LLM.Provider {
	case "openai":
		processor, err = llm.NewOpenAIProcessor(cfg.LLM)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.LLM.Provider)
	}

	service := pipeline.NewService(cfg, fetch, processor, store)
	handler := httpapi.New(service)
	return &App{
		store:   store,
		service: service,
		handler: handler,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	a.cancelFn = cancel

	go a.runScheduler(runCtx)

	refreshCtx, refreshCancel := context.WithTimeout(runCtx, 3*time.Minute)
	defer refreshCancel()
	return a.service.RefreshAll(refreshCtx)
}

func (a *App) Router() http.Handler {
	return a.handler.Router()
}

func (a *App) Close() error {
	if a.cancelFn != nil {
		a.cancelFn()
	}
	return a.store.Close()
}

func (a *App) runScheduler(ctx context.Context) {
	tickers := make(map[string]*time.Ticker)
	for id, source := range a.service.Sources() {
		if !source.Enabled {
			continue
		}
		ticker := time.NewTicker(source.RefreshInterval)
		tickers[id] = ticker
		go func(sourceID string, ticker *time.Ticker) {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
					err := a.service.RefreshSource(refreshCtx, sourceID)
					cancel()
					if err != nil {
						log.Printf("scheduled refresh %s: %v", sourceID, err)
					}
				}
			}
		}(id, ticker)
	}

	<-ctx.Done()
	for _, ticker := range tickers {
		ticker.Stop()
	}
}
