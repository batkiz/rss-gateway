package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/batkiz/rss-gateway/internal/config"
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
		`CREATE TABLE IF NOT EXISTS llm_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			api_key TEXT NOT NULL,
			base_url TEXT NOT NULL,
			timeout TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS modes (
			name TEXT PRIMARY KEY,
			system_prompt TEXT NOT NULL,
			task_prompt TEXT NOT NULL,
			temperature REAL,
			max_output_tokens INTEGER NOT NULL DEFAULT 0,
			schema_name TEXT NOT NULL,
			title_field TEXT NOT NULL,
			summary_field TEXT NOT NULL,
			content_field TEXT NOT NULL,
			extra_fields_json TEXT NOT NULL DEFAULT '[]',
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS source_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			refresh_interval TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			max_items INTEGER NOT NULL DEFAULT 20,
			pipeline_mode TEXT NOT NULL,
			system_prompt TEXT NOT NULL DEFAULT '',
			task_prompt TEXT NOT NULL DEFAULT '',
			max_input_chars INTEGER NOT NULL DEFAULT 8000,
			extract_full_content INTEGER NOT NULL DEFAULT 0,
			temperature REAL,
			max_output_tokens INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMP NOT NULL
		);`,
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

func (s *SQLiteStore) SeedRuntimeConfig(ctx context.Context, cfg config.Config) error {
	if err := s.seedLLMSettings(ctx, cfg.LLM); err != nil {
		return err
	}
	modes, err := s.ListModes(ctx)
	if err != nil {
		return err
	}
	if len(modes) == 0 {
		for name, mode := range cfg.Modes {
			if err := s.UpsertMode(ctx, toRuntimeMode(name, mode)); err != nil {
				return err
			}
		}
	}
	sources, err := s.ListSources(ctx)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		for _, source := range cfg.Sources {
			if err := s.UpsertSource(ctx, toRuntimeSource(source)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SQLiteStore) seedLLMSettings(ctx context.Context, cfg config.LLMConfig) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM llm_settings`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.UpsertLLMSettings(ctx, model.LLMSettings{
		Provider: cfg.Provider,
		Model:    cfg.Model,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
		Timeout:  cfg.Timeout,
	})
}

func (s *SQLiteStore) UpsertLLMSettings(ctx context.Context, settings model.LLMSettings) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_settings (id, provider, model, api_key, base_url, timeout, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			api_key = excluded.api_key,
			base_url = excluded.base_url,
			timeout = excluded.timeout,
			updated_at = excluded.updated_at
	`, settings.Provider, settings.Model, settings.APIKey, settings.BaseURL, settings.Timeout, time.Now().UTC())
	return err
}

func (s *SQLiteStore) GetLLMSettings(ctx context.Context) (model.LLMSettings, error) {
	var settings model.LLMSettings
	err := s.db.QueryRowContext(ctx, `
		SELECT provider, model, api_key, base_url, timeout
		FROM llm_settings WHERE id = 1
	`).Scan(&settings.Provider, &settings.Model, &settings.APIKey, &settings.BaseURL, &settings.Timeout)
	if err == sql.ErrNoRows {
		return model.LLMSettings{}, nil
	}
	return settings, err
}

