# Архитектура бэкенда

Модульный монолит, готовый к разделению на BFF и микросервисы. Один бинарник, один
Postgres, но внутри — три изолированных bounded-context'а и отдельный BFF-слой. Правила
ниже обязательны для всех задач `tasks/B*.md`.

## 1. Слои и границы

```
HTTP/JSON (grpc-gateway)  ──►  gRPC hbc.v1.HseBusinessClub (BFF)
                                      │  только через порты (Go-интерфейсы)
            ┌─────────────────────────┼─────────────────────────┐
            ▼                         ▼                         ▼
      users (context)          listings (context)       applications (context)
      schema "users"           schema "listings"        schema "applications"
            └────────────── platform: config, logger, telemetry, db, server, auth, apperr ───┘
```

| Слой | Пакет | Отвечает за | Не знает о |
|---|---|---|---|
| BFF | `internal/bff` | реализация `hbc.v1`: валидация proto-запросов, авторизация вызывающего, **композиция** ответов (автор, мой отклик, статистика), маппинг в proto, маппинг ошибок | SQL, pgx, внутренностях контекстов |
| Context | `internal/users`, `internal/listings`, `internal/applications` | свой домен, правила, хранение в **своей схеме** | других контекстах (кроме явных портов), proto, HTTP |
| Platform | `internal/platform/*` | сквозное: конфиг, логи, метрики, трейсы, БД, сервер, JWT, ошибки | домене |
| Adapters | `internal/adapters` | склейка контекстов между собой (реализация порта одного контекста поверх сервиса другого) | — |
| Composition root | `cmd/hbc/main.go` | сборка всего, запуск, graceful shutdown | — |

### Правила изоляции (проверяются на ревью)

1. Контекст импортирует только `platform/*` и свой корневой пакет. Импорт
   `internal/listings/...` из `internal/applications/...` запрещён; нужна информация —
   объяви порт в своём пакете (`applications.ListingReader`), реализуй в `internal/adapters`.
2. Каждый контекст — своя Postgres-схема (`users`, `listings`, `applications`). Миграции —
   goose SQL в одной папке `back/migrations/` (`YYYYMMDDHHMMSS_<name>.sql`, как в других
   проектах автора), одна таблица версий `public.goose_db_version`; каждая миграция создаёт
   объекты только своей схемы с полностью квалифицированными именами (`users.users`).
   Применяются **не кодом приложения**, а goose CLI: entrypoint контейнера выполняет
   `goose up` перед стартом бинарника; локально — `make migration-up`; в тестах —
   библиотека goose по той же папке (`platform/db/dbtest`). **Никаких FK и JOIN между
   схемами.** Ссылки между контекстами — `uuid` без ограничения.
3. Транзакция не пересекает границу контекста. Если нужна согласованность — денормализация
   на границе (например, `applications.listing_author_id` копируется при создании отклика,
   автор публикации не меняется) и проверка перед действием через порт.
4. Контекст не возвращает наружу типы pgx/sql. Только свои структуры из корневого пакета.
5. Авторизация действий внутри контекста (`actorID`) — обязанность контекста
   («редактировать может только автор»). Аутентификация (кто вызывает) — platform/auth + BFF.
6. Порты имеют «gRPC-форму»: `ctx` + параметры-структуры, батчи по id (`GetMany`),
   без ленивых загрузок и колбэков. Это делает порт переносимым на proto 1:1.
7. Контекст не знает о JWT, HTTP-кодах и proto.

## 2. Порты (контракт между параллельными задачами)

Файлы создаёт задача B0 **ровно в таком виде**; задачи B1–B3 реализуют, B4 потребляет.
Менять сигнатуры можно только с записью в отчёт и правкой этого документа.

### `internal/platform/apperr/apperr.go`

```go
package apperr

type Kind int

const (
    KindUnknown Kind = iota
    KindValidation     // InvalidArgument / 400
    KindUnauthenticated// 401
    KindForbidden      // 403
    KindNotFound       // 404
    KindConflict       // AlreadyExists / 409
    KindPrecondition   // FailedPrecondition / 400
)

type Error struct {
    Kind  Kind
    Msg   string // готов к показу пользователю, на русском
    Field string // для KindValidation, опционально
    Err   error  // причина, для логов
}

func (e *Error) Error() string
func (e *Error) Unwrap() error
func New(kind Kind, msg string) *Error
func Wrap(kind Kind, msg string, err error) *Error
func Validation(field, msg string) *Error
func KindOf(err error) Kind // KindUnknown, если не *Error
```

