package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/batkiz/rss-gateway/internal/model"
	"github.com/mmcdole/gofeed"
)

type Fetcher struct {
	parser *gofeed.Parser
	client *http.Client
}

func New(timeout time.Duration) *Fetcher {
	client := &http.Client{Timeout: timeout}
	parser := gofeed.NewParser()
	parser.Client = client
	return &Fetcher{
		parser: parser,
		client: client,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, source model.Source) ([]model.RawItem, error) {
	feed, err := f.parser.ParseURLWithContext(source.URL, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", source.URL, err)
	}

	items := make([]model.RawItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			content = strings.TrimSpace(item.Description)
		}
		if content == "" {
			continue
		}

		guid := strings.TrimSpace(item.GUID)
		if guid == "" {
			guid = strings.TrimSpace(item.Link)
		}
		if guid == "" {
			guid = hashString(source.ID + ":" + item.Title + ":" + content)
		}

		published := time.Now().UTC()
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.UTC()
		} else if item.UpdatedParsed != nil {
			published = item.UpdatedParsed.UTC()
		}

		items = append(items, model.RawItem{
			SourceID:    source.ID,
			GUID:        guid,
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Description: strings.TrimSpace(item.Description),
			Content:     content,
			Author:      authorName(item),
			PublishedAt: published,
			Hash:        hashString(strings.TrimSpace(item.Title) + "\n" + content),
		})
	}

	if len(items) > source.MaxItems {
		items = items[:source.MaxItems]
	}
	return items, nil
}

func authorName(item *gofeed.Item) string {
	if item.Author != nil {
		return strings.TrimSpace(item.Author.Name)
	}
	return ""
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
