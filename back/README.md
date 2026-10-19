# hbc-back

Бэкенд модуля «Стартапы и вакансии» Бизнес-клуба НИУ ВШЭ. Go, модульный монолит:
gRPC-сервис `hbc.v1.HseBusinessClub` (BFF) + HTTP/JSON через grpc-gateway, три
bounded-context'а (`users`, `listings`, `applications`) в своих схемах PostgreSQL,
фото профиля — в S3-совместимом хранилище (MinIO).

Контракт API — [`api/hbc/v1/hbc.proto`](api/hbc/v1/hbc.proto), архитектура и порты —
[`ARCHITECTURE.md`](ARCHITECTURE.md), продукт — [`../SPEC.md`](../SPEC.md).

## Требования

- Go 1.25+ (`go.mod`: `go 1.25.0`), Docker, [buf](https://buf.build) — только для `make generate`.
- protoc-плагины ставятся из секции `tool` в `go.mod` (`make tools` → `./bin`), отдельно их ставить не нужно.

## Быстрый старт

```sh
make test-env-up          # postgres:17 на :5433 и MinIO на :9000 (docker-compose.test.yaml)
cp .env.example .env      # .env читается только при APP_ENV=local
make run                  # :8080 — API, :8081 — debug, gRPC — 127.0.0.1:8082
```

Проверка:

```sh
curl localhost:8080/v1/ping      # {"ok":true}
curl localhost:8080/readyz       # {"db":"ok","blob":"ok"}
curl localhost:8080/v1/me        # 401 {"code":16,"message":"Требуется вход","details":[]}
open http://localhost:8081/docs/ # Swagger UI («Try it out» ходит в API через тот же порт)
curl localhost:8081/metrics      # Prometheus
```

Трейсы: `docker run --rm -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one:1.60`,
`OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 make run`, UI — http://localhost:16686.

## Make-цели

| Цель | Что делает |
|---|---|
| `make generate` | `buf generate`: `pkg/api/hbc/v1/*` (pb, grpc, gateway, swagger). Результат коммитится |
| `make lint` | `buf lint` + golangci-lint (собирается локальным Go, версия зафиксирована в Makefile) |
| `make test` | unit-тесты (`-race`) |
| `make test-integration` | тесты с тегом `integration` на postgres/MinIO из `docker-compose.test.yaml` |
| `make test-e2e` | HTTP e2e-тесты (тег `e2e`, задача B5) |
| `make run` / `make build` | запуск / сборка `./build/hbc` с версией из git |
| `make migration-create ctx=users name=init` | новая SQL-миграция в `internal/<ctx>/postgres/migrations` |
| `make test-env-up` / `make test-env-down` | поднять / удалить тестовую инфраструктуру |

## Переменные окружения

| Имя | По умолчанию | Назначение |
|---|---|---|
| `APP_ENV` | `local` | `local` подхватывает `.env`; `production` — `JWT_SECRET` ≥ 32 байт; также `test` |
| `HTTP_ADDR` | `:8080` | публичный: `/v1/*`, `/healthz`, `/readyz` |
| `DEBUG_ADDR` | `:8081` | `/metrics`, `/debug/pprof/`, `/docs/`, `/healthz`, `/readyz`, `/v1/*` (для Swagger UI) |
| `GRPC_ADDR` | `127.0.0.1:8082` | внутренний gRPC (gateway ходит в него по сети) |
| `POSTGRES_DSN` | — | **обязателен** |
| `POSTGRES_MAX_CONNS` | `10` | |
| `JWT_SECRET` | — | **обязателен** |
| `JWT_TTL` | `24h` | |
| `CORS_ORIGINS` | `http://localhost:3000` | через запятую |
| `SEED_DEMO_DATA` | `false` | демо-данные (SPEC §6) |
| `S3_ENDPOINT` | — | **обязателен**, `host:port` без схемы |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | — | **обязательны** |
| `S3_BUCKET` | `hbc-avatars` | создаётся при старте, если нет |
| `S3_USE_SSL` | `false` | |
| `S3_REGION` | `us-east-1` | |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/gRPC, например `localhost:4317`; пусто — не экспортировать |
| `OTEL_SERVICE_NAME` | `hbc-back` | |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | доля трейсов, 0..1 |
| `SHUTDOWN_TIMEOUT` | `10s` | graceful shutdown |

## Docker

```sh
docker build -t hbc-back .
```

Многостадийная сборка `golang:1.25-alpine` → `alpine:3.20`, только публичные модули
(без SSH), пользователь без root, `HEALTHCHECK` на `/healthz`. Наружу — 8080 и 8081.

## Структура

```
cmd/hbc/                 composition root: config → logger → telemetry → pool → blob → app → run
internal/app/            сборка приложения (Deps{Log, Telemetry, Pool, Blob}), миграции контекстов
internal/bff/            реализация hbc.v1 (пока V1Ping, остальное — Unimplemented)
internal/users|listings|applications/   порты контекстов (ARCHITECTURE §2)
internal/platform/       apperr, config, logger, telemetry, db, auth, blob, server
pkg/api/hbc/v1/          сгенерированный код + embed swagger
```
