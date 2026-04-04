# rss-gateway

A Go service that sits between upstream RSS feeds and your RSS reader, adding LLM processing and emitting transformed RSS feeds.

## Features

- Pull RSS or Atom feeds on a schedule
- Transform items with an LLM pipeline
- Output new RSS feeds over HTTP
- Store processed results in SQLite
- Support custom OpenAI-compatible `base_url`

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
- `http://localhost:8080/feeds/hackernews-summary.rss`

4. Manual refresh:

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/admin/refresh
Invoke-WebRequest -Method POST "http://localhost:8080/admin/refresh?source=hackernews-summary"
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

## Notes

- This v1 processes feed-provided content only.
- If an item has already been processed for the same source and GUID, it is skipped.
- The OpenAI integration uses the `/responses` API and expects JSON output from the model prompt.
- If a source references an undefined mode, startup fails unless that source provides inline prompt overrides.
