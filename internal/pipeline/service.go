package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/fetcher"
	"github.com/batkiz/rss-gateway/internal/llm"
	"github.com/batkiz/rss-gateway/internal/model"
)

type Store interface {
	GetLLMSettings(ctx context.Context) (model.LLMSettings, error)
	UpsertLLMSettings(ctx context.Context, settings model.LLMSettings) error
	ListModes(ctx context.Context) ([]model.Mode, error)
	GetMode(ctx context.Context, name string) (model.Mode, error)
	UpsertMode(ctx context.Context, mode model.Mode) error
	ListSources(ctx context.Context) ([]model.Source, error)
	GetSource(ctx context.Context, id string) (model.Source, error)
	UpsertSource(ctx context.Context, source model.Source) error
	UpsertRawItem(ctx context.Context, item model.RawItem) error
	ListRawItems(ctx context.Context, sourceID string, limit int) ([]model.RawItem, error)
	GetRawItem(ctx context.Context, sourceID, guid string) (model.RawItem, error)
	GetProcessedInputHash(ctx context.Context, sourceID, guid string) (string, bool, error)
	UpsertProcessedItem(ctx context.Context, item model.ProcessedItem) error
	ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error)
	GetProcessedItem(ctx context.Context, sourceID, guid string) (model.ProcessedItem, error)
	UpdateFeedState(ctx context.Context, state model.FeedState) error
	GetFeedState(ctx context.Context, sourceID string) (model.FeedState, error)
	ListFeedStates(ctx context.Context) ([]model.FeedState, error)
}

type Service struct {
	fetcher *fetcher.Fetcher
	store   Store
}

func NewService(fetcher *fetcher.Fetcher, store Store) *Service {
	return &Service{
		fetcher: fetcher,
		store:   store,
	}
}

func (s *Service) ListSources(ctx context.Context) ([]model.Source, error) {
	sources, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].ID < sources[j].ID
	})
	return sources, nil
}

func (s *Service) SourcesMap(ctx context.Context) (map[string]model.Source, error) {
	sources, err := s.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.Source, len(sources))
	for _, source := range sources {
		out[source.ID] = source
	}
	return out, nil
}

func (s *Service) GetSource(ctx context.Context, sourceID string) (model.Source, error) {
	return s.store.GetSource(ctx, sourceID)
}

func (s *Service) ListModes(ctx context.Context) ([]model.Mode, error) {
	return s.store.ListModes(ctx)
}

func (s *Service) GetMode(ctx context.Context, modeName string) (model.Mode, error) {
	return s.store.GetMode(ctx, modeName)
}

func (s *Service) GetLLMSettings(ctx context.Context) (model.LLMSettings, error) {
	return s.store.GetLLMSettings(ctx)
}

func (s *Service) SaveLLMSettings(ctx context.Context, settings model.LLMSettings) error {
	settings.Provider = strings.TrimSpace(settings.Provider)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.Timeout = strings.TrimSpace(settings.Timeout)

	if settings.Provider == "" {
		return errors.New("llm provider is required")
	}
	if settings.Model == "" {
		return errors.New("llm model is required")
	}
	if settings.APIKey == "" {
		return errors.New("llm api key is required")
	}
	if settings.BaseURL == "" {
		settings.BaseURL = "https://api.openai.com/v1"
	}
	if settings.Timeout == "" {
		settings.Timeout = "60s"
	}
	if _, err := time.ParseDuration(settings.Timeout); err != nil {
		return fmt.Errorf("invalid llm timeout: %w", err)
	}
	return s.store.UpsertLLMSettings(ctx, settings)
}

func (s *Service) SaveMode(ctx context.Context, mode model.Mode) error {
	mode.Name = strings.TrimSpace(mode.Name)
	mode.SystemPrompt = strings.TrimSpace(mode.SystemPrompt)
	mode.TaskPrompt = strings.TrimSpace(mode.TaskPrompt)
	mode.OutputSchema.Name = strings.TrimSpace(mode.OutputSchema.Name)
	mode.OutputSchema.TitleField = strings.TrimSpace(mode.OutputSchema.TitleField)
	mode.OutputSchema.SummaryField = strings.TrimSpace(mode.OutputSchema.SummaryField)
	mode.OutputSchema.ContentField = strings.TrimSpace(mode.OutputSchema.ContentField)

	if mode.Name == "" {
		return errors.New("mode name is required")
	}
	if mode.OutputSchema.Name == "" {
		mode.OutputSchema.Name = mode.Name
	}
	if mode.OutputSchema.TitleField == "" {
		mode.OutputSchema.TitleField = "title"
	}
	if mode.OutputSchema.SummaryField == "" {
		mode.OutputSchema.SummaryField = "summary"
	}
	if mode.OutputSchema.ContentField == "" {
		mode.OutputSchema.ContentField = "content"
	}
	if len(mode.OutputSchema.Fields) == 0 {
		mode.OutputSchema.Fields = defaultSchemaFields(mode.OutputSchema)
	}
	if err := validateSchemaFields(mode.OutputSchema); err != nil {
		return err
	}
	return s.store.UpsertMode(ctx, mode)
}

