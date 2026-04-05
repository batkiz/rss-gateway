# API Reference

[中文版](./api-reference.md)

## Page Routes

- `GET /`
  - dashboard page
- `GET /settings/llm`
  - LLM settings page
- `GET /modes`
  - mode management page
- `GET /sources`
  - source management page
- `GET /items?source=<id>&guid=<guid>`
  - single-item page
  - inspect raw content, processed output, prompt preview, and rerun one item

Supported page query parameters:

- `?lang=zh`
- `?lang=en`

## System Endpoint

### `GET /healthz`

Returns service health.

Example response:

```json
{"status":"ok"}
```

## API Endpoints

### `GET /api/sources`

Returns the current source list.

### `GET /api/status`

Returns refresh status and counters for each source.

Key fields include:

- `source_id`
- `last_success_at`
- `last_error`
- `last_fetched_count`
- `last_processed_count`
- `last_reprocessed_count`
- `raw_item_count`
- `processed_item_count`

### `POST /api/refresh`

Refresh all enabled sources.

Example:

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/api/refresh
```

### `POST /api/refresh?source=<id>`

Refresh a single source.

Example:

```powershell
Invoke-WebRequest -Method POST "http://localhost:8080/api/refresh?source=hackernews-summary"
```

### `POST /api/reprocess?source=<id>&limit=<n>`

Reprocess the latest N stored raw items.

Example:

```powershell
Invoke-WebRequest -Method POST "http://localhost:8080/api/reprocess?source=hackernews-summary&limit=10"
```

### `GET /api/raw-items?source=<id>&limit=<n>`

Returns recently stored raw items.

## Settings Save Endpoints

### `POST /api/settings/llm`

Save runtime LLM settings.

Form fields:

- `provider`
- `model`
- `api_key`
- `base_url`
- `timeout`

### `POST /api/settings/mode`

Save a mode.

Form fields:

- `name`
- `system_prompt`
- `task_prompt`
- `temperature`
- `max_output_tokens`
- `schema_name`
- `title_field`
- `summary_field`
- `content_field`
- `extra_fields_json`

### `POST /api/settings/source`

Save a source.

Form fields:

- `id`
- `name`
- `url`
- `refresh_interval`
- `enabled`
- `max_items`
- `pipeline_mode`
- `system_prompt`
- `task_prompt`
- `max_input_chars`
- `extract_full_content`
- `temperature`
- `max_output_tokens`

## Feed Output

### `GET /feeds/{sourceID}.rss`

Returns the transformed RSS feed.

Example:

```text
/feeds/hackernews-summary.rss
```
