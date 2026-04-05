# 部署指南

[English Version](./deployment.en.md)

## 本地运行

```powershell
go run ./cmd/server -config configs/config.toml
```

首次启动时：

- 如果 `configs/config.toml` 不存在，会自动生成默认配置
- 如果数据库文件不存在，会自动初始化 SQLite

启动后先访问：

- `http://localhost:8080/`
- `http://localhost:8080/settings/llm`

## Docker

本地构建：

```powershell
docker build -t rss-gateway .
docker run --rm -p 8080:8080 -v ${PWD}/configs/config.toml:/app/configs/config.toml:ro -v ${PWD}/data:/app/data rss-gateway
```

## Docker Compose

仓库内的 `docker-compose.yml` 现在直接使用 GHCR 镜像：

```powershell
docker compose up -d
```

它会使用：

- `ghcr.io/batkiz/rss-gateway:latest`
- `./configs/config.toml`
- `./data`

## 容器镜像

镜像地址：

```text
ghcr.io/batkiz/rss-gateway
```

## GitHub Actions

当前包含这些工作流：

- `ci`
  - 运行 `go test ./...`
  - 运行 `go build ./...`
  - 构建多架构 Docker 镜像
- `release`
  - 在 GitHub Release 发布后构建多平台二进制
  - 上传 release assets
- `tag-release`
  - 手动输入版本号
  - 创建 tag 和 GitHub Release

## 发布流程

推荐方式：

1. 在 GitHub Actions 里手动运行 `tag-release`
2. 输入版本号，例如 `0.2.0`
3. 工作流创建 `v0.2.0` tag 和 Release
4. `release` workflow 自动构建各架构 binary 并附加到 Release

## Homelab 场景建议

- 把 `configs/` 和 `data/` 都挂载成持久卷
- 首次启动后优先到 `/settings/llm` 补齐 API key
- 如果你使用 OpenAI 兼容网关，确认 `base_url` 指向兼容的 `/v1`
