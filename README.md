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
- 支持在管理页面中编辑 LLM 设置、modes 和 sources

## 快速开始

1. 启动服务：

```powershell
go run ./cmd/server -config configs/config.toml
```

2. 打开这些地址：

- `http://localhost:8080/healthz`
- `http://localhost:8080/`
- `http://localhost:8080/sources`
- `http://localhost:8080/items?source=hackernews-summary&guid=<guid>`
- `http://localhost:8080/api/sources`
- `http://localhost:8080/api/status`
- `http://localhost:8080/feeds/hackernews-summary.rss`

3. 第一次使用时，到 `http://localhost:8080/settings/llm` 填写 LLM provider、model、API key 和 base URL。

4. 手动触发刷新：

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/api/refresh
Invoke-WebRequest -Method POST "http://localhost:8080/api/refresh?source=hackernews-summary"
Invoke-WebRequest -Method POST "http://localhost:8080/api/reprocess?source=hackernews-summary&limit=10"
```

## 文档

- [配置指南](./docs/configuration.md)
- [API 参考](./docs/api-reference.md)
- [部署指南](./docs/deployment.md)

## 设计说明

- 默认配置文件是 `configs/config.toml`
- 运行时配置保存在 SQLite，Web 页面可直接编辑
- HTTP 服务优先启动，首次 refresh 在后台异步执行
- OpenAI provider 当前使用 `/chat/completions`
- 输出 RSS 入口是 `/feeds/{sourceID}.rss`