### `internal/users/users.go`

```go
package users

type User struct {
    ID          uuid.UUID
    Username    string
    DisplayName string
    Headline    string
    About       string
    Skills      []string
    Telegram    string
    AvatarKey   string // "<uuid>.jpg" в хранилище; "" — фото нет
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type RegisterParams struct{ Username, Password, DisplayName string }

// Готовый к отдаче файл фото.
type Avatar struct {
    ContentType string // всегда "image/jpeg"
    Data        []byte
}

type UpdateProfileParams struct {
    DisplayName, Headline, About string
    Skills                       []string
    Telegram                     string
}

type Service interface {
    // KindConflict «Логин уже занят», KindValidation.
    Register(ctx context.Context, p RegisterParams) (*User, error)
    // KindUnauthenticated «Неверный логин или пароль» (одно сообщение на оба случая).
    Authenticate(ctx context.Context, username, password string) (*User, error)
    // KindNotFound.
    Get(ctx context.Context, id uuid.UUID) (*User, error)
    // Отсутствующие id просто не попадают в map.
    GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*User, error)
    // KindNotFound, KindValidation.
    UpdateProfile(ctx context.Context, id uuid.UUID, p UpdateProfileParams) (*User, error)
    // image — исходный файл (JPEG/PNG/WebP, ≤ 5 МБ). Нормализует до 512×512 JPEG, кладёт
    // в хранилище под новым ключом, старый удаляет. KindNotFound, KindValidation «Фото: …».
    SetAvatar(ctx context.Context, id uuid.UUID, image []byte) (*User, error)
    // Идемпотентно. KindNotFound.
    RemoveAvatar(ctx context.Context, id uuid.UUID) (*User, error)
    // key — значение User.AvatarKey. KindNotFound «Фото не найдено» (и при плохом формате key).
    OpenAvatar(ctx context.Context, key string) (*Avatar, error)

    // Refresh-токены (SPEC §4.2, §5). Выдаёт новую семью токенов для входа.
    IssueRefresh(ctx context.Context, userID uuid.UUID) (*RefreshToken, error)
    // Ротация: token отзывается, выдаётся следующий в той же семье. KindUnauthenticated
    // «Сессия истекла» при неизвестном/истёкшем/отозванном; повторное использование уже
    // ротированного токена отзывает всю семью.
    RotateRefresh(ctx context.Context, token string) (userID uuid.UUID, next *RefreshToken, err error)
    // Идемпотентно: неизвестный токен — не ошибка.
    RevokeRefresh(ctx context.Context, token string) error
}

type RefreshToken struct {
    Token     string    // непрозрачный, base64url(32 случайных байта); наружу — только через BFF
    ExpiresAt time.Time
}
```

Хранилище файлов — порт платформы `internal/platform/blob`, реализацию получает сервис users:

```go
package blob

type Object struct {
    ContentType string
    Size        int64
    Body        io.ReadCloser
}

type Store interface {
    Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error
    Get(ctx context.Context, key string) (*Object, error) // ErrNotFound
    Delete(ctx context.Context, key string) error         // отсутствие — не ошибка
    Ping(ctx context.Context) error                       // для /readyz
}

var ErrNotFound = errors.New("blob: not found")
```

Реализации: `blob/s3` (minio-go/v7; в compose — образ `cgr.dev/chainguard/minio`, т. к. `minio/minio` больше не публикуется; bucket создаётся при старте, если нет; транспорт
обёрнут `otelhttp`; метрики `blob_operations_total{op,status}`,
`blob_operation_duration_seconds{op}`) и `blob/memory` (map, для unit/e2e-тестов).

### `internal/listings/listings.go`

