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
make test-env-up          # postgres:17 на :5433 и MinIO на :9000 (../docker-compose.yaml + ../docker-compose.test.yaml)
cp .env.example .env      # .env читается только при APP_ENV=local
make migration-up         # goose up из ./migrations в POSTGRES_DSN (см. «Миграции»)
SEED_DEMO_DATA=true make run   # :8080 — API, :8081 — debug, gRPC — 127.0.0.1:8082
```

Приложение само миграции не применяет и считает схему готовой (в контейнере их применяет
entrypoint, см. «Миграции»). При старте: проверка/создание бакета, демо-данные (если
`SEED_DEMO_DATA=true` и таблица `users.users` пуста), затем сервер; `/readyz` = 200 —
«БД и хранилище доступны». Данные тестового окружения (корневой `docker-compose.test.yaml`, проект `hbc-test-env`) живут в tmpfs и пропадают при
`make test-env-down`.

Демо-аккаунты (SPEC §6), пароль у всех `demo1234`: `anna`, `boris`, `vera`, `dmitry`.
Сид идёт через сервисы контекстов (не SQL), поэтому UUID случайные; стабильные ключи —
`username` и `title`. Повторный старт сид пропускает (`demo data skipped` в логе);
одновременный старт нескольких экземпляров сериализуется advisory-lock'ом Postgres.
Сервисы коммитят каждый вызов отдельно, поэтому при ошибке на середине сид удаляет всё,
что успел создать (`demo data rolled back after an error`), и бэкенд не стартует; следующий
старт загрузит данные заново. Если не удался и откат (`demo data rollback failed`), база
останется с неполными данными — тогда пересоздать её: `make reset && make up` в корне.

Проверка:

```sh
curl localhost:8080/v1/ping      # {"ok":true}
curl localhost:8080/readyz       # {"db":"ok","blob":"ok"}
curl localhost:8080/v1/me        # 401 {"code":16,"message":"Требуется вход","details":[]}
curl localhost:8080/v1/auth/login -d '{"username":"anna","password":"demo1234"}'   # {accessToken, refreshToken, accessExpiresIn, user}
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
| `make test` | unit-тесты (`-race`), без инфраструктуры |
| `make test-integration` | поднимает `db` + `minio` из корневого compose с override `docker-compose.test.yaml` (ждёт healthcheck: `pg_isready`), гоняет тесты с тегом `integration` (репозитории, `db`, `blob/s3`), гасит контейнеры |
| `make test-e2e` | то же только с postgres, гоняет HTTP e2e-тесты `tests/e2e` (тег `e2e`) |
| `make test-all` | `test` + `test-integration` + `test-e2e` на одном поднятии инфраструктуры |
| `make run` / `make build` | запуск / сборка `./build/hbc` с версией из git |
| `make migration-create name=add_x` | новая SQL-миграция `migrations/<timestamp>_add_x.sql` |
| `make migration-up` / `make migration-down` | применить все / откатить последнюю миграцию в `POSTGRES_DSN` |
| `make migration-status` | статус миграций `POSTGRES_DSN` |
| `make test-env-up` / `make test-env-down` | поднять / удалить тестовую инфраструктуру |

Переменные целей: `TEST_ENV=external` — не поднимать compose, а использовать уже запущенные
`TEST_POSTGRES_DSN` / `TEST_S3_ENDPOINT`; `TEST_POSTGRES_PORT` / `TEST_MINIO_PORT` — порты тестового окружения на хосте (по умолчанию 5433 / 9000); `E2E_FLAGS=` — e2e без `-race` (≈ 6 с вместо ≈ 60 с:
bcrypt под race-детектором медленный). Цели с compose гасят контейнеры (`down -v`) и после
падения тестов, код возврата сохраняется; уже поднятую `make test-env-up` инфраструктуру
они тоже погасят.

## Миграции

Схема БД — только в [`migrations/`](migrations): goose SQL-файлы `YYYYMMDDHHMMSS_<name>.sql`
(как в других проектах автора), секции `-- +goose Up` / `-- +goose Down`, одна таблица версий
`public.goose_db_version`. Каждый контекст остаётся в своей схеме: миграция создаёт её
(`CREATE SCHEMA IF NOT EXISTS listings`) и объекты с полными именами (`listings.listings`),
FK между схемами нет. Порядок `users` → `listings` → `applications` задают номера.

```sh
make migration-create name=add_listing_views   # пустой файл с текущим UTC-timestamp
make migration-up                              # применить все новые
make migration-down                            # откатить последнюю
make migration-status                          # что применено, что нет
```

goose CLI запускается через `go run …/cmd/goose@<версия из go.mod>` (ставить глобально не
нужно, первый запуск скачивает модуль), только с драйвером Postgres. DSN — `POSTGRES_DSN`
(из `.env` или окружения), по умолчанию тестовое окружение
`postgres://hbc:hbc@localhost:5433/hbc`. Другая база: `POSTGRES_DSN=… make migration-status`.

**В контейнере** (compose, стенд): образ содержит бинарник `goose` (собран в builder-стадии
той же версией) и `/app/migrations` (другой путь — `MIGRATIONS_DIR`). Entrypoint
`docker-entrypoint.sh` ждёт БД до 30 с (`MIGRATIONS_WAIT_SECONDS`; SIGTERM в это время
завершает контейнер сразу, кодом 143), выполняет `goose up` и делает `exec hbc` — приложение
получает PID 1 и SIGTERM. Отказ сервера (неверный пароль, нет базы или роли) — выход сразу,
без ожидания. Повторный старт пишет `goose: no migrations to run`; ошибка миграции или
недоступная БД — контейнер завершается с кодом 1, `hbc` не стартует. Отдельного шага
миграций в `docker-compose.yaml` и при деплое не нужно.

