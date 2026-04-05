# rss-gateway

[中文版 README](./README.md)

A Go-based RSS gateway that adds LLM processing to upstream feeds and emits transformed RSS feeds over HTTP.

## Features

- Pull RSS or Atom feeds on a schedule
- Optionally fetch linked article pages and extract cleaner text
- Transform items with config-driven LLM modes
- Output new RSS feeds over HTTP
- Store raw items, processed results, and feed state in SQLite
- Support custom OpenAI-compatible `base_url`
- Reprocess recent items from stored raw content
- Provide a simple bilingual admin UI
- Edit runtime LLM settings, modes, and sources from the admin UI

## Quick Start

1. Run the server:

```powershell
go run ./cmd/server -config configs/config.toml
```

2. Open:

- `http://localhost:8080/healthz`
- `http://localhost:8080/`
- `http://localhost:8080/sources`
- `http://localhost:8080/items?source=hackernews-summary&guid=<guid>`
- `http://localhost:8080/api/sources`
- `http://localhost:8080/api/status`
- `http://localhost:8080/feeds/hackernews-summary.rss`

3. On first use, open `http://localhost:8080/settings/llm` and fill in the LLM provider, model, API key, and base URL.

4. Trigger refresh manually:

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/api/refresh
Invoke-WebRequest -Method POST "http://localhost:8080/api/refresh?source=hackernews-summary"
Invoke-WebRequest -Method POST "http://localhost:8080/api/reprocess?source=hackernews-summary&limit=10"
```

## Configuration

Only TOML is supported.

The web pages can now edit runtime configuration directly:

- LLM provider / model / API key / base URL / timeout
- modes
- sources

These values are stored in SQLite and apply immediately after save.  
TOML is still supported, but mainly as the initial seed on first startup. Once runtime config already exists in SQLite, later restarts will not overwrite it from TOML.

`llm.base_url` can be used with OpenAI-compatible gateways. `api_key` can be stored directly in TOML, or left empty and filled in from the web UI:

```toml
[llm]
provider = "openai"
model = "gpt-4.1-mini"
api_key = ""
base_url = "https://api.openai.com/v1"
```

## Mode Configuration

Modes can still be defined in TOML for the initial seed, and then edited directly in the web UI. Define a mode, then reference it from a source:

```toml
[modes.summary]
system_prompt = "..."
temperature = 0.2
max_output_tokens = 900
task_prompt = """
1. Keep or lightly rewrite the title for clarity.
2. Write a short summary in 3 to 5 sentences.
3. Produce concise output content suitable for an RSS reader.
"""

[modes.summary.output_schema]
name = "summary"
title_field = "title"
summary_field = "summary"
content_field = "content"

[[sources]]
id = "hackernews-summary"
url = "https://news.ycombinator.com/rss"

[sources.pipeline]
mode = "summary"
```

Source-level `pipeline.system_prompt` and `pipeline.task_prompt` can override mode defaults.  
`pipeline.temperature`, `pipeline.max_output_tokens`, and `pipeline.extract_full_content` can also override mode values per source.

## Admin Endpoints

- `GET /`: dashboard page, supports `?lang=zh|en`
- `GET /settings/llm`: LLM settings page
- `GET /modes`: mode management page
- `GET /sources`: source management page
- `GET /items?source=<id>&guid=<guid>`: single-item page for full content inspection, prompt preview, and one-off reprocessing
- `POST /api/settings/llm`: save runtime LLM settings
- `POST /api/settings/mode`: save a mode
- `POST /api/settings/source`: save a source
- `GET /api/status`: per-source refresh status and item counts
- `POST /api/refresh?source=<id>`: fetch and process the latest feed
- `POST /api/reprocess?source=<id>&limit=<n>`: rerun LLM processing from stored raw items
- `GET /api/raw-items?source=<id>&limit=<n>`: inspect recent stored raw items
- `GET /api/sources`: return the source list as JSON

## Deployment

Docker is supported:

```powershell
docker build -t rss-gateway .
docker run --rm -p 8080:8080 -v ${PWD}/configs/config.toml:/app/configs/config.toml:ro -v ${PWD}/data:/app/data rss-gateway
```

You can also use the included `docker-compose.yml` to pull the image from GHCR directly:

```powershell
docker compose up -d
```

## CI And Release

GitHub Actions currently includes:

- `ci`: runs `go test ./...`, `go build ./...`, and builds Docker images for `linux/amd64` and `linux/arm64`
- `release`: on GitHub Release publication, builds binaries for `linux`, `darwin`, and `windows` on `amd64` / `arm64` and uploads them as release assets
- `tag-release`: manually create a `vX.Y.Z` tag and GitHub release with auto-generated release notes as the changelog

Container images are published to:

```text
ghcr.io/batkiz/rss-gateway
```

## Notes

- The OpenAI provider currently uses `/chat/completions` with structured `response_format`.
- Raw items are persisted first, then reprocessed only when input content changes or reprocessing is requested.
- The single-item page lets you temporarily override mode, prompts, temperature, and token limits for preview, then save a reprocess for just that item.
- Linked article extraction now applies more aggressive cleaning, candidate scoring, and fallback selection to pick content that looks more like the main article body.
- The HTTP server starts first, and the initial refresh runs asynchronously in the background.
- Startup fails if a source references an undefined mode without inline prompt overrides.