```go
package listings

type Kind string   // "startup" | "vacancy"
type Status string // "active" | "closed"
type Stage string  // "" | "idea" | "prototype" | "mvp" | "launched"

const (
    KindStartup Kind = "startup"; KindVacancy Kind = "vacancy"
    StatusActive Status = "active"; StatusClosed Status = "closed"
    StageIdea Stage = "idea"; StagePrototype Stage = "prototype"; StageMVP Stage = "mvp"; StageLaunched Stage = "launched"
)

type Listing struct {
    ID           uuid.UUID
    AuthorID     uuid.UUID
    Kind         Kind
    Title        string
    Description  string
    LookingFor   string
    Tags         []string
    Stage        Stage  // только для startup, иначе ""
    Compensation string // только для vacancy, иначе ""
    Status       Status
    CreatedAt    time.Time
    UpdatedAt    time.Time
    ClosedAt     *time.Time
}

type CreateParams struct {
    AuthorID     uuid.UUID
    Kind         Kind
    Title, Description, LookingFor string
    Tags         []string
    Stage        Stage
    Compensation string
}

type UpdateParams struct {
    Title, Description, LookingFor string
    Tags         []string
    Stage        Stage
    Compensation string
}

type ListFilter struct {
    Kind  Kind   // "" = все
    Query string // "" = без поиска
    Page  int    // с 1
    Limit int    // 1..50
}

type Page struct {
    Items       []*Listing
    Total, Page, Limit int
}

type Service interface {
    Create(ctx context.Context, p CreateParams) (*Listing, error)                     // KindValidation
    Get(ctx context.Context, id uuid.UUID) (*Listing, error)                           // KindNotFound; closed тоже возвращается
    GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Listing, error)
    ListActive(ctx context.Context, f ListFilter) (*Page, error)                       // KindValidation для page/limit/query
    ListByAuthor(ctx context.Context, authorID uuid.UUID, status Status) ([]*Listing, error) // status "" = все
    Update(ctx context.Context, id, actorID uuid.UUID, p UpdateParams) (*Listing, error)   // KindNotFound, KindForbidden, KindPrecondition «Публикация закрыта»
    Close(ctx context.Context, id, actorID uuid.UUID) (*Listing, error)                // идемпотентно; KindNotFound, KindForbidden
}
```

### `internal/applications/applications.go`

```go
package applications

type Status string // "pending" | "accepted" | "declined" | "withdrawn"

const (
    StatusPending Status = "pending"; StatusAccepted Status = "accepted"
    StatusDeclined Status = "declined"; StatusWithdrawn Status = "withdrawn"
)

type Application struct {
    ID              uuid.UUID
    ListingID       uuid.UUID
    ListingAuthorID uuid.UUID // денормализовано при создании
    ApplicantID     uuid.UUID
    Message         string
    Status          Status
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DecidedAt       *time.Time
}

type Stats struct{ Pending, Accepted, Declined, Total int }

// Что контексту нужно знать о публикации. Реализуется в internal/adapters поверх listings.Service.
type ListingInfo struct {
    ID       uuid.UUID
    AuthorID uuid.UUID
    IsOpen   bool
}

type ListingReader interface {
    GetListing(ctx context.Context, id uuid.UUID) (ListingInfo, error) // KindNotFound
}

type Service interface {
    // KindNotFound (публикации), KindPrecondition «Нельзя откликнуться на свою публикацию» / «Публикация закрыта»,
    // KindConflict «Вы уже откликались на эту публикацию», KindValidation.
    Apply(ctx context.Context, listingID, applicantID uuid.UUID, message string) (*Application, error)
    // Виден автору публикации и откликнувшемуся, остальным KindNotFound.
    Get(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
    // actor = автор публикации. Откликнувшийся — KindForbidden «Недостаточно прав»; посторонний —
    // KindNotFound «Отклик не найден» (SPEC §2.3). Права проверяются до перехода.
    // pending→accepted; accepted→accepted = no-op; из declined/withdrawn — KindPrecondition
    // «Решение по отклику уже принято».
    Accept(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
    Decline(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
    // actor = откликнувшийся. Автор публикации — KindForbidden; посторонний — KindNotFound.
    // pending→withdrawn; withdrawn = no-op; иначе KindPrecondition.
    Withdraw(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
    // actor должен быть автором публикации, иначе KindForbidden. status "" = все. Сортировка: pending первыми, затем created_at DESC.
    ListByListing(ctx context.Context, listingID, actorID uuid.UUID, status Status) ([]*Application, error)
    // Отправленные. created_at DESC.
    ListByApplicant(ctx context.Context, applicantID uuid.UUID, status Status) ([]*Application, error)
    // Полученные: все отклики на публикации автора. pending первыми, затем created_at DESC.
    ListReceived(ctx context.Context, authorID uuid.UUID, status Status) ([]*Application, error)
    // Не отозванный отклик applicant'а на каждую из публикаций (для ленты/деталей). Нет — нет ключа.
    GetMineForListings(ctx context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*Application, error)
    // Статистика по публикациям (для автора). Нет откликов — нулевая Stats присутствует.
    StatsForListings(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]Stats, error)
}
```

