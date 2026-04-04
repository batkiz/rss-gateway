package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/batkiz/rss-gateway/internal/model"
)

func TestSQLiteStoreRawAndProcessedItems(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	raw := model.RawItem{
		SourceID:    "demo",
		GUID:        "1",
		Title:       "t",
		Link:        "https://example.com",
		Description: "d",
		ContentHTML: "<p>x</p>",
		ContentText: "x",
		Hash:        "hash1",
		PublishedAt: time.Now().UTC(),
		FetchedAt:   time.Now().UTC(),
	}
	if err := store.UpsertRawItem(ctx, raw); err != nil {
		t.Fatalf("UpsertRawItem error: %v", err)
	}
	if err := store.UpsertProcessedItem(ctx, model.ProcessedItem{
		SourceID:      "demo",
		GUID:          "1",
		OriginalTitle: "t",
		OriginalLink:  "https://example.com",
		PublishedAt:   raw.PublishedAt,
		OutputTitle:   "ot",
		OutputSummary: "os",
		OutputContent: "oc",
		OutputJSON:    `{"title":"ot"}`,
		Model:         "m",
		InputHash:     "hash1",
		ProcessedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertProcessedItem error: %v", err)
	}

	items, err := store.ListRawItems(ctx, "demo", 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListRawItems unexpected result: len=%d err=%v", len(items), err)
	}

	hash, exists, err := store.GetProcessedInputHash(ctx, "demo", "1")
	if err != nil || !exists || hash != "hash1" {
		t.Fatalf("GetProcessedInputHash unexpected result: hash=%q exists=%v err=%v", hash, exists, err)
	}
}
