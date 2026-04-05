# API 参考

[English Version](./api-reference.en.md)

## 页面路由

- `GET /`
  - 仪表盘页面
- `GET /settings/llm`
  - LLM 设置页面
- `GET /modes`
  - mode 管理页面
- `GET /sources`
  - source 管理页面
- `GET /items?source=<id>&guid=<guid>`
  - 单条 item 页面
  - 可查看原始内容、处理结果、预览 prompt、重跑单条 item

页面支持：

- `?lang=zh`
- `?lang=en`

## 系统接口

### `GET /healthz`

返回服务健康状态。

示例响应：

```json
{"status":"ok"}
```

## API 接口

### `GET /api/sources`

返回当前 source 列表。

### `GET /api/status`

返回每个 source 的刷新状态和计数信息。

主要字段包括：

- `source_id`
- `last_success_at`
- `last_error`
- `last_fetched_count`
- `last_processed_count`
- `last_reprocessed_count`
- `raw_item_count`
- `processed_item_count`

### `POST /api/refresh`

刷新全部启用的 source。

示例：

```powershell
Invoke-WebRequest -Method POST http://localhost:8080/api/refresh
```

### `POST /api/refresh?source=<id>`

只刷新指定 source。

示例：

```powershell
Invoke-WebRequest -Method POST "http://localhost:8080/api/refresh?source=hackernews-summary"
```

### `POST /api/reprocess?source=<id>&limit=<n>`

基于已存储的原始条目重新处理最近 N 条。

示例：

```powershell
Invoke-WebRequest -Method POST "http://localhost:8080/api/reprocess?source=hackernews-summary&limit=10"
```

### `GET /api/raw-items?source=<id>&limit=<n>`

返回最近保存的原始条目。

## 配置保存接口

### `POST /api/settings/llm`

保存运行时 LLM 设置。

表单字段：

- `provider`
- `model`
- `api_key`
- `base_url`
- `timeout`

### `POST /api/settings/mode`

保存 mode。

表单字段：

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

保存 source。

表单字段：

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

## Feed 输出

### `GET /feeds/{sourceID}.rss`

返回处理后的 RSS。

示例：

```text
/feeds/hackernews-summary.rss
```
