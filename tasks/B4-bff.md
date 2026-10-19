# B4 — BFF: реализация hbc.v1 поверх портов

**Зависимости:** B0 (порты и скелет). Не ждёт B1–B3: работает на фейках. **Блокирует:** B5.
**Читать:** `SPEC.md` §2.3–2.4, §4 (целиком, особенно §4.2–4.3), §5; `back/ARCHITECTURE.md` §1–3;
`tasks/reports/B0.md`; `back/api/hbc/v1/hbc.proto`; `back/internal/{users,listings,applications}/*.go` (порты).

## Что сделать

1. `internal/bff/fakes_test.go`: in-memory фейки трёх портов (достаточные для тестов,
   с теми же ошибками `apperr`), фейковый `auth.Issuer`.
2. `internal/bff/validate.go`: проверка proto-запросов до вызова портов там, где это
   «форма» (uuid парсится, enum не UNSPECIFIED где обязателен, page/limit в диапазоне);
   содержательная валидация остаётся в контекстах — не дублируй правила длины.
3. `internal/bff/mapping.go`: `domain ↔ proto` для enum (`listings.Kind` ↔ `ListingKind` и
   т. д.), `time.Time`/`*time.Time` ↔ `timestamppb`, `users.User` → `User`, `listings.Listing`
   + author + myApplication + stats → `Listing`, `applications.Application` + listing + users
   → `Application` (с `ListingSummary`).
4. Методы (по файлам `auth.go`, `users.go`, `listings.go`, `applications.go`):
   - `V1Register`/`V1Login` → `Users` + `Issuer.Issue`; `V1GetMe`/`V1UpdateMe`/`V1GetUser`.
   - `V1UploadAvatar` → `Users.SetAvatar(me, data)` (пустой `data` → InvalidArgument «Фото:
     файл пустой»); `V1DeleteAvatar` → `Users.RemoveAvatar`; `V1GetAvatar` →
     `Users.OpenAvatar(file)` → `&httpbody.HttpBody{ContentType, Data}` +
     `grpc.SetHeader(ctx, metadata.Pairs("cache-control", "public, max-age=31536000, immutable"))`.
     В `mapping.go` `users.User.AvatarKey` → `avatar_url` = `"/v1/avatars/" + key` или `""`.
   - `V1ListListings`: `Listings.ListActive` → собрать `authorIDs` и `listingIDs` → **два
     батча** `Users.GetMany`, `Applications.GetMineForListings(me, ids)`; `stats` только для
     `is_mine` — `Applications.StatsForListings(myIDs)`. Никаких вызовов в цикле.
   - `V1GetListing`, `V1CreateListing`, `V1UpdateListing`, `V1CloseListing`,
     `V1ListMyListings` (со stats для всех — это свои).
   - `V1CreateApplication`, `V1ListListingApplications`, `V1AcceptApplication`,
     `V1DeclineApplication`, `V1WithdrawApplication`, `V1ListSentApplications`,
     `V1ListReceivedApplications` — для списков собрать `listingIDs` и `userIDs` батчами
     (`Listings.GetMany`, `Users.GetMany`) и построить `ListingSummary`/`applicant`.
   - Текущий пользователь — `auth.UserIDFromContext`; отсутствие → `Unauthenticated`
     (страховка, интерцептор уже отсёк).
5. `errors.go`: единая функция `toStatus(err) error` по ARCHITECTURE §3; `Internal` логирует
   оригинал с `trace_id`.
6. Тесты на каждый RPC с фейками: happy path, каждая ошибка из SPEC §2.3 → ожидаемый
   `codes.*` и текст; композиция (`author`, `myApplication`, `stats` null для чужих,
   `isMine`); `EmitUnpopulated` не твоя забота, но `closedAt == nil` → поле отсутствует в
   proto (nil).

## Выходы

`internal/bff/*` с полной реализацией и тестами. `cmd/hbc/main.go` не трогать (B5).

## Владение

`back/internal/bff/**`.

## Параллелизация внутри

После fakes+mapping+errors: субагент на `users.go`+`auth.go`, субагент на `listings.go`,
субагент на `applications.go` — независимы.

## Критерии приёмки

- [ ] Нет `codes.Unimplemented`; `go test ./internal/bff/...` покрывает все 22 RPC; `make lint` чисто.
- [ ] Тест через `httptest` + реальный gateway (как smoke в B0): `GET /v1/avatars/<key>` отдаёт байты с `Content-Type: image/jpeg` и `Cache-Control`, а не JSON; неизвестный — 404 JSON.
- [ ] В коде BFF нет импортов pgx и внутренних пакетов контекстов (`.../service`, `.../postgres`) — только корневые пакеты портов.
- [ ] Для `V1ListListings` с N публикаций число вызовов портов не зависит от N (проверить счётчиком в фейке).

## Отчёт `tasks/reports/B4.md`

Список RPC, вопросы к контракту, найденные неоднозначности SPEC и как решены.
