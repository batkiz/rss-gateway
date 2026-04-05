# Configuration Guide

[中文版](./configuration.md)

## Configuration Sources

The project currently has two configuration layers:

- `configs/config.toml`
- runtime configuration stored in SQLite

The rules are:

- `config.toml` is used as the initial seed on first startup
- once the database already contains LLM settings, modes, and sources, later restarts do not overwrite them from TOML
- values saved from the web UI are written to SQLite immediately and take effect right away

## Config File Location

Default path:

```text
configs/config.toml
```

If the file does not exist on startup, the service creates a default one automatically.

## Top-Level Structure

```toml
[server]
addr = ":8080"

[storage]
path = "data/rss-gateway.db"

[llm]
provider = "openai"
model = "gpt-4.1-mini"
api_key = ""
base_url = "https://api.openai.com/v1"
timeout = "60s"
```

## `server`

- `addr`
  - HTTP listen address
  - default: `:8080`

## `storage`

- `path`
  - SQLite file path
  - default: `data/rss-gateway.db`

If the database file does not exist, the service creates it automatically.

## `llm`

- `provider`
  - currently supports `openai`
- `model`
  - model name to call
- `api_key`
  - initial API key, may be left empty
- `base_url`
  - OpenAI-compatible gateway URL
- `timeout`
  - LLM request timeout, for example `60s`

Recommended setup:

- keep `provider`, `model`, and `base_url` in TOML
- leave `api_key` empty
- fill and save it later from `/settings/llm`

## `modes`

A mode defines how the LLM transforms an article.

Example:

```toml
[modes.summary]
system_prompt = "You transform RSS articles into concise reader-friendly summaries."
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
```

Configurable fields:

- `system_prompt`
- `task_prompt`
- `temperature`
- `max_output_tokens`
- `output_schema`

The default schema always includes:

- `title`
- `summary`
- `content`

Extra fields are also supported:

```toml
[[modes.translate_zh.output_schema.extra_fields]]
name = "keywords"
type = "array"
description = "A short list of important keywords."
required = false
```

## `sources`

Each source maps to an upstream RSS or Atom feed.

Example:

```toml
[[sources]]
id = "hackernews-summary"
name = "Hacker News Summary"
url = "https://news.ycombinator.com/rss"
refresh_interval = "10m"
enabled = true
max_items = 15

[sources.pipeline]
mode = "summary"
max_input_chars = 6000
extract_full_content = true
```

Source fields:

- `id`
- `name`
- `url`
- `refresh_interval`
- `enabled`
- `max_items`

`pipeline` fields:

- `mode`
- `system_prompt`
- `task_prompt`
- `max_input_chars`
- `extract_full_content`
- `temperature`
- `max_output_tokens`

Override rules:

- source-level `pipeline.system_prompt` and `pipeline.task_prompt` override mode defaults
- source-level `temperature` and `max_output_tokens` also override mode defaults

## Web Editing vs TOML

You can edit these in the UI:

- LLM settings
- modes
- sources

Pages:

- `/settings/llm`
- `/modes`
- `/sources`

Treat TOML as the initial template, and SQLite as the runtime source of truth.