### `internal/platform/auth` (используется BFF и сервером)

```go
type Issuer interface{ Issue(userID uuid.UUID) (token string, err error) } // access-JWT, TTL = ACCESS_TTL
type Verifier interface{ Verify(token string) (userID uuid.UUID, err error) }
func (j *JWT) TTL() time.Duration // для access_expires_in в ответе
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool)
func UnaryInterceptor(v Verifier, publicMethods map[string]bool) grpc.UnaryServerInterceptor
```

## 3. Поток запроса

`HTTP` → chi (`request-id`, `otelhttp`, http-метрики, recoverer, CORS) → grpc-gateway
(`JSONPb{EmitUnpopulated}`, header matcher `Authorization→authorization`) → gRPC-клиент на
`GRPC_ADDR` → gRPC-сервер с `ChainUnaryInterceptor(recovery, prometheus, logging, auth)` и
otel через `StatsHandler` (`otelgrpc.NewServerHandler`; интерцепторов в otelgrpc больше нет)
→ `bff` → порты → сервисы контекстов → репозитории (`otelpgx`).

Сервис — нативно gRPC: `bff.Implementation` реализует сгенерированный
`hbcv1.HseBusinessClubServer`, HTTP/JSON — только автогенерённый реверс-прокси
grpc-gateway. Сборка строго как в `blanc-back/cmd/app.go`:

1. `hbcv1.RegisterHseBusinessClubServer(grpcServer, impl)`; `grpcServer.Serve` на `GRPC_ADDR`.
2. `conn := grpc.NewClient(GRPC_ADDR, insecure, otelgrpc.NewClientHandler())`.
3. `mux := runtime.NewServeMux(opts...)`; `hbcv1.RegisterHseBusinessClubHandler(ctx, mux, conn)`.
4. `public.Mount("/", mux)` в chi; `http.Serve` на `HTTP_ADDR`.

Gateway обязан ходить через реальный gRPC-хоп. `RegisterXHandlerServer` (in-process, без
соединения) запрещён: интерцепторы не выполняются, метрики и трейсы gRPC пустые.

Опции `runtime.NewServeMux`: `WithMarshalerOption(MIMEWildcard, &HTTPBodyMarshaler{JSONPb{
EmitUnpopulated, DiscardUnknown}})` — чтобы `V1GetAvatar` (`google.api.HttpBody`) отдавался
байтами с `Content-Type` из сообщения; `WithIncomingHeaderMatcher` (`Authorization`);
`WithOutgoingHeaderMatcher` пропускает `cache-control`, `etag` как есть (без префикса
`Grpc-Metadata-`) — BFF ставит их через `grpc.SetHeader`. Лимит сообщения 16 МБ на
сервере (`MaxRecvMsgSize`) и у клиента gateway (`MaxCallRecvMsgSize`/`MaxCallSendMsgSize`)
— под base64-загрузку фото до 5 МБ.
Ручных HTTP-хендлеров для бизнес-методов нет; всё, что есть в HTTP API, есть в proto.

Ошибки разбора тела запроса в gateway (невалидный JSON, неизвестный enum) — `InvalidArgument`
с текстом «Некорректный формат запроса» (подмена в error handler gateway, SPEC §2.4).
`DiscardUnknown` игнорирует только неизвестные **поля**; имена enum сверяются с дескриптором
запроса до декодирования (иначе protojson молча превращал бы `"stage":"MVP"` в UNSPECIFIED).

