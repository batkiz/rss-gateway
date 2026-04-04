package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
		`CREATE TABLE IF NOT EXISTS raw_items (
			source_id TEXT NOT NULL,
			guid TEXT NOT NULL,
			title TEXT NOT NULL,
			link TEXT NOT NULL,
			description TEXT NOT NULL,
			content_html TEXT NOT NULL,
			content_text TEXT NOT NULL,
			author TEXT NOT NULL,
			published_at TIMESTAMP NOT NULL,
			content_hash TEXT NOT NULL,
			fetched_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id, guid)
		);`,
		`CREATE TABLE IF NOT EXISTS processed_items (
			source_id TEXT NOT NULL,
			guid TEXT NOT NULL,
			original_title TEXT NOT NULL,
			original_link TEXT NOT NULL,
			published_at TIMESTAMP NOT NULL,
			output_title TEXT NOT NULL,
			output_summary TEXT NOT NULL,
			output_content TEXT NOT NULL,
			output_json TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL,
			input_hash TEXT NOT NULL DEFAULT '',
			processed_at TIMESTAMP NOT NULL,
			PRIMARY KEY (source_id, guid)
		);`,
		`CREATE TABLE IF NOT EXISTS feed_state (
			source_id TEXT PRIMARY KEY,
			last_success_at TIMESTAMP,
			last_error TEXT NOT NULL DEFAULT '',
			last_fetched_count INTEGER NOT NULL DEFAULT 0,
			last_processed_count INTEGER NOT NULL DEFAULT 0,
			last_reprocessed_count INTEGER NOT NULL DEFAULT 0
		);`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	alterStatements := []string{
		`ALTER TABLE processed_items ADD COLUMN output_json TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE processed_items ADD COLUMN input_hash TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE feed_state ADD COLUMN last_fetched_count INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE feed_state ADD COLUMN last_processed_count INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE feed_state ADD COLUMN last_reprocessed_count INTEGER NOT NULL DEFAULT 0;`,
	}
	for _, stmt := range alterStatements {
		_, _ = s.db.Exec(stmt)
	}
	return nil
}

func (s *SQLiteStore) UpsertRawItem(ctx context.Context, item model.RawItem) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO raw_items (
			source_id, guid, title, link, description, content_html,
			content_text, author, published_at, content_hash, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, guid) DO UPDATE SET
			title = excluded.title,
			link = excluded.link,
			description = excluded.description,
			content_html = excluded.content_html,
			content_text = excluded.content_text,
			author = excluded.author,
			published_at = excluded.published_at,
			content_hash = excluded.content_hash,
			fetched_at = excluded.fetched_at
	`, item.SourceID, item.GUID, item.Title, item.Link, item.Description, item.ContentHTML,
		item.ContentText, item.Author, item.PublishedAt.UTC(), item.Hash, item.FetchedAt.UTC())
	return err
}

func (s *SQLiteStore) ListRawItems(ctx context.Context, sourceID string, limit int) ([]model.RawItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, guid, title, link, description, content_html,
		       content_text, author, published_at, content_hash, fetched_at
		FROM raw_items
		WHERE source_id = ?
		ORDER BY published_at DESC, fetched_at DESC
		LIMIT ?
	`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.RawItem
	for rows.Next() {
		var item model.RawItem
		if err := rows.Scan(
			&item.SourceID,
			&item.GUID,
			&item.Title,
			&item.Link,
			&item.Description,
			&item.ContentHTML,
			&item.ContentText,
			&item.Author,
			&item.PublishedAt,
			&item.Hash,
			&item.FetchedAt,
		); err != nil {
			return nil, err
		}
		item.Content = item.ContentText
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetProcessedInputHash(ctx context.Context, sourceID, guid string) (string, bool, error) {
	var inputHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT input_hash FROM processed_items WHERE source_id = ? AND guid = ? LIMIT 1
	`, sourceID, guid).Scan(&inputHash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return inputHash, true, nil
}

func (s *SQLiteStore) UpsertProcessedItem(ctx context.Context, item model.ProcessedItem) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO processed_items (
			source_id, guid, original_title, original_link, published_at,
			output_title, output_summary, output_content, output_json, model, input_hash, processed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, guid) DO UPDATE SET
			original_title = excluded.original_title,
			original_link = excluded.original_link,
			published_at = excluded.published_at,
			output_title = excluded.output_title,
			output_summary = excluded.output_summary,
			output_content = excluded.output_content,
			output_json = excluded.output_json,
			model = excluded.model,
			input_hash = excluded.input_hash,
			processed_at = excluded.processed_at
	`, item.SourceID, item.GUID, item.OriginalTitle, item.OriginalLink, item.PublishedAt.UTC(),
		item.OutputTitle, item.OutputSummary, item.OutputContent, item.OutputJSON, item.Model, item.InputHash, item.ProcessedAt.UTC())
	return err
}

