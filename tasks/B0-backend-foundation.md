# B0 — Фундамент бэкенда

**Зависимости:** нет. **Блокирует:** B1–B5, I0.
**Читать:** `CLAUDE.md`, `SPEC.md` §4–5, `back/ARCHITECTURE.md` (целиком), `back/api/hbc/v1/hbc.proto`.
**Референсы (читать, не импортировать):** `/Users/trcmkr/Проекты/blanc-back` (Makefile, .golangci.yaml, logger),
`/Users/trcmkr/Проекты/hermes` (server.go, run.go, init.go, swagger.go).

## Цель

Запускаемый бинарник без бизнес-логики: `V1Ping` через gateway, health, метрики, трейсы,
swagger, миграции-раннер, JWT-интерцептор — и **порты контекстов**, по которым параллельно
пойдут B1–B4.

## Что сделать

1. `go mod init github.com/trcmkr/hse-business-club/back`, `go 1.25`. Секция `tool`:
   `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`.
   (В `blanc-back` вместо `go-grpc` стоят `go-vtproto` + `servitor`; здесь они не нужны —
   gRPC-код даёт `go-grpc`, swagger встраивается руками, см. ниже.)
   `buf generate` → `pkg/api/hbc/v1/{hbc.pb.go, hbc_grpc.pb.go, hbc.pb.gw.go, hbc.swagger.json}`
   + `pkg/api/hbc/v1/swagger.go` с `//go:embed hbc.swagger.json` и `func SwaggerJSON() []byte`.
   Сверить пути в swagger со SPEC §4.1. **Закоммитить отдельно.**
2. `internal/platform/apperr` — ровно по ARCHITECTURE §2 + тесты.
3. `internal/platform/config` — по ARCHITECTURE §5; `Load() (*Config, error)`; тест на
   дефолты и обязательные поля.
4. `internal/platform/logger` — zap JSON (взять из blanc-back), `FromContext(ctx)`,
   `WithContext(ctx, *zap.Logger)`, обогащение `trace_id`/`span_id`/`request_id`/`user_id`.
5. `internal/platform/telemetry` — `Init(ctx, cfg) (*Telemetry, error)` с `Shutdown(ctx)`:
   otel TracerProvider (OTLP gRPC exporter или no-op), Prometheus registry с Go runtime,
   build info; хелперы: `GRPCServerOptions()`, `HTTPMiddleware()`, `PgxTracer()`,
   `PoolCollector(pool)`.
6. `internal/platform/db` — `NewPool(ctx, cfg, tracer)`, `Migrator{Schema string; FS fs.FS;
   Dir string}` → `Up(ctx, pool)`: `CREATE SCHEMA IF NOT EXISTS`, goose provider с
   `search_path` и таблицей версий `<schema>.goose_db_version`; `WithTx(ctx, pool, fn)`.
7. `internal/platform/auth` — bcrypt-хелперы, `JWT{secret, ttl}` реализует `Issuer` и
   `Verifier` (HS256, claims `sub`, `exp`, `iat`), контекст-хелперы, `UnaryInterceptor` с
   `publicMethods`; тесты: issue/verify, протухший, чужой секрет, интерцептор пропускает
   public и режет остальные `Unauthenticated` «Требуется вход».
7a. `internal/platform/blob` — порт `Store` verbatim из ARCHITECTURE §2; `blob/memory`
   (map + mutex, тесты); `blob/s3` на `github.com/minio/minio-go/v7`: `New(ctx, cfg)`
   создаёт клиент (транспорт — `otelhttp.NewTransport`), `BucketExists`/`MakeBucket`,
   `Put`→`PutObject`, `Get`→`GetObject`+`Stat` (NoSuchKey → `ErrNotFound`), `Delete`,
   `Ping`→`BucketExists`; метрики по ARCHITECTURE §4. Интеграционный тест под
   `//go:build integration` с `TEST_S3_ENDPOINT` (MinIO из `docker-compose.test.yaml`:
   `minio/minio`, порт 9000, `minioadmin/minioadmin`).
