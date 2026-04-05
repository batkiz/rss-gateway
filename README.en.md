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

## Documentation

- [Configuration Guide](./docs/configuration.en.md)
- [API Reference](./docs/api-reference.en.md)
- [Deployment Guide](./docs/deployment.en.md)

## Notes

- The default config file is `configs/config.toml`
- Runtime configuration is stored in SQLite and editable from the web UI
- The HTTP server starts first, and the initial refresh runs asynchronously
- The OpenAI provider currently uses `/chat/completions`
- Transformed RSS feeds are exposed at `/feeds/{sourceID}.rss`
