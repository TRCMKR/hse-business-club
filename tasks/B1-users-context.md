# B1 — Контекст users

**Зависимости:** B0. **Блокирует:** B5.
**Читать:** `SPEC.md` §2.4 (валидация), §2.3, §3 (таблица users), §5; `back/ARCHITECTURE.md` §1–2;
`tasks/reports/B0.md`; `back/internal/users/users.go`; `back/internal/platform/blob`.

## Что сделать

1. `internal/users/domain`: нормализация и валидация `RegisterParams`/`UpdateProfileParams`
   по SPEC §2.4 (username → lower-case, telegram без `@`, skills trim/dedupe), ошибки через
   `apperr.Validation(field, msg)` с русскими сообщениями вида «Логин: от 3 до 32 символов,
   латиница, цифры и _». Чистый Go, табличные тесты.
1a. `internal/users/domain/avatar.go`: `NormalizeAvatar(data []byte) ([]byte, error)` по
   SPEC §2.4: размер ≤ 5 МБ; `image.DecodeConfig` (зарегистрированы `image/jpeg`,
   `image/png`, `golang.org/x/image/webp`) → формат из списка, 64×64 ≤ размеры ≤ 8000×8000
   — всё **до** полного декодирования; затем decode, центральный квадрат, resize до 512×512
   (`golang.org/x/image/draw.CatmullRom`), `jpeg.Encode` q85. Ошибки —
   `apperr.Validation("avatar", «Фото: …»)` с текстами из SPEC §2.3. EXIF-ориентация не
   обязательна (отметить в отчёте, если не сделана). Тесты: картинки генерируются в тесте
   (`image.NewRGBA` + encode), без бинарных фикстур: png/jpeg/webp-заголовок, слишком
   маленькая, не изображение, прямоугольник → квадрат 512×512.
2. `internal/users/postgres`: миграция `migrations/00001_users.sql` (goose; схема `users`,
   таблица по SPEC §3, без FK наружу), `Repository` на pgx: `Create` (unique violation
   `23505` → `apperr.New(KindConflict, "Логин уже занят")`), `GetByID`, `GetByUsername`
   (возвращает и hash — отдельная внутренняя структура), `GetMany` (`WHERE id = ANY($1)`),
   `Update`, `SetAvatarKey(ctx, id, key) (*User, error)`. Контекст `search_path` не
   используем — имена таблиц полностью квалифицированы (`users.users`).
3. `internal/users/service`: `New(repo, hasher, store blob.Store) users.Service`.
   `SetAvatar`: `NormalizeAvatar` → `store.Put("<uuid>.jpg", "image/jpeg")` → `SetAvatarKey`
   → старый объект `Delete` (ошибку только логировать). `RemoveAvatar`: ключ пуст → вернуть
   как есть; иначе `SetAvatarKey("")` → `Delete`. `OpenAvatar(key)`: key по regexp
   `^[0-9a-f-]{36}\.jpg$`, иначе `KindNotFound`; `store.Get` → `ErrNotFound` → `KindNotFound`
   «Фото не найдено»; читать тело в память (≤ ~100 КБ). `Register` хеширует bcrypt
   cost 10; `Authenticate` сравнивает хеш и при любой ошибке возвращает
   `apperr.New(KindUnauthenticated, "Неверный логин или пароль")` (без утечки, существует ли
   логин; при отсутствии пользователя всё равно выполнить bcrypt-сравнение с фиктивным
   хешем, чтобы не отличалось по времени).
4. Тесты: domain — unit; repository — интеграционные под `//go:build integration` c
   `TEST_POSTGRES_DSN` (поднять `postgres:17-alpine` через `docker run` или
   `back/docker-compose.test.yaml` из B0/I0; применить миграцию через `platform/db.Migrator`,
   `TRUNCATE users.users` между тестами); service — unit с фейковым репо и `blob/memory`:
   register/login happy path, дубль, неверный пароль, валидация; аватар: set → в сторе один
   объект и ключ в профиле, повторный set → старый удалён, remove идемпотентен, open по
   чужому/кривому ключу → NotFound.

## Выходы

Пакеты `domain`, `postgres`, `service` в `internal/users`; миграция; тесты.

## Владение

`back/internal/users/**` (кроме `users.go` — менять только с отчётом).

## Параллелизация внутри

domain ‖ postgres (после согласования внутренней структуры с hash) → service.

## Критерии приёмки

- [ ] `go test ./internal/users/...` и `go test -tags integration ./internal/users/...` зелёные; `make lint` чисто.
- [ ] Все ошибки наружу — `*apperr.Error` с сообщениями из SPEC; `password_hash` не выходит за пределы `postgres`/`service`.
- [ ] 5-мегабайтный PNG 4000×4000 нормализуется < 2 с; PNG 9000×9000 отклоняется без полного декодирования (проверить по памяти/времени в тесте).
- [ ] Миграция идемпотентна при повторном запуске.

## Отчёт `tasks/reports/B1.md`

Конструкторы (`postgres.New(pool)`, `service.New(...)`), как запускать интеграционные
тесты, отклонения от порта.
