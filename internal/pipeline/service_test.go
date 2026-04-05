package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/batkiz/rss-gateway/internal/fetcher"
	"github.com/batkiz/rss-gateway/internal/model"
	"github.com/batkiz/rss-gateway/internal/storage"
)

func TestRefreshSourceContinuesWhenLLMFails(t *testing.T) {
	var baseURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>demo</title>
    <item>
      <guid>item-1</guid>
      <title>Hello</title>
      <link>%s/article</link>
      <description><![CDATA[<p>Body text from feed.</p>]]></description>
      <pubDate>Sat, 05 Apr 2026 08:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`, baseURL)
		case "/chat/completions":
			http.Error(w, `{"error":"boom"}`, http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.UpsertLLMSettings(ctx, model.LLMSettings{
		Provider: "openai",
		Model:    "gpt-test",
		APIKey:   "test-key",
		BaseURL:  baseURL,
		Timeout:  "5s",
	}); err != nil {
		t.Fatalf("UpsertLLMSettings error: %v", err)
	}
	if err := store.UpsertMode(ctx, model.Mode{
		Name:         "summary",
		SystemPrompt: "Summarize it",
		TaskPrompt:   "Return title summary and content",
		OutputSchema: model.OutputSchema{
			Name:         "summary",
			TitleField:   "title",
			SummaryField: "summary",
			ContentField: "content",
			Fields: []model.OutputField{
				{Name: "title", Type: "string", Required: true},
				{Name: "summary", Type: "string", Required: true},
				{Name: "content", Type: "string", Required: true},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMode error: %v", err)
	}
	if err := store.UpsertSource(ctx, model.Source{
		ID:              "demo",
		Name:            "demo",
		URL:             baseURL + "/feed",
		RefreshInterval: 30 * time.Minute,
		Enabled:         true,
		MaxItems:        10,
		PipelineMode:    "summary",
		MaxInputChars:   8000,
	}); err != nil {
		t.Fatalf("UpsertSource error: %v", err)
	}

	service := NewService(fetcher.New(5*time.Second), store)
	if err := service.RefreshSource(ctx, "demo"); err != nil {
		t.Fatalf("RefreshSource error: %v", err)
	}

	rawItems, err := service.ListRawItems(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListRawItems error: %v", err)
	}
	if len(rawItems) != 1 {
		t.Fatalf("expected 1 raw item, got %d", len(rawItems))
	}
	if !strings.Contains(rawItems[0].ContentText, "Body text from feed.") {
		t.Fatalf("unexpected raw content: %q", rawItems[0].ContentText)
	}

	processedItems, err := service.ListProcessedItems(ctx, "demo", 10)
	if err != nil {
		t.Fatalf("ListProcessedItems error: %v", err)
	}
	if len(processedItems) != 0 {
		t.Fatalf("expected 0 processed items, got %d", len(processedItems))
	}

	state, err := service.FeedState(ctx, "demo")
	if err != nil {
		t.Fatalf("FeedState error: %v", err)
	}
	if state.LastFetchedCount != 1 {
		t.Fatalf("expected fetched_count=1, got %d", state.LastFetchedCount)
	}
	if state.LastProcessedCount != 0 {
		t.Fatalf("expected processed_count=0, got %d", state.LastProcessedCount)
	}
	if state.LastSuccessAt.IsZero() {
		t.Fatal("expected last_success_at to be set")
	}
}
