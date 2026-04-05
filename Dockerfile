FROM --platform=$BUILDPLATFORM golang:1.26 AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/rss-gateway ./cmd/server

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=build /out/rss-gateway /app/rss-gateway
COPY configs/config.toml /app/configs/config.toml

EXPOSE 8080

ENTRYPOINT ["/app/rss-gateway"]
CMD ["-config", "/app/configs/config.toml"]