func (s *SQLiteStore) UpsertMode(ctx context.Context, mode model.Mode) error {
	extraJSON, err := extraFieldsJSON(mode.OutputSchema)
	if err != nil {
		return err
	}
	var temperature any
	if mode.Temperature != nil {
		temperature = *mode.Temperature
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO modes (
			name, system_prompt, task_prompt, temperature, max_output_tokens,
			schema_name, title_field, summary_field, content_field, extra_fields_json, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			system_prompt = excluded.system_prompt,
			task_prompt = excluded.task_prompt,
			temperature = excluded.temperature,
			max_output_tokens = excluded.max_output_tokens,
			schema_name = excluded.schema_name,
			title_field = excluded.title_field,
			summary_field = excluded.summary_field,
			content_field = excluded.content_field,
			extra_fields_json = excluded.extra_fields_json,
			updated_at = excluded.updated_at
	`, mode.Name, mode.SystemPrompt, mode.TaskPrompt, temperature, mode.MaxOutputTokens,
		mode.OutputSchema.Name, mode.OutputSchema.TitleField, mode.OutputSchema.SummaryField, mode.OutputSchema.ContentField,
		extraJSON, time.Now().UTC())
	return err
}

func (s *SQLiteStore) GetMode(ctx context.Context, name string) (model.Mode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, system_prompt, task_prompt, temperature, max_output_tokens,
		       schema_name, title_field, summary_field, content_field, extra_fields_json
		FROM modes WHERE name = ?
	`, name)
	return scanMode(row)
}

func (s *SQLiteStore) ListModes(ctx context.Context) ([]model.Mode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, system_prompt, task_prompt, temperature, max_output_tokens,
		       schema_name, title_field, summary_field, content_field, extra_fields_json
		FROM modes ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modes []model.Mode
	for rows.Next() {
		mode, err := scanMode(rows)
		if err != nil {
			return nil, err
		}
		modes = append(modes, mode)
	}
	return modes, rows.Err()
}

func (s *SQLiteStore) UpsertSource(ctx context.Context, source model.Source) error {
	var temperature any
	if source.Temperature != nil {
		temperature = *source.Temperature
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO source_configs (
			id, name, url, refresh_interval, enabled, max_items, pipeline_mode,
			system_prompt, task_prompt, max_input_chars, extract_full_content,
			temperature, max_output_tokens, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			url = excluded.url,
			refresh_interval = excluded.refresh_interval,
			enabled = excluded.enabled,
			max_items = excluded.max_items,
			pipeline_mode = excluded.pipeline_mode,
			system_prompt = excluded.system_prompt,
			task_prompt = excluded.task_prompt,
			max_input_chars = excluded.max_input_chars,
			extract_full_content = excluded.extract_full_content,
			temperature = excluded.temperature,
			max_output_tokens = excluded.max_output_tokens,
			updated_at = excluded.updated_at
	`, source.ID, source.Name, source.URL, source.RefreshInterval.String(), boolToInt(source.Enabled), source.MaxItems, source.PipelineMode,
		source.SystemPrompt, source.TaskPrompt, source.MaxInputChars, boolToInt(source.ExtractFull),
		temperature, source.MaxOutputTokens, time.Now().UTC())
	return err
}

func (s *SQLiteStore) GetSource(ctx context.Context, id string) (model.Source, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, url, refresh_interval, enabled, max_items, pipeline_mode,
		       system_prompt, task_prompt, max_input_chars, extract_full_content,
		       temperature, max_output_tokens
		FROM source_configs WHERE id = ?
	`, id)
	return scanSource(row)
}

func (s *SQLiteStore) ListSources(ctx context.Context) ([]model.Source, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, url, refresh_interval, enabled, max_items, pipeline_mode,
		       system_prompt, task_prompt, max_input_chars, extract_full_content,
		       temperature, max_output_tokens
		FROM source_configs ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.Source
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
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
			&item.SourceID, &item.GUID, &item.Title, &item.Link, &item.Description,
			&item.ContentHTML, &item.ContentText, &item.Author, &item.PublishedAt,
			&item.Hash, &item.FetchedAt,
		); err != nil {
			return nil, err
		}
		item.Content = item.ContentText
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetRawItem(ctx context.Context, sourceID, guid string) (model.RawItem, error) {
	var item model.RawItem
	err := s.db.QueryRowContext(ctx, `
		SELECT source_id, guid, title, link, description, content_html,
		       content_text, author, published_at, content_hash, fetched_at
		FROM raw_items
		WHERE source_id = ? AND guid = ?
		LIMIT 1
	`, sourceID, guid).Scan(
		&item.SourceID, &item.GUID, &item.Title, &item.Link, &item.Description,
		&item.ContentHTML, &item.ContentText, &item.Author, &item.PublishedAt,
		&item.Hash, &item.FetchedAt,
	)
	if err != nil {
		return model.RawItem{}, err
	}
	item.Content = item.ContentText
	return item, nil
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
			&item.SourceID, &item.GUID, &item.OriginalTitle, &item.OriginalLink, &item.PublishedAt,
			&item.OutputTitle, &item.OutputSummary, &item.OutputContent, &item.OutputJSON, &item.Model,
			&item.InputHash, &item.ProcessedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) GetProcessedItem(ctx context.Context, sourceID, guid string) (model.ProcessedItem, error) {
	var item model.ProcessedItem
	err := s.db.QueryRowContext(ctx, `
		SELECT source_id, guid, original_title, original_link, published_at,
		       output_title, output_summary, output_content, output_json, model, input_hash, processed_at
		FROM processed_items
		WHERE source_id = ? AND guid = ?
		LIMIT 1
	`, sourceID, guid).Scan(
		&item.SourceID, &item.GUID, &item.OriginalTitle, &item.OriginalLink, &item.PublishedAt,
		&item.OutputTitle, &item.OutputSummary, &item.OutputContent, &item.OutputJSON, &item.Model,
		&item.InputHash, &item.ProcessedAt,
	)
	if err != nil {
		return model.ProcessedItem{}, err
	}
	return item, nil
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
		&state.SourceID, &lastSuccess, &state.LastError, &state.LastFetchedCount,
		&state.LastProcessedCount, &state.LastReprocessedCount,
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
			&state.SourceID, &lastSuccess, &state.LastError,
			&state.LastFetchedCount, &state.LastProcessedCount, &state.LastReprocessedCount,
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

func scanSource(scanner interface{ Scan(dest ...any) error }) (model.Source, error) {
	var source model.Source
	var refreshInterval string
	var enabled int
	var extractFull int
	var temperature sql.NullFloat64
	err := scanner.Scan(
		&source.ID, &source.Name, &source.URL, &refreshInterval, &enabled, &source.MaxItems,
		&source.PipelineMode, &source.SystemPrompt, &source.TaskPrompt, &source.MaxInputChars,
		&extractFull, &temperature, &source.MaxOutputTokens,
	)
	if err != nil {
		return model.Source{}, err
	}
	duration, err := time.ParseDuration(refreshInterval)
	if err != nil {
		return model.Source{}, err
	}
	source.RefreshInterval = duration
	source.Enabled = enabled == 1
	source.ExtractFull = extractFull == 1
	if temperature.Valid {
		value := temperature.Float64
		source.Temperature = &value
	}
	return source, nil
}

func scanMode(scanner interface{ Scan(dest ...any) error }) (model.Mode, error) {
	var mode model.Mode
	var temperature sql.NullFloat64
	var extraFieldsRaw string
	err := scanner.Scan(
		&mode.Name, &mode.SystemPrompt, &mode.TaskPrompt, &temperature, &mode.MaxOutputTokens,
		&mode.OutputSchema.Name, &mode.OutputSchema.TitleField, &mode.OutputSchema.SummaryField,
		&mode.OutputSchema.ContentField, &extraFieldsRaw,
	)
	if err != nil {
		return model.Mode{}, err
	}
	if temperature.Valid {
		value := temperature.Float64
		mode.Temperature = &value
	}
	mode.OutputSchema.Fields = []model.OutputField{
		{Name: mode.OutputSchema.TitleField, Type: "string", Description: "Reader-facing title", Required: true},
		{Name: mode.OutputSchema.SummaryField, Type: "string", Description: "Short summary", Required: true},
		{Name: mode.OutputSchema.ContentField, Type: "string", Description: "RSS content body", Required: true},
	}
	var extras []model.OutputField
	if err := json.Unmarshal([]byte(extraFieldsRaw), &extras); err != nil {
		return model.Mode{}, err
	}
	mode.OutputSchema.Fields = append(mode.OutputSchema.Fields, extras...)
	return mode, nil
}

func extraFieldsJSON(schema model.OutputSchema) (string, error) {
	extras := make([]model.OutputField, 0)
	for _, field := range schema.Fields {
		if field.Name == schema.TitleField || field.Name == schema.SummaryField || field.Name == schema.ContentField {
			continue
		}
		extras = append(extras, field)
	}
	data, err := json.Marshal(extras)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func toRuntimeMode(name string, cfg config.ModeConfig) model.Mode {
	return model.Mode{
		Name:            name,
		SystemPrompt:    cfg.SystemPrompt,
		TaskPrompt:      cfg.TaskPrompt,
		Temperature:     cfg.Temperature,
		MaxOutputTokens: cfg.MaxOutputTokens,
		OutputSchema: model.OutputSchema{
			Name:         cfg.OutputSchema.Name,
			TitleField:   cfg.OutputSchema.TitleField,
			SummaryField: cfg.OutputSchema.SummaryField,
			ContentField: cfg.OutputSchema.ContentField,
			Fields:       toOutputFields(cfg.OutputSchema),
		},
	}
}

func toRuntimeSource(cfg config.Source) model.Source {
	enabled := true
	if cfg.Enabled != nil {
		enabled = *cfg.Enabled
	}
	return model.Source{
		ID:              cfg.ID,
		Name:            cfg.Name,
		URL:             cfg.URL,
		RefreshInterval: cfg.RefreshInterval.Duration,
		Enabled:         enabled,
		MaxItems:        cfg.MaxItems,
		PipelineMode:    cfg.Pipeline.Mode,
		SystemPrompt:    cfg.Pipeline.SystemPrompt,
		TaskPrompt:      cfg.Pipeline.TaskPrompt,
		MaxInputChars:   cfg.Pipeline.MaxInputChars,
		ExtractFull:     cfg.Pipeline.ExtractFullContent,
		Temperature:     cfg.Pipeline.Temperature,
		MaxOutputTokens: cfg.Pipeline.MaxOutputTokens,
	}
}

func toOutputFields(cfg config.OutputSchemaConfig) []model.OutputField {
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
	return fields
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