**В тестах** integration- и e2e-тесты применяют ту же папку библиотекой goose
(`internal/platform/db/dbtest`: `Migrate`, `Up`, `Reset` — `DROP SCHEMA … CASCADE` и таблицы
версий).

## Тесты

- **unit** — домены, сервисы на фейках, BFF на фейках портов, сервер (gateway, ошибки, метрики).
- **integration** (`//go:build integration`) — репозитории контекстов на настоящем Postgres,
  миграции `migrations/` (up идемпотентен, down до нуля, снова up), `blob/s3` на MinIO.
  Каждый тест начинает с чистой базы (`dbtest.Migrate`: `Reset` + goose up, после теста —
  `Reset`). База и таблица версий общие, поэтому пакеты, которые ходят в Postgres, берут
  advisory-lock и выполняются по очереди (остальные — параллельно).
- **e2e** (`//go:build e2e`, `tests/e2e`) — приложение целиком in-process (`app.New`) на
  свободных портах `127.0.0.1:0`: реальный gRPC-хоп и grpc-gateway, Postgres из
  `TEST_POSTGRES_DSN` (в `TestMain` — `dbtest.Reset` + goose up), `blob/memory` вместо S3.
  Перед каждым тестом — `TRUNCATE` всех таблиц,
  у каждого экземпляра свой Prometheus-реестр. Сценарии: основной сценарий SPEC §1, правила
  §2.3 (повтор/отзыв/повторный отклик, своя/закрытая публикация, 403/404, идемпотентность),
  лента (фильтры, поиск, пагинация, валидация), регистрация/вход/профиль, фото, сид,
  ошибки разбора запроса и соответствие `hbc.swagger.json` маршрутам. Проверяется и форма
  JSON (camelCase, все поля, enum-строки, `null`, формат ошибки `{code, message, details}`).
  Без `TEST_POSTGRES_DSN` пакет пропускается.

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
| `ACCESS_TTL` | `15m` | срок access-JWT; > 0 |
| `REFRESH_TTL` | `720h` | срок refresh-токена (30 дней); > 0 |
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
`HEALTHCHECK` берёт порт из `HTTP_ADDR` (`:8080`, `0.0.0.0:9090`, …; по умолчанию 8080) и
ходит на `127.0.0.1:<порт>` — если API слушает только конкретный не-loopback адрес,
healthcheck надо переопределить в compose.

Запуск с внешними Postgres и MinIO (сеть тестового окружения `hbc-test-env`; весь стек целиком — `make up` в корне):

```sh
make test-env-up
docker run --rm --network hbc-test-env_default -p 8080:8080 -p 8081:8081 \
  -e POSTGRES_DSN='postgres://hbc:hbc@db:5432/hbc?sslmode=disable' \
  -e JWT_SECRET=change-me -e SEED_DEMO_DATA=true \
  -e S3_ENDPOINT=minio:9000 -e S3_ACCESS_KEY=minioadmin -e S3_SECRET_KEY=minioadmin hbc-back
```

## Соглашения API для клиентов

- Ошибки — `{"code": <gRPC code>, "message": "<текст для пользователя>", "details": []}`;
  HTTP: 400 (InvalidArgument/FailedPrecondition), 401, 403, 404, 409, 500 «Внутренняя ошибка».
- Ошибки разбора запроса (невалидный JSON, неверный тип поля, неизвестное имя enum в теле или
  query, не число в `page`/`limit`) — 400 «Некорректный формат запроса». Неизвестные *поля*
  тела игнорируются. Пустой параметр query (`?kind=`, `?status=`, `?page=`) равен
  отсутствующему; `null` у поля тела (в том числе enum) — «не задано».
- 401 — «Требуется вход» (нет/невалидный/истёкший access-токен), «Неверный логин или пароль»
  (login) или «Сессия истекла» (refresh: пустой, неизвестный, истёкший, отозванный или
  повторно использованный refresh-токен); заголовок `WWW-Authenticate: Bearer`.
- Токены: `register`/`login`/`refresh` отдают `{accessToken, refreshToken, accessExpiresIn,
  user}`. Refresh ротируется при каждом `POST /v1/auth/refresh`; повторное предъявление уже
  ротированного токена отзывает всю семью (цепочку этого входа). `POST /v1/auth/logout
  {refreshToken}` отзывает семью и всегда отвечает `{}`. Refresh и logout — без
  `Authorization`. В базе — только SHA-256 токена (`users.refresh_tokens`).
- POST-действия без полей (`accept`, `decline`, `withdraw`, `close`) принимают пустое тело или `{}`.

## Структура

```
cmd/hbc/                 composition root: config → logger → telemetry → pool → blob → app → run
migrations/              goose SQL всех схем (единственный источник схемы БД)
docker-entrypoint.sh     entrypoint образа: goose up → exec hbc
internal/app/            сборка: сервисы, адаптеры, сид, BFF, сервер
internal/adapters/       стыки контекстов: applications.ListingReader поверх listings.Service
internal/seed/           демо-данные SPEC §6 через сервисы контекстов
internal/bff/            реализация hbc.v1 (24 RPC): валидация, композиция, маппинг ошибок
internal/users|listings|applications/   контексты: порт, domain, service, postgres (своя схема)
internal/platform/       apperr, config, logger, telemetry, db, auth, blob, server
pkg/api/hbc/v1/          сгенерированный код + embed swagger
tests/e2e/               HTTP e2e-сценарии (тег e2e)
```
