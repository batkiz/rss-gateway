package pipeline

import (
	"context"
	"fmt"
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
	UpsertRawItem(ctx context.Context, item model.RawItem) error
	ListRawItems(ctx context.Context, sourceID string, limit int) ([]model.RawItem, error)
	GetProcessedInputHash(ctx context.Context, sourceID, guid string) (string, bool, error)
	UpsertProcessedItem(ctx context.Context, item model.ProcessedItem) error
	ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error)
	UpdateFeedState(ctx context.Context, state model.FeedState) error
	GetFeedState(ctx context.Context, sourceID string) (model.FeedState, error)
	ListFeedStates(ctx context.Context) ([]model.FeedState, error)
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
			ExtractFull:     source.Pipeline.ExtractFullContent,
			Temperature:     source.Pipeline.Temperature,
			MaxOutputTokens: source.Pipeline.MaxOutputTokens,
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
		return fmt.Errorf("source %s not found", sourceID)
	}

	items, err := s.fetcher.Fetch(ctx, source)
	if err != nil {
		state, stateErr := s.store.GetFeedState(ctx, source.ID)
		if stateErr != nil {
			return err
		}
		state.LastError = err.Error()
		state.SourceID = source.ID
		_ = s.store.UpdateFeedState(ctx, state)
		return err
	}

	processedCount := 0
	for _, item := range items {
		if err := s.store.UpsertRawItem(ctx, item); err != nil {
			return err
		}

		processed, err := s.processRawItem(ctx, source, item, false)
		if err != nil {
			return err
		}
		if processed {
			processedCount++
		}
	}

	return s.store.UpdateFeedState(ctx, model.FeedState{
		SourceID:           source.ID,
		LastSuccessAt:      time.Now().UTC(),
		LastFetchedCount:   len(items),
		LastProcessedCount: processedCount,
	})
}

func (s *Service) ReprocessSource(ctx context.Context, sourceID string, limit int) error {
	source, ok := s.sources[sourceID]
	if !ok {
		return fmt.Errorf("source %s not found", sourceID)
	}
	if limit <= 0 {
		limit = source.MaxItems
	}

	items, err := s.store.ListRawItems(ctx, sourceID, limit)
	if err != nil {
		return err
	}

	reprocessedCount := 0
	for _, item := range items {
		processed, err := s.processRawItem(ctx, source, item, true)
		if err != nil {
			return err
		}
		if processed {
			reprocessedCount++
		}
	}

	state, err := s.store.GetFeedState(ctx, sourceID)
	if err != nil {
		return err
	}
	state.SourceID = sourceID
	state.LastSuccessAt = time.Now().UTC()
	state.LastError = ""
	state.LastReprocessedCount = reprocessedCount
	return s.store.UpdateFeedState(ctx, state)
}

func (s *Service) ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error) {
	return s.store.ListProcessedItems(ctx, sourceID, limit)
}

func (s *Service) ListRawItems(ctx context.Context, sourceID string, limit int) ([]model.RawItem, error) {
	return s.store.ListRawItems(ctx, sourceID, limit)
}

func (s *Service) FeedStatus(ctx context.Context) ([]model.FeedState, error) {
	states, err := s.store.ListFeedStates(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]model.FeedState, len(states))
	for _, state := range states {
		byID[state.SourceID] = state
	}

	out := make([]model.FeedState, 0, len(s.sources))
	for id := range s.sources {
		state, ok := byID[id]
		if !ok {
			var err error
			state, err = s.store.GetFeedState(ctx, id)
			if err != nil {
				return nil, err
			}
		}
		state.SourceID = id
		out = append(out, state)
	}
	return out, nil
}

func (s *Service) processRawItem(ctx context.Context, source model.Source, item model.RawItem, force bool) (bool, error) {
	inputHash, exists, err := s.store.GetProcessedInputHash(ctx, source.ID, item.GUID)
	if err != nil {
		return false, err
	}
	if exists && inputHash == item.Hash && !force {
		return false, nil
	}

	modeCfg := s.modeConfig(source.PipelineMode)
	resp, err := s.processor.Process(ctx, model.ProcessRequest{
		Mode:            source.PipelineMode,
		Title:           fallbackTitle(item.Title, item.Link),
		Link:            item.Link,
		Content:         item.Content,
		SystemPrompt:    firstNonEmpty(source.SystemPrompt, modeCfg.SystemPrompt),
		TaskPrompt:      firstNonEmpty(source.TaskPrompt, modeCfg.TaskPrompt),
		MaxInputChars:   source.MaxInputChars,
		Temperature:     firstNonNilFloat(source.Temperature, modeCfg.Temperature),
		MaxOutputTokens: firstNonZero(source.MaxOutputTokens, modeCfg.MaxOutputTokens),
		OutputSchema:    toModelSchema(modeCfg.OutputSchema),
	})
	if err != nil {
		log.Printf("process %s/%s: %v", source.ID, item.GUID, err)
		return false, nil
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
		OutputJSON:    resp.OutputJSON,
		Model:         resp.Model,
		InputHash:     item.Hash,
		ProcessedAt:   time.Now().UTC(),
	})
	if err != nil {
		return false, err
	}
	return true, nil
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

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonNilFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func (s *Service) modeConfig(mode string) config.ModeConfig {
	if cfg, ok := s.modes[mode]; ok {
		return cfg
	}
	return config.ModeConfig{}
}

func toModelSchema(cfg config.OutputSchemaConfig) model.OutputSchema {
	if cfg.Name == "" {
		cfg.Name = "rss_output"
	}
	if cfg.TitleField == "" {
		cfg.TitleField = "title"
	}
	if cfg.SummaryField == "" {
		cfg.SummaryField = "summary"
	}
	if cfg.ContentField == "" {
		cfg.ContentField = "content"
	}
	fields := []model.OutputField{
		{Name: cfg.TitleField, Type: "string", Description: "Reader-facing title", Required: true},
		{Name: cfg.SummaryField, Type: "string", Description: "Short summary", Required: true},
		{Name: cfg.ContentField, Type: "string", Description: "RSS content body", Required: true},
	}
	for _, extra := range cfg.ExtraFields {
		required := true
		if extra.Required != nil {
			required = *extra.Required
		}
		fields = append(fields, model.OutputField{
			Name:        extra.Name,
			Type:        extra.Type,
			Description: extra.Description,
			Required:    required,
		})
	}
	return model.OutputSchema{
		Name:         cfg.Name,
		TitleField:   cfg.TitleField,
		SummaryField: cfg.SummaryField,
		ContentField: cfg.ContentField,
		Fields:       fields,
	}
}
