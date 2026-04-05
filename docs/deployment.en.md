# Deployment Guide

[中文版](./deployment.md)

## Local Run

```powershell
go run ./cmd/server -config configs/config.toml
```

On first startup:

- if `configs/config.toml` does not exist, a default file is created automatically
- if the database file does not exist, SQLite is initialized automatically

After startup, open:

- `http://localhost:8080/`
- `http://localhost:8080/settings/llm`

## Docker

Build locally:

```powershell
docker build -t rss-gateway .
docker run --rm -p 8080:8080 -v ${PWD}/configs/config.toml:/app/configs/config.toml:ro -v ${PWD}/data:/app/data rss-gateway
```

## Docker Compose

The repository `docker-compose.yml` now pulls from GHCR directly:

```powershell
docker compose up -d
```

It uses:

- `ghcr.io/batkiz/rss-gateway:latest`
- `./configs/config.toml`
- `./data`

## Container Image

Image:

```text
ghcr.io/batkiz/rss-gateway
```

## GitHub Actions

Current workflows:

- `ci`
  - runs `go test ./...`
  - runs `go build ./...`
  - builds multi-arch Docker images
- `release`
  - builds multi-platform binaries after a GitHub Release is published
  - uploads release assets
- `tag-release`
  - accepts a version number
  - creates a tag and GitHub Release

## Release Flow

Recommended flow:

1. Run `tag-release` manually in GitHub Actions
2. Enter a version such as `0.2.0`
3. The workflow creates the `v0.2.0` tag and Release
4. The `release` workflow builds binaries for multiple architectures and attaches them to the Release

## Homelab Tips

- mount both `configs/` and `data/` as persistent volumes
- after first startup, fill in the API key in `/settings/llm`
- if you use an OpenAI-compatible gateway, make sure `base_url` points to a compatible `/v1` endpoint
