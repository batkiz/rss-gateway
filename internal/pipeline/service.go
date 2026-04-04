package pipeline

import (
	"context"
	"log"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/fetcher"
	"github.com/batkiz/rss-gateway/internal/llm"
	"github.com/batkiz/rss-gateway/internal/model"
	"github.com/batkiz/rss-gateway/internal/storage"
)

type Store interface {
	HasProcessedItem(ctx context.Context, sourceID, guid string) (bool, error)
	UpsertProcessedItem(ctx context.Context, item model.ProcessedItem) error
	ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error)
	UpdateFeedState(ctx context.Context, sourceID string, successAt *time.Time, lastError string) error
}

type Service struct {
	sources   map[string]model.Source
	modes     map[string]config.ModeConfig
	fetcher   *fetcher.Fetcher
	processor llm.Processor
	store     Store
}

func NewService(cfg config.Config, fetcher *fetcher.Fetcher, processor llm.Processor, store *storage.SQLiteStore) *Service {
	sources := make(map[string]model.Source, len(cfg.Sources))
	for _, source := range cfg.Sources {
		enabled := true
		if source.Enabled != nil {
			enabled = *source.Enabled
		}
		sources[source.ID] = model.Source{
			ID:              source.ID,
			Name:            source.Name,
			URL:             source.URL,
			RefreshInterval: source.RefreshInterval.Duration,
			Enabled:         enabled,
			MaxItems:        source.MaxItems,
			PipelineMode:    source.Pipeline.Mode,
			SystemPrompt:    source.Pipeline.SystemPrompt,
			TaskPrompt:      source.Pipeline.TaskPrompt,
			MaxInputChars:   source.Pipeline.MaxInputChars,
		}
	}
	return &Service{
		sources:   sources,
		modes:     cfg.Modes,
		fetcher:   fetcher,
		processor: processor,
		store:     store,
	}
}

func (s *Service) Sources() map[string]model.Source {
	return s.sources
}

func (s *Service) RefreshAll(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for _, source := range s.sources {
		source := source
		if !source.Enabled {
			continue
		}
		group.Go(func() error {
			if err := s.RefreshSource(groupCtx, source.ID); err != nil {
				log.Printf("refresh source %s: %v", source.ID, err)
			}
			return nil
		})
	}
	return group.Wait()
}

func (s *Service) RefreshSource(ctx context.Context, sourceID string) error {
	source, ok := s.sources[sourceID]
	if !ok {
		return nil
	}

	items, err := s.fetcher.Fetch(ctx, source)
	if err != nil {
		_ = s.store.UpdateFeedState(ctx, source.ID, nil, err.Error())
		return err
	}

	for _, item := range items {
		exists, err := s.store.HasProcessedItem(ctx, source.ID, item.GUID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		resp, err := s.processor.Process(ctx, model.ProcessRequest{
			Mode:          source.PipelineMode,
			Title:         fallbackTitle(item.Title, item.Link),
			Link:          item.Link,
			Content:       item.Content,
			SystemPrompt:  firstNonEmpty(source.SystemPrompt, s.modeSystemPrompt(source.PipelineMode)),
			TaskPrompt:    firstNonEmpty(source.TaskPrompt, s.modeTaskPrompt(source.PipelineMode)),
			MaxInputChars: source.MaxInputChars,
		})
		if err != nil {
			log.Printf("process %s/%s: %v", source.ID, item.GUID, err)
			continue
		}

		err = s.store.UpsertProcessedItem(ctx, model.ProcessedItem{
			SourceID:      source.ID,
			GUID:          item.GUID,
			OriginalTitle: item.Title,
			OriginalLink:  item.Link,
			PublishedAt:   item.PublishedAt,
			OutputTitle:   fallbackTitle(resp.Title, item.Title),
			OutputSummary: resp.Summary,
			OutputContent: resp.Content,
			Model:         resp.Model,
			ProcessedAt:   time.Now().UTC(),
		})
		if err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	return s.store.UpdateFeedState(ctx, source.ID, &now, "")
}

func (s *Service) ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error) {
	return s.store.ListProcessedItems(ctx, sourceID, limit)
}

func fallbackTitle(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(secondary)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) modeSystemPrompt(mode string) string {
	if cfg, ok := s.modes[mode]; ok {
		return cfg.SystemPrompt
	}
	return ""
}

func (s *Service) modeTaskPrompt(mode string) string {
	if cfg, ok := s.modes[mode]; ok {
		return cfg.TaskPrompt
	}
	return ""
}
