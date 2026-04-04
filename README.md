# rss-gateway

A Go service that sits between upstream RSS feeds and your RSS reader, adding LLM processing and emitting transformed RSS feeds.

## Features

- Pull RSS or Atom feeds on a schedule
- Optionally fetch linked article pages and extract cleaner text
- Transform items with config-driven LLM modes
- Output new RSS feeds over HTTP
- Store raw items, processed results, and feed status in SQLite
- Support custom OpenAI-compatible `base_url`
- Reprocess recent items without refetching the feed

## Quick start

1. Set your API key:

```powershell
$env:OPENAI_API_KEY="your-key"
```

2. Run the server:

```powershell
go run ./cmd/server -config configs/config.example.yaml
```

3. Open:

- `http://localhost:8080/healthz`
- `http://localhost:8080/sources`
- `http://localhost:8080/admin/status`
- `http://localhost:8080/feeds/hackernews-summary.rss`

4. Manual refresh:

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/admin/refresh
Invoke-WebRequest -Method POST "http://localhost:8080/admin/refresh?source=hackernews-summary"
Invoke-WebRequest -Method POST "http://localhost:8080/admin/reprocess?source=hackernews-summary&limit=10"
```

## Configuration

`llm.base_url` is configurable for OpenAI-compatible gateways:

```yaml
llm:
  provider: "openai"
  model: "gpt-4.1-mini"
  api_key_env: "OPENAI_API_KEY"
  base_url: "https://api.openai.com/v1"
```

## Mode configuration

Modes are now config-driven. Define them once and let each source reference a mode name:

```yaml
modes:
  summary:
    system_prompt: "..."
    temperature: 0.2
    max_output_tokens: 900
    output_schema:
      name: "summary"
      title_field: "title"
      summary_field: "summary"
      content_field: "content"
    task_prompt: |
      1. Keep or lightly rewrite the title for clarity.
      2. Write a short summary in 3 to 5 sentences.
      3. Produce concise output content suitable for an RSS reader.

sources:
  - id: "hackernews-summary"
    url: "https://news.ycombinator.com/rss"
    pipeline:
      mode: "summary"
```

Source-level `pipeline.system_prompt` and `pipeline.task_prompt` can still override the referenced mode definition for a specific feed.

Source-level `pipeline.temperature`, `pipeline.max_output_tokens`, and `pipeline.extract_full_content` can override mode defaults per feed.

## Admin endpoints

- `GET /admin/status`: per-source refresh state and item counts
- `POST /admin/refresh?source=<id>`: fetch latest feed items and process changed content
- `POST /admin/reprocess?source=<id>&limit=<n>`: rerun LLM processing from stored raw items
- `GET /admin/raw-items?source=<id>&limit=<n>`: inspect recent stored raw items

## Deployment

Docker support is included:

```powershell
docker build -t rss-gateway .
docker run --rm -p 8080:8080 -e OPENAI_API_KEY=your-key rss-gateway
```

## CI And Release

GitHub Actions now does two things:

- `ci`: runs `go test ./...`, `go build ./...`, and builds a multi-arch Docker image for `linux/amd64` and `linux/arm64`
- `release`: when a GitHub Release is published, builds binaries for `linux`, `darwin`, and `windows` on `amd64` and `arm64`, then uploads them to the release assets

Container images are pushed to GitHub Container Registry under:

```text
ghcr.io/batkiz/rss-gateway
```

## Notes

- The OpenAI integration uses the `/chat/completions` API and applies JSON schema via `response_format`.
- Raw items are persisted first, then processed items are updated only when input content changes or a reprocess is requested.
- If a source references an undefined mode, startup fails unless that source provides inline prompt overrides.
