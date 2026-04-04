# rss-gateway

[English README](./README.en.md)

一个使用 Go 实现的 RSS 中间服务，用来为上游 RSS 增加 LLM 处理能力，并继续以 RSS 输出。

## 功能特性

- 定时拉取 RSS 或 Atom
- 可选抓取文章链接页并提取更干净的正文
- 通过配置化 mode 执行 LLM 处理
- 通过 HTTP 输出新的 RSS feed
- 使用 SQLite 存储原始条目、处理结果和 feed 状态
- 支持自定义 OpenAI 兼容 `base_url`
- 支持基于已存储原始数据重处理最近条目
- 提供一个简单的中英文管理页面

## 快速开始

1. 设置 API Key：

```powershell
$env:OPENAI_API_KEY="your-key"
```

2. 启动服务：

```powershell
go run ./cmd/server -config configs/config.example.toml
```

3. 打开这些地址：

- `http://localhost:8080/healthz`
- `http://localhost:8080/sources`
- `http://localhost:8080/admin`
- `http://localhost:8080/admin/status`
- `http://localhost:8080/feeds/hackernews-summary.rss`

4. 手动触发刷新：

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/admin/refresh
Invoke-WebRequest -Method POST "http://localhost:8080/admin/refresh?source=hackernews-summary"
Invoke-WebRequest -Method POST "http://localhost:8080/admin/reprocess?source=hackernews-summary&limit=10"
```

## 配置

当前只支持 TOML 配置。

`llm.base_url` 可用于接入 OpenAI 兼容网关：

```toml
[llm]
provider = "openai"
model = "gpt-4.1-mini"
api_key_env = "OPENAI_API_KEY"
base_url = "https://api.openai.com/v1"
```

## Mode 配置

mode 完全由配置驱动。先定义 mode，再让 source 引用：

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

source 级别的 `pipeline.system_prompt`、`pipeline.task_prompt` 可以覆盖 mode 默认值。  
`pipeline.temperature`、`pipeline.max_output_tokens`、`pipeline.extract_full_content` 也可以逐 source 覆盖。

## 管理接口

- `GET /admin`：管理页面，支持 `?lang=zh|en`
- `GET /admin/status`：按 source 查看刷新状态和条目计数
- `POST /admin/refresh?source=<id>`：拉取并处理最新 feed
- `POST /admin/reprocess?source=<id>&limit=<n>`：基于原始条目重新跑 LLM
- `GET /admin/raw-items?source=<id>&limit=<n>`：查看最近保存的原始条目

## 部署

支持 Docker：

```powershell
docker build -t rss-gateway .
docker run --rm -p 8080:8080 -e OPENAI_API_KEY=your-key rss-gateway
```

## CI 与 Release

GitHub Actions 当前包含两类流程：

- `ci`：运行 `go test ./...`、`go build ./...`，并构建 `linux/amd64` 与 `linux/arm64` Docker 镜像
- `release`：发布 GitHub Release 时，构建 `linux`、`darwin`、`windows` 的 `amd64` / `arm64` 二进制并上传为 release asset

容器镜像会发布到：

```text
ghcr.io/batkiz/rss-gateway
```

## 说明

- OpenAI provider 当前使用 `/chat/completions`，并通过 `response_format` 应用 JSON Schema。
- 原始条目会先落库，再根据输入内容变化决定是否重新处理。
- HTTP 服务会优先启动，首次 refresh 在后台异步执行。
- 如果 source 引用了未定义的 mode，且又没有提供内联 prompt 覆盖，启动会失败。