func (s *Service) SaveSource(ctx context.Context, source model.Source) error {
	source.ID = strings.TrimSpace(source.ID)
	source.Name = strings.TrimSpace(source.Name)
	source.URL = strings.TrimSpace(source.URL)
	source.PipelineMode = strings.TrimSpace(source.PipelineMode)
	source.SystemPrompt = strings.TrimSpace(source.SystemPrompt)
	source.TaskPrompt = strings.TrimSpace(source.TaskPrompt)

	if source.ID == "" {
		return errors.New("source id is required")
	}
	if source.URL == "" {
		return fmt.Errorf("source %s url is required", source.ID)
	}
	if source.PipelineMode == "" {
		return fmt.Errorf("source %s pipeline mode is required", source.ID)
	}
	if source.RefreshInterval <= 0 {
		source.RefreshInterval = 30 * time.Minute
	}
	if source.MaxItems <= 0 {
		source.MaxItems = 20
	}
	if source.MaxInputChars <= 0 {
		source.MaxInputChars = 8000
	}
	if _, err := s.store.GetMode(ctx, source.PipelineMode); err != nil {
		return fmt.Errorf("source %s references undefined mode %q", source.ID, source.PipelineMode)
	}
	return s.store.UpsertSource(ctx, source)
}

func (s *Service) RefreshAll(ctx context.Context) error {
	sources, err := s.ListSources(ctx)
	if err != nil {
		return err
	}
	group, groupCtx := errgroup.WithContext(ctx)
	for _, source := range sources {
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
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("source %s not found: %w", sourceID, err)
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
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("source %s not found: %w", sourceID, err)
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

func (s *Service) GetRawItem(ctx context.Context, sourceID, guid string) (model.RawItem, error) {
	return s.store.GetRawItem(ctx, sourceID, guid)
}

func (s *Service) GetProcessedItem(ctx context.Context, sourceID, guid string) (*model.ProcessedItem, error) {
	item, err := s.store.GetProcessedItem(ctx, sourceID, guid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Service) FeedState(ctx context.Context, sourceID string) (model.FeedState, error) {
	return s.store.GetFeedState(ctx, sourceID)
}

func (s *Service) FeedStatus(ctx context.Context) ([]model.FeedState, error) {
	sources, err := s.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	states, err := s.store.ListFeedStates(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]model.FeedState, len(states))
	for _, state := range states {
		byID[state.SourceID] = state
	}

	out := make([]model.FeedState, 0, len(sources))
	for _, source := range sources {
		id := source.ID
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
	_, _, err = s.processRawItemWithOverrides(ctx, source, item, model.ProcessOverrides{}, force, true)
	if err != nil {
		log.Printf("process item source=%s guid=%s: %v", source.ID, item.GUID, err)
		return false, nil
	}
	return true, nil
}

func (s *Service) PreviewItem(ctx context.Context, sourceID, guid string, overrides model.ProcessOverrides) (model.ItemProcessPreview, error) {
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	return s.previewItemWithOverrides(ctx, source, guid, overrides, false)
}

func (s *Service) ReprocessItem(ctx context.Context, sourceID, guid string, overrides model.ProcessOverrides) (model.ItemProcessPreview, error) {
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	return s.previewItemWithOverrides(ctx, source, guid, overrides, true)
}

func (s *Service) previewItemWithOverrides(ctx context.Context, source model.Source, guid string, overrides model.ProcessOverrides, save bool) (model.ItemProcessPreview, error) {
	rawItem, err := s.store.GetRawItem(ctx, source.ID, guid)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	request, response, err := s.processRawItemWithOverrides(ctx, source, rawItem, overrides, true, save)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	var processed *model.ProcessedItem
	processedItem, err := s.store.GetProcessedItem(ctx, source.ID, guid)
	if err == nil {
		processed = &processedItem
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.ItemProcessPreview{}, err
	}
	return model.ItemProcessPreview{
		Source:    source,
		RawItem:   rawItem,
		Request:   request,
		Response:  response,
		Processed: processed,
	}, nil
}

func (s *Service) processRawItemWithOverrides(ctx context.Context, source model.Source, item model.RawItem, overrides model.ProcessOverrides, force, save bool) (model.ProcessRequest, model.ProcessResponse, error) {
	inputHash, exists, err := s.store.GetProcessedInputHash(ctx, source.ID, item.GUID)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	if exists && inputHash == item.Hash && !force {
		existing, err := s.store.GetProcessedItem(ctx, source.ID, item.GUID)
		if err != nil {
			return model.ProcessRequest{}, model.ProcessResponse{}, err
		}
		request, err := s.buildProcessRequest(ctx, source, item, overrides)
		if err != nil {
			return model.ProcessRequest{}, model.ProcessResponse{}, err
		}
		return request, model.ProcessResponse{
			Title:      existing.OutputTitle,
			Summary:    existing.OutputSummary,
			Content:    existing.OutputContent,
			Model:      existing.Model,
			OutputJSON: existing.OutputJSON,
		}, nil
	}

	request, err := s.buildProcessRequest(ctx, source, item, overrides)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	processor, err := s.processorFor(ctx)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	resp, err := processor.Process(ctx, request)
	if err != nil {
		log.Printf("process %s/%s: %v", source.ID, item.GUID, err)
		return request, model.ProcessResponse{}, err
	}

	if save {
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
			return request, model.ProcessResponse{}, err
		}
	}
	return request, resp, nil
}

func (s *Service) buildProcessRequest(ctx context.Context, source model.Source, item model.RawItem, overrides model.ProcessOverrides) (model.ProcessRequest, error) {
	modeName := firstNonEmpty(overrides.Mode, source.PipelineMode)
	modeCfg, err := s.modeConfig(ctx, modeName)
	if err != nil {
		return model.ProcessRequest{}, err
	}
	request := model.ProcessRequest{
		Mode:            modeName,
		Title:           fallbackTitle(item.Title, item.Link),
		Link:            item.Link,
		Content:         item.Content,
		SystemPrompt:    firstNonEmpty(overrides.SystemPrompt, source.SystemPrompt, modeCfg.SystemPrompt),
		TaskPrompt:      firstNonEmpty(overrides.TaskPrompt, source.TaskPrompt, modeCfg.TaskPrompt),
		MaxInputChars:   firstNonZero(overrides.MaxInputChars, source.MaxInputChars),
		Temperature:     firstNonNilFloat(overrides.Temperature, source.Temperature, modeCfg.Temperature),
		MaxOutputTokens: firstNonZero(overrides.MaxOutputTokens, source.MaxOutputTokens, modeCfg.MaxOutputTokens),
		OutputSchema:    modeCfg.OutputSchema,
	}
	if request.MaxInputChars <= 0 {
		request.MaxInputChars = 8000
	}
	return request, nil
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

func (s *Service) modeConfig(ctx context.Context, mode string) (model.Mode, error) {
	if strings.TrimSpace(mode) == "" {
		return model.Mode{}, nil
	}
	cfg, err := s.store.GetMode(ctx, mode)
	if err != nil {
		return model.Mode{}, err
	}
	return cfg, nil
}

func (s *Service) processorFor(ctx context.Context) (llm.Processor, error) {
	settings, err := s.store.GetLLMSettings(ctx)
	if err != nil {
		return nil, err
	}
	switch settings.Provider {
	case "", "openai":
		return llm.NewOpenAIProcessor(config.LLMConfig{
			Provider: settings.Provider,
			Model:    settings.Model,
			APIKey:   settings.APIKey,
			BaseURL:  settings.BaseURL,
			Timeout:  settings.Timeout,
		})
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", settings.Provider)
	}
}

func validateSchemaFields(schema model.OutputSchema) error {
	if schema.TitleField == "" || schema.SummaryField == "" || schema.ContentField == "" {
		return errors.New("output schema title, summary, and content fields are required")
	}
	seen := map[string]struct{}{}
	for _, field := range schema.Fields {
		field.Name = strings.TrimSpace(field.Name)
		if field.Name == "" {
			return errors.New("output schema field name is required")
		}
		if _, ok := seen[field.Name]; ok {
			return fmt.Errorf("duplicate output schema field %q", field.Name)
		}
		seen[field.Name] = struct{}{}
	}
	return nil
}

func defaultSchemaFields(schema model.OutputSchema) []model.OutputField {
	return []model.OutputField{
		{Name: schema.TitleField, Type: "string", Description: "Reader-facing title", Required: true},
		{Name: schema.SummaryField, Type: "string", Description: "Short summary", Required: true},
		{Name: schema.ContentField, Type: "string", Description: "RSS content body", Required: true},
	}
}
