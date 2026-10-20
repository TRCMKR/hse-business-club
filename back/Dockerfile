# syntax=docker/dockerfile:1

# ---------- build ----------
FROM golang:1.25-alpine AS builder

WORKDIR /src
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly

# Public modules only: no SSH, no GOPRIVATE.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# goose CLI for the entrypoint, at the version from go.mod, Postgres driver only.
# `go install pkg@version` resolves its own module graph, hence GOFLAGS is cleared.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOFLAGS= GOBIN=/out go install -trimpath -ldflags "-s -w" \
      -tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
      "github.com/pressly/goose/v3/cmd/goose@$(go list -m -f '{{.Version}}' github.com/pressly/goose/v3)"

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w \
        -X github.com/trcmkr/hse-business-club/back/internal/platform/telemetry.Version=${VERSION} \
        -X github.com/trcmkr/hse-business-club/back/internal/platform/telemetry.Commit=${COMMIT}" \
      -o /out/hbc ./cmd/hbc

# ---------- runtime ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 hbc \
    && adduser -S -D -H -u 10001 -G hbc hbc

COPY --from=builder /out/hbc /out/goose /usr/local/bin/
COPY migrations/ /app/migrations/
COPY --chmod=0755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER hbc:hbc

# 8080 — public API (/v1/*, /healthz, /readyz); 8081 — debug (/metrics, /debug/pprof, /docs/).
# gRPC listens on 127.0.0.1:8082 inside the container only.
EXPOSE 8080 8081

# The port is taken from HTTP_ADDR (":8080", "0.0.0.0:9090", …), default 8080.
ENV HTTP_ADDR=:8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD port="${HTTP_ADDR##*:}"; wget -q -O /dev/null "http://127.0.0.1:${port:-8080}/healthz" || exit 1

# Applies /app/migrations (goose up, waiting up to 30 s for the database), then execs hbc.
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
