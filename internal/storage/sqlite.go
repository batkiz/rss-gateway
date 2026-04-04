package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/batkiz/rss-gateway/internal/model"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS processed_items (
			source_id TEXT NOT NULL,
			guid TEXT NOT NULL,
			original_title TEXT NOT NULL,
			original_link TEXT NOT NULL,
			published_at TIMESTAMP NOT NULL,
			output_title TEXT NOT NULL,
			output_summary TEXT NOT NULL,
			output_content TEXT NOT NULL,
			model TEXT NOT NULL,
			processed_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id, guid)
		);`,
		`CREATE TABLE IF NOT EXISTS feed_state (
			source_id TEXT PRIMARY KEY,
			last_success_at TIMESTAMP,
			last_error TEXT NOT NULL DEFAULT ''
		);`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertProcessedItem(ctx context.Context, item model.ProcessedItem) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO processed_items (
			source_id, guid, original_title, original_link, published_at,
			output_title, output_summary, output_content, model, processed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, guid) DO UPDATE SET
			original_title = excluded.original_title,
			original_link = excluded.original_link,
			published_at = excluded.published_at,
			output_title = excluded.output_title,
			output_summary = excluded.output_summary,
			output_content = excluded.output_content,
			model = excluded.model,
			processed_at = excluded.processed_at
	`, item.SourceID, item.GUID, item.OriginalTitle, item.OriginalLink, item.PublishedAt.UTC(),
		item.OutputTitle, item.OutputSummary, item.OutputContent, item.Model, item.ProcessedAt.UTC())
	return err
}

func (s *SQLiteStore) HasProcessedItem(ctx context.Context, sourceID, guid string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM processed_items WHERE source_id = ? AND guid = ? LIMIT 1
	`, sourceID, guid).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteStore) ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, guid, original_title, original_link, published_at,
		       output_title, output_summary, output_content, model, processed_at
		FROM processed_items
		WHERE source_id = ?
		ORDER BY published_at DESC
		LIMIT ?
	`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ProcessedItem
	for rows.Next() {
		var item model.ProcessedItem
		if err := rows.Scan(
			&item.SourceID,
			&item.GUID,
			&item.OriginalTitle,
			&item.OriginalLink,
			&item.PublishedAt,
			&item.OutputTitle,
			&item.OutputSummary,
			&item.OutputContent,
			&item.Model,
			&item.ProcessedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpdateFeedState(ctx context.Context, sourceID string, successAt *time.Time, lastError string) error {
	var ts any
	if successAt != nil {
		ts = successAt.UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO feed_state (source_id, last_success_at, last_error)
		VALUES (?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET
			last_success_at = excluded.last_success_at,
			last_error = excluded.last_error
	`, sourceID, ts, lastError)
	return err
}