func (s *SQLiteStore) ListProcessedItems(ctx context.Context, sourceID string, limit int) ([]model.ProcessedItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, guid, original_title, original_link, published_at,
		       output_title, output_summary, output_content, output_json, model, input_hash, processed_at
		FROM processed_items
		WHERE source_id = ?
		ORDER BY published_at DESC, processed_at DESC
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
			&item.OutputJSON,
			&item.Model,
			&item.InputHash,
			&item.ProcessedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpdateFeedState(ctx context.Context, state model.FeedState) error {
	var ts any
	if !state.LastSuccessAt.IsZero() {
		ts = state.LastSuccessAt.UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO feed_state (
			source_id, last_success_at, last_error, last_fetched_count,
			last_processed_count, last_reprocessed_count
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET
			last_success_at = excluded.last_success_at,
			last_error = excluded.last_error,
			last_fetched_count = excluded.last_fetched_count,
			last_processed_count = excluded.last_processed_count,
			last_reprocessed_count = excluded.last_reprocessed_count
	`, state.SourceID, ts, state.LastError, state.LastFetchedCount, state.LastProcessedCount, state.LastReprocessedCount)
	return err
}

func (s *SQLiteStore) GetFeedState(ctx context.Context, sourceID string) (model.FeedState, error) {
	var state model.FeedState
	var lastSuccess sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT source_id, last_success_at, last_error, last_fetched_count,
		       last_processed_count, last_reprocessed_count
		FROM feed_state WHERE source_id = ?
	`, sourceID).Scan(
		&state.SourceID,
		&lastSuccess,
		&state.LastError,
		&state.LastFetchedCount,
		&state.LastProcessedCount,
		&state.LastReprocessedCount,
	)
	if err == sql.ErrNoRows {
		return model.FeedState{SourceID: sourceID}, nil
	}
	if err != nil {
		return model.FeedState{}, err
	}
	if lastSuccess.Valid {
		state.LastSuccessAt = lastSuccess.Time.UTC()
	}

	rawCount, processedCount, err := s.countItems(ctx, sourceID)
	if err != nil {
		return model.FeedState{}, err
	}
	state.RawItemCount = rawCount
	state.ProcessedItemCount = processedCount
	return state, nil
}

func (s *SQLiteStore) ListFeedStates(ctx context.Context) ([]model.FeedState, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_id, last_success_at, last_error, last_fetched_count,
		       last_processed_count, last_reprocessed_count
		FROM feed_state
		ORDER BY source_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []model.FeedState
	for rows.Next() {
		var state model.FeedState
		var lastSuccess sql.NullTime
		if err := rows.Scan(
			&state.SourceID,
			&lastSuccess,
			&state.LastError,
			&state.LastFetchedCount,
			&state.LastProcessedCount,
			&state.LastReprocessedCount,
		); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			state.LastSuccessAt = lastSuccess.Time.UTC()
		}
		rawCount, processedCount, err := s.countItems(ctx, state.SourceID)
		if err != nil {
			return nil, err
		}
		state.RawItemCount = rawCount
		state.ProcessedItemCount = processedCount
		states = append(states, state)
	}
	return states, rows.Err()
}

func (s *SQLiteStore) countItems(ctx context.Context, sourceID string) (int, int, error) {
	var rawCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM raw_items WHERE source_id = ?`, sourceID).Scan(&rawCount); err != nil {
		return 0, 0, err
	}

	var processedCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_items WHERE source_id = ?`, sourceID).Scan(&processedCount); err != nil {
		return 0, 0, err
	}
	return rawCount, processedCount, nil
}
