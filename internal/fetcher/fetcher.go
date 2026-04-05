package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/batkiz/rss-gateway/internal/content"
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
		contentHTML := mergeFeedContent(item.Content, item.Description)
		if contentHTML == "" && strings.TrimSpace(item.Link) == "" {
			continue
		}

		guid := strings.TrimSpace(item.GUID)
		if guid == "" {
			guid = strings.TrimSpace(item.Link)
		}
		if guid == "" {
			guid = hashString(source.ID + ":" + item.Title + ":" + contentHTML)
		}

		published := time.Now().UTC()
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.UTC()
		} else if item.UpdatedParsed != nil {
			published = item.UpdatedParsed.UTC()
		}

		contentText := content.ExtractText(contentHTML)
		if source.ExtractFull && strings.TrimSpace(item.Link) != "" {
			articleHTML, articleText, err := f.fetchLinkedArticle(ctx, item.Link)
			if err == nil && shouldPreferLinkedArticle(contentText, articleText) {
				contentHTML = articleHTML
				contentText = articleText
			}
		}
		if contentText == "" {
			continue
		}

		items = append(items, model.RawItem{
			SourceID:    source.ID,
			GUID:        guid,
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Description: strings.TrimSpace(item.Description),
			Content:     contentText,
			ContentHTML: contentHTML,
			ContentText: contentText,
			Author:      authorName(item),
			PublishedAt: published,
			Hash:        hashString(strings.TrimSpace(item.Title) + "\n" + contentText),
			FetchedAt:   time.Now().UTC(),
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

func (f *Fetcher) fetchLinkedArticle(ctx context.Context, link string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "rss-gateway/0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("fetch linked article status %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return "", "", fmt.Errorf("unsupported linked article content-type %q", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", "", err
	}

	return content.ExtractReadableHTML(strings.NewReader(string(body)))
}

func mergeFeedContent(contentHTML, description string) string {
	contentHTML = strings.TrimSpace(contentHTML)
	description = strings.TrimSpace(description)
	switch {
	case contentHTML == "":
		return description
	case description == "":
		return contentHTML
	case content.ExtractText(contentHTML) == content.ExtractText(description):
		return contentHTML
	default:
		return contentHTML + "\n\n" + description
	}
}

func shouldPreferLinkedArticle(feedText, articleText string) bool {
	feedText = strings.TrimSpace(feedText)
	articleText = strings.TrimSpace(articleText)
	if articleText == "" {
		return false
	}
	if feedText == "" {
		return true
	}
	if len(feedText) < 500 && len(articleText) > len(feedText)+200 {
		return true
	}
	return len(articleText) > int(float64(len(feedText))*1.35)
}