8. `internal/platform/server` — `New(cfg, log, tel, verifier, services...)`:
   gRPC-сервер (`ChainUnaryInterceptor(recovery, otelgrpc, prometheus, logging, auth)`),
   gateway (`HTTPBodyMarshaler{JSONPb{EmitUnpopulated:true, DiscardUnknown:true}}`,
   incoming/outgoing header matcher, лимит сообщения 16 МБ, `DefaultHTTPErrorHandler` c
   подменой текста для `Internal` — всё по ARCHITECTURE §3), chi публичный (request-id,
   real-ip, otelhttp, http-метрики, recoverer, CORS, `/healthz`, `/readyz` с проверкой
   db и blob, mount gateway),
   chi debug (`/metrics`, `/debug/pprof`, `/docs/`, `/docs/swagger.json`, health);
   `Run(ctx)` и graceful shutdown по ARCHITECTURE §4. Интерфейс регистрируемого сервиса:
   `RegisterGRPC(grpc.ServiceRegistrar)`, `RegisterGateway(ctx, *runtime.ServeMux, *grpc.ClientConn) error`.
   Порядок подъёма — ARCHITECTURE §3 (как `blanc-back/cmd/app.go`): gRPC-сервер слушает
   `GRPC_ADDR` → `grpc.NewClient(GRPC_ADDR)` → `RegisterGateway(ctx, mux, conn)` → mount
   mux в chi на `HTTP_ADDR`. `RegisterXHandlerServer` не использовать. Тест: запрос в
   `/v1/ping` через HTTP проходит auth/logging-интерцепторы (видно по логу и
   `grpc_server_handled_total{grpc_method="V1Ping"}`).
9. Порты: `internal/users/users.go`, `internal/listings/listings.go`,
   `internal/applications/applications.go` — **verbatim** из ARCHITECTURE §2 (+ doc-комменты).
10. `internal/bff` скелет: `Implementation` с зависимостями-портами (поля `Users users.Service`
    и т. д., пока nil), `V1Ping`, `RegisterGRPC`, `RegisterGateway`, `errors.go`
    (`apperr.Kind` → `codes`), заглушки остальных 21 методов — `codes.Unimplemented`.
    `publicMethods` интерцептора: `V1Ping`, `V1Register`, `V1Login`, `V1GetAvatar`.
11. `cmd/hbc/main.go`: config → logger → telemetry → pool → blob (`s3.New`, бакет) →
    (миграции контекстов — пока пусто, но вызов `Migrator` на месте) → bff → server → run.
    Сборку вынести в `internal/app.New(ctx, cfg, Deps{Pool, Blob, ...})`, чтобы e2e (B5)
    мог подменить `blob.Store` на `memory`.
12. `Makefile` (`generate`, `lint`, `test`, `test-e2e` заглушка, `run`, `build`,
    `migration-create ctx=users name=...`), `.golangci.yaml` (из blanc-back), `Dockerfile`
    (multi-stage, `golang:1.25-alpine` → `alpine:3.20`, non-root, `HEALTHCHECK /healthz`,
    без SSH), `.dockerignore`, `.env.example`, `README.md` (запуск, env, генерация).
    `docker-compose.test.yaml` в `back/`: `postgres:17-alpine` на 5433 и `minio/minio` на
    9000 (`server /data`), для интеграционных тестов B1/B0 (I0 заменит на корневой).

## Выходы

Всё из списка выше; `make run` с `POSTGRES_DSN` на пустую БД и `S3_*` на MinIO стартует
(бакет создан), `/readyz` → `{"db":"ok","blob":"ok"}`, `curl :8080/v1/ping`
→ `{"ok":true}`, `:8080/v1/me` без токена → 401 с телом по SPEC §4.2, `:8081/metrics`
содержит `grpc_server_handled_total`, `:8081/docs/` открывается, `:8081/debug/pprof/`
отвечает, с `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317` и запущенным Jaeger спан `V1Ping`
виден (проверь `docker run --rm -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one:1.60`).

## Владение

`back/**` кроме `back/api/**` и `back/ARCHITECTURE.md`. Порты создаёшь ты, после B0 их
меняют только с отчётом.

## Параллелизация внутри

После шага 1 независимы: (a) apperr+config+logger, (b) telemetry+db+blob, (c) auth,
(d) server+bff-скелет (нужны a–c как интерфейсы — договорись о сигнатурах заранее или
делай d последним). Шаги 9–12 — после.

## Критерии приёмки

- [ ] `make generate` без диффа; `make lint` чисто; `make test` зелёный (apperr, config, auth, blob/memory, server smoke-тест с `httptest`); `make test-integration` зелёный (blob/s3 на MinIO).
- [ ] `GET /v1/avatars/x.jpg` без токена доходит до BFF (сейчас `Unimplemented` → 501, а не 401): `V1GetAvatar` публичный.
- [ ] `docker build -t hbc-back ./back` без SSH и приватных модулей.
- [ ] Проверки из «Выходы» выполнены (приложи curl-вывод в отчёт).
- [ ] Порты совпадают с ARCHITECTURE §2 (иначе — обновить документ и объяснить).

## Отчёт `tasks/reports/B0.md`

Структура пакетов, сигнатуры конструкторов platform (чтобы B1–B4 знали, как получить
pool/logger/tracer в тестах), команды проверки, отступления.
