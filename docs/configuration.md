# 配置指南

[English Version](./configuration.en.md)

## 配置来源

项目当前有两层配置来源：

- `configs/config.toml`
- SQLite 里的运行时配置

规则是：

- `config.toml` 用于首次启动时的初始化 seed
- 如果数据库里已经有 LLM 设置、modes、sources，后续启动不会再被 TOML 覆盖
- Web 页面保存的内容会立即写入数据库并生效

## 配置文件位置

默认配置文件路径：

```text
configs/config.toml
```

如果启动时文件不存在，服务会自动创建默认配置。

## 顶层结构

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
  - HTTP 监听地址
  - 默认值：`:8080`

## `storage`

- `path`
  - SQLite 文件路径
  - 默认值：`data/rss-gateway.db`

如果数据库文件不存在，服务会自动创建。

## `llm`

- `provider`
  - 当前支持 `openai`
- `model`
  - 使用的模型名
- `api_key`
  - 初始 API key，可留空
- `base_url`
  - OpenAI 兼容网关地址
- `timeout`
  - LLM 请求超时，例如 `60s`

推荐做法：

- 把 `provider`、`model`、`base_url` 放在 TOML 里
- 把 `api_key` 留空
- 首次启动后到 `/settings/llm` 页面里填写并保存

## `modes`

`mode` 定义 LLM 如何处理文章。

示例：

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

可配置项：

- `system_prompt`
- `task_prompt`
- `temperature`
- `max_output_tokens`
- `output_schema`

`output_schema` 默认至少包含：

- `title`
- `summary`
- `content`

也支持额外字段：

```toml
[[modes.translate_zh.output_schema.extra_fields]]
name = "keywords"
type = "array"
description = "A short list of important keywords."
required = false
```

## `sources`

每个 source 对应一个上游 RSS/Atom 源。

示例：

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

source 字段：

- `id`
- `name`
- `url`
- `refresh_interval`
- `enabled`
- `max_items`

`pipeline` 字段：

- `mode`
- `system_prompt`
- `task_prompt`
- `max_input_chars`
- `extract_full_content`
- `temperature`
- `max_output_tokens`

覆盖规则：

- source 的 `pipeline.system_prompt` / `pipeline.task_prompt` 会覆盖 mode 默认值
- source 的 `temperature` / `max_output_tokens` 也会覆盖 mode 默认值

## Web 编辑与 TOML 的关系

你可以在页面里编辑：

- LLM 设置
- modes
- sources

页面入口：

- `/settings/llm`
- `/modes`
- `/sources`

建议把 TOML 看成“首启模板”，把数据库看成“运行时真相源”。
