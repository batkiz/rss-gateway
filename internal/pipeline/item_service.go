package pipeline

import (
	"context"
	"database/sql"
	"errors"

	"github.com/batkiz/rss-gateway/internal/model"
)

type ItemService struct {
	store  Store
	runner runtimeProcessor
}

func NewItemService(store Store) *ItemService {
	return &ItemService{
		store:  store,
		runner: newRuntimeProcessor(store),
	}
}

func (s *ItemService) GetRawItem(ctx context.Context, sourceID, guid string) (model.RawItem, error) {
	return s.store.GetRawItem(ctx, sourceID, guid)
}

func (s *ItemService) GetProcessedItem(ctx context.Context, sourceID, guid string) (*model.ProcessedItem, error) {
	item, err := s.store.GetProcessedItem(ctx, sourceID, guid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *ItemService) PreviewItem(ctx context.Context, sourceID, guid string, overrides model.ProcessOverrides) (model.ItemProcessPreview, error) {
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	return s.previewItemWithOverrides(ctx, source, guid, overrides, false)
}

func (s *ItemService) ReprocessItem(ctx context.Context, sourceID, guid string, overrides model.ProcessOverrides) (model.ItemProcessPreview, error) {
	source, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	return s.previewItemWithOverrides(ctx, source, guid, overrides, true)
}

func (s *ItemService) previewItemWithOverrides(ctx context.Context, source model.Source, guid string, overrides model.ProcessOverrides, save bool) (model.ItemProcessPreview, error) {
	rawItem, err := s.store.GetRawItem(ctx, source.ID, guid)
	if err != nil {
		return model.ItemProcessPreview{}, err
	}
	request, response, err := s.runner.processRawItemWithOverrides(ctx, source, rawItem, overrides, true, save)
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