Ошибки: сервис контекста возвращает `*apperr.Error`; BFF в одном месте
(`internal/bff/errors.go`) переводит `Kind` → `codes.*` и `Msg` → `status.Message`.
`KindUnknown` → `codes.Internal` c сообщением «Внутренняя ошибка», оригинал — в лог с
`trace_id`.

## 4. Наблюдаемость (platform)

| Аспект | Реализация |
|---|---|
| Конфиг | `internal/platform/config`: env → структуры по компонентам (`Server`, `DB`, `Auth`, `Blob`, `Telemetry`, `Seed`), `caarlos0/env/v11`, `.env` через godotenv только при `APP_ENV=local`. Валидация обязательных полей при старте, печать итогового конфига в лог без секретов. |
| Логи | zap JSON. `logger.FromContext(ctx)` добавляет `request_id`, `trace_id`, `span_id`, `user_id`. gRPC logging-интерцептор: метод, код, длительность, peer. Уровень из `LOG_LEVEL`. |
| Метрики | Prometheus на `DEBUG_ADDR/metrics`: gRPC server metrics (`go-grpc-middleware/v2/interceptors/prometheus`), HTTP gateway (`http_requests_total`, `http_request_duration_seconds` по route/method/status), `pgxpool` (acquired/idle/total conns, acquire duration), `build_info{version,commit}`, Go runtime. |
| Трейсы | OpenTelemetry: `otelgrpc` (сервер и клиент gateway), `otelhttp` (gateway), `otelpgx` (БД). Экспорт OTLP/gRPC в `OTEL_EXPORTER_OTLP_ENDPOINT`; если пусто — провайдер с no-op экспортером (span context всё равно создаётся для `trace_id` в логах). Ресурс: `service.name=hbc-back`, `service.version`. Sampler из `OTEL_TRACES_SAMPLER_ARG` (по умолчанию 1.0). |
| Health | `GET /healthz` — liveness (200 всегда после старта); `GET /readyz` — `pool.Ping` и `blob.Ping` с таймаутом 1s каждая (миграции к моменту старта уже применены entrypoint'ом), тело `{"db":"ok","blob":"ok"}` / 503 с причиной. Оба на публичном и debug-портах. |
| Файлы | `internal/platform/blob`: S3-совместимое хранилище (в compose — MinIO) для фото профиля; бакет создаётся при старте; трейсы через `otelhttp` на транспорте клиента; метрики `blob_operations_total`, `blob_operation_duration_seconds`. |
| pprof | `DEBUG_ADDR/debug/pprof/*`. |
| Swagger | `DEBUG_ADDR/docs/` (swagger-ui с CDN) + `/docs/swagger.json` из embed. |
| Shutdown | `signal.NotifyContext`; порядок: перестать принимать HTTP → gRPC `GracefulStop` с таймаутом → flush трейсов → закрыть pool. |

## 5. Переменные окружения

| Имя | По умолчанию | Назначение |
|---|---|---|
| `APP_ENV` | `local` | `local` подхватывает `.env`; в `production` `JWT_SECRET` обязателен и ≥ 32 байт, отладочные дефолты запрещены |
| `HTTP_ADDR` | `:8080` | публичный: `/v1/*`, `/healthz`, `/readyz` |
| `DEBUG_ADDR` | `:8081` | `/metrics`, `/debug/pprof`, `/docs/`, `/healthz`, `/readyz`; также проксирует `/v1/*` в публичный роутер, чтобы «Try it out» в Swagger UI работал без CORS. Наружу в проде не публикуется |
| `GRPC_ADDR` | `127.0.0.1:8082` | внутренний gRPC |
| `POSTGRES_DSN` | — | обязателен |
| `POSTGRES_MAX_CONNS` | `10` | |
| `JWT_SECRET` | — | обязателен |
| `ACCESS_TTL` | `15m` | срок access-JWT |
| `REFRESH_TTL` | `720h` | срок refresh-токена (30 дней) |
| `CORS_ORIGINS` | `http://localhost:3000` | через запятую |
| `SEED_DEMO_DATA` | `false` | SPEC §6 |
| `S3_ENDPOINT` | — | обязателен, `host:port` без схемы (`minio:9000`) |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | — | обязательны |
| `S3_BUCKET` | `hbc-avatars` | создаётся при старте, если нет |
| `S3_USE_SSL` | `false` | |
| `S3_REGION` | `us-east-1` | |
| `LOG_LEVEL` | `info` | |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | пусто = не экспортировать |
| `OTEL_SERVICE_NAME` | `hbc-back` | |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | |
| `SHUTDOWN_TIMEOUT` | `10s` | |

## 6. Структура

```
back/
├── api/hbc/v1/hbc.proto                 публичный контракт (BFF)
├── buf.yaml, buf.gen.yaml
├── cmd/hbc/main.go                      composition root
├── internal/
│   ├── platform/
│   │   ├── config/      ├── logger/     ├── telemetry/   (otel + prometheus)
│   │   ├── db/          (pgxpool, WithTx; dbtest/ — goose up из ../migrations для тестов)
│   │   ├── server/      (grpc + gateway + chi, health, docs, pprof, metrics)
│   │   ├── auth/        (bcrypt, jwt, interceptor, ctx)
│   │   ├── blob/        (Store порт; s3/ на minio-go, memory/ для тестов)
│   │   └── apperr/
│   ├── users/
│   │   ├── users.go               порт + типы (B0)
│   │   ├── domain/                валидация, инварианты, avatar.go (нормализация фото; чистый Go + x/image)
│   │   ├── service/               реализация Service
│   │   └── postgres/              repo (schema users)
│   ├── listings/      (то же)
│   ├── applications/  (то же)
│   ├── adapters/      listings→applications.ListingReader и т. п.
│   ├── bff/           реализация hbc.v1: service.go, auth.go, users.go, listings.go, applications.go, mapping.go, errors.go, validate.go
│   └── seed/
├── migrations/                          goose SQL (все схемы), применяются entrypoint'ом/`make migration-up`
├── pkg/api/hbc/v1/                      сгенерировано, коммитится
├── tests/e2e/                           интеграционные HTTP-тесты (build tag e2e)
├── Dockerfile, Makefile, .golangci.yaml, .env.example, .dockerignore, README.md
└── ARCHITECTURE.md
```

## 6a. Тесты: моки через mockery

Unit-тесты BFF и сервисов контекстов используют моки интерфейсов (портов, репозиториев,
`blob.Store`, `auth.Issuer/Verifier`, `applications.ListingReader`, `users/service.Hasher`),
сгенерированные **mockery v3** по `back/.mockery.yml` в пакеты `internal/<pkg>/mocks`
(`MockUsersService` и т. п., `with-expecter: true`, testify). Моки коммитятся; `make
generate-mocks` не должен давать диффа; `golangci-lint` исключает `mocks/`. Рукописные
in-memory фейки в unit-тестах не используются; поведение БД проверяют integration-тесты на
реальном Postgres (`dbtest`), сквозное — `tests/e2e`.

## 7. Путь к BFF + микросервисам

1. Для контекста написать `api/<ctx>/v1/<ctx>.proto`, зеркалящий его порт (структуры
   параметров уже «proto-образные»). Сгенерировать.
2. В `cmd/<ctx>/main.go` поднять gRPC-сервер контекста на том же `platform/server`
   (без gateway). Схема `<ctx>` уезжает в отдельную БД — её миграции выделяются из
   `migrations/` по префиксу схемы. Бакет фото
   уезжает вместе с `users` (к нему обращается только этот контекст); отдача файлов
   остаётся в BFF через `OpenAvatar` — или заменяется на публичный CDN-URL в `avatar_url`.
3. В `internal/adapters` добавить gRPC-клиентскую реализацию порта; в composition root
   BFF заменить in-process реализацию на клиент. Код BFF не меняется.
4. `hbc.v1` остаётся BFF для веба; для Telegram mini-app можно поднять второй BFF с другим
   proto, переиспользуя те же порты.
5. Межсервисная согласованность (например, счётчики) — через события; сейчас не нужно,
   точка расширения — `applications` публикует `ApplicationDecided` (не реализуем).
