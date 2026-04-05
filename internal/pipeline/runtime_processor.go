package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/llm"
	"github.com/batkiz/rss-gateway/internal/model"
)

type runtimeProcessor struct {
	store Store
}

func newRuntimeProcessor(store Store) runtimeProcessor {
	return runtimeProcessor{store: store}
}

func (p runtimeProcessor) modeConfig(ctx context.Context, mode string) (model.Mode, error) {
	if strings.TrimSpace(mode) == "" {
		return model.Mode{}, nil
	}
	cfg, err := p.store.GetMode(ctx, mode)
	if err != nil {
		return model.Mode{}, err
	}
	return cfg, nil
}

func (p runtimeProcessor) processorFor(ctx context.Context) (llm.Processor, error) {
	settings, err := p.store.GetLLMSettings(ctx)
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

func (p runtimeProcessor) buildProcessRequest(ctx context.Context, source model.Source, item model.RawItem, overrides model.ProcessOverrides) (model.ProcessRequest, error) {
	modeName := firstNonEmpty(overrides.Mode, source.PipelineMode)
	modeCfg, err := p.modeConfig(ctx, modeName)
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

func (p runtimeProcessor) processRawItemWithOverrides(ctx context.Context, source model.Source, item model.RawItem, overrides model.ProcessOverrides, force, save bool) (model.ProcessRequest, model.ProcessResponse, error) {
	inputHash, exists, err := p.store.GetProcessedInputHash(ctx, source.ID, item.GUID)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	if exists && inputHash == item.Hash && !force {
		existing, err := p.store.GetProcessedItem(ctx, source.ID, item.GUID)
		if err != nil {
			return model.ProcessRequest{}, model.ProcessResponse{}, err
		}
		request, err := p.buildProcessRequest(ctx, source, item, overrides)
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

	request, err := p.buildProcessRequest(ctx, source, item, overrides)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	processor, err := p.processorFor(ctx)
	if err != nil {
		return model.ProcessRequest{}, model.ProcessResponse{}, err
	}
	resp, err := processor.Process(ctx, request)
	if err != nil {
		log.Printf("process %s/%s: %v", source.ID, item.GUID, err)
		return request, model.ProcessResponse{}, err
	}

	if save {
		err = p.store.UpsertProcessedItem(ctx, model.ProcessedItem{
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
