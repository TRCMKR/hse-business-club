# SPEC — модуль «Стартапы и вакансии» Бизнес-клуба

Единый источник истины по продукту, домену, модели данных и API. Задания для агентов в `tasks/` ссылаются сюда и не повторяют этот текст. Если при реализации нужно отступить от спеки — сначала поправить
спеку (и `back/api/hbc/v1/hbc.proto`), затем код, и явно сообщить об изменении в отчёте.

Исходное ТЗ: https://docs.google.com/document/d/19PrrbstXrUcBN78HO_rAlZRkWVS7aEvRo0-gxDmczQk

---

## 1. Что строим

Прототип модуля для будущего Telegram mini-app Бизнес-клуба НИУ ВШЭ. Участники клуба:

- публикуют **стартапы** («ищу сооснователя/команду») и **вакансии** («нужен разработчик в проект»);
- смотрят **ленту** предложений, открывают подробности, **откликаются**;
- автор видит отклики и **принимает решение** (принять / отклонить);
- откликнувшийся видит результат в **личном кабинете**.

Один пользователь может быть и автором, и откликнувшимся. Основной сценарий, который
должен работать от начала до конца и покрыт тестами:

> Анна публикует стартап → Борис находит его в ленте и откликается → Анна видит отклик и
> принимает → Борис видит «принят» в кабинете.

### 1.1. Оценивают (из ТЗ)

Работоспособность сценария, обоснованность решений, качество кода, удобство интерфейса
(телефон + компьютер), безопасность, осознанную работу с ИИ. Нужны: Docker Compose,
тесты ключевых сценариев, учёт повторных действий, ошибок и пустых состояний.

### 1.2. Отклонения от ТЗ и допущения

| Тема | Решение | Обоснование для README |
|---|---|---|
| ТЗ: «Используйте Python» | **Go** (бэк) + PostgreSQL + Nuxt/Vue/TypeScript (фронт) | Основной стек автора; архитектура (слои, API-контракт, миграции) переносима. Указать явно в README как осознанное отклонение. |
| Telegram | Не интегрируемся с Telegram SDK/initData | Прототип — обычный веб; вход по логину/паролю. UI делаем «как мобильное приложение» (нижняя навигация), чтобы потом встроиться в mini-app. |
| Регистрация | Есть (логин + пароль + имя) | Дёшево, позволяет проверяющему завести своего пользователя. Основная демонстрация — на сид-аккаунтах. |
| Удаление публикаций | Нет, только закрытие | Сохраняем историю откликов; закрытая публикация остаётся видна автору и откликнувшимся. |
| Принятие отклика | Не закрывает публикацию и не отклоняет остальных | В стартап можно взять несколько людей; автор закрывает публикацию сам. |
| Закрытие публикации | Не меняет статусы откликов | Автор может доразобрать отклики после закрытия; UI показывает «публикация закрыта». |
| Связь вакансия → стартап | Нет (вне MVP) | Обе сущности — самостоятельные «предложения» в одной ленте. Возможное развитие: `startup_id` у вакансии. |
| Поиск | `ILIKE` по title/description | Достаточно для прототипа; развитие — `pg_trgm`/FTS. |
| Пагинация | page/limit (offset) | Простая и понятная для ленты прототипа. |

---

## 2. Домен

### 2.1. Сущности

**User** — участник клуба.
`id`, `username` (логин, уникальный, lower-case), `password_hash`, `display_name`, `headline`
(кто я, одной строкой), `about`, `skills[]`, `telegram`, `avatar_key` (имя файла фото в
объектном хранилище, пусто — фото нет), `created_at`, `updated_at`.

Аватар: если фото загружено — оно (квадрат 512×512, JPEG, хранится в S3-совместимом
хранилище, в compose — MinIO); если нет — детерминированный сгенерированный аватар
(своя библиотека `@fe/blob-avatar`, seed = `username`: спокойная абстрактная композиция из
пастельных фигур, без лиц), рисуется на фронте, на бэке ничего не хранится.

**Listing** — публикация (предложение). Один тип таблицы, два вида (`kind`):
`startup` и `vacancy`.
Общие поля: `title`, `description`, `looking_for` (кого ищет автор, свободный текст), `tags[]`
(роли/навыки — теги карточки), `status`.
Только для startup: `stage` (idea / prototype / mvp / launched).
Только для vacancy: `compensation` (условия: доля, оплата, формат — короткий текст).

**Application** — отклик участника на публикацию. `listing_id`, `applicant_id`, `message`
(сопроводительный текст), `status`, `decided_at`.
В UI и README называется «отклик»; в коде и API — `application`, чтобы не путать с
`Response`-сообщениями API.

### 2.2. Жизненные циклы

**Listing**: `active` → `closed`. Переход только автором, финальный. Закрытая публикация:

- не показывается в ленте;
- доступна по прямой ссылке всем залогиненным (с плашкой «закрыта»);
- не принимает новые отклики и не редактируется.

**Application**: `pending` → `accepted` | `declined` (решение автора публикации, финальное),
`pending` → `withdrawn` (откликнувшийся отозвал, финальное).

### 2.3. Бизнес-правила

| Правило | Ошибка API |
|---|---|
| Все методы, кроме ping/register/login, требуют валидный JWT | `Unauthenticated` (401) |
| Нельзя откликнуться на свою публикацию | `FailedPrecondition` (400) «Нельзя откликнуться на свою публикацию» |
| Нельзя откликнуться на закрытую публикацию | `FailedPrecondition` (400) «Публикация закрыта» |
| Нельзя откликнуться повторно, пока есть отклик в статусе pending/accepted/declined | `AlreadyExists` (409) «Вы уже откликались на эту публикацию» |
| После `withdrawn` можно откликнуться заново (создаётся новая запись) | — |
| Редактировать/закрывать публикацию может только автор | `PermissionDenied` (403) |
| Редактировать можно только активную публикацию; `kind` не меняется | `FailedPrecondition` (400) |
| Смотреть отклики на публикацию может только автор | `PermissionDenied` (403) |
| accept/decline — только автор публикации; withdraw — только откликнувшийся | `PermissionDenied` (403) |
| accept/decline/withdraw из финального статуса в другой финальный | `FailedPrecondition` (400) «Решение по отклику уже принято» |
| Повтор того же перехода (accept→accept, close→close, withdraw→withdraw) | 200, возвращается текущее состояние (идемпотентность) |
| Чужой/несуществующий id | `NotFound` (404) — не раскрываем существование чужих объектов там, где это уместно: для откликов отдаём 404 вместо 403, если пользователь ни автор публикации, ни откликнувшийся |
| Регистрация с занятым username | `AlreadyExists` (409) «Логин уже занят» |
| Загрузка фото: не изображение / не JPEG, PNG, WebP | `InvalidArgument` (400) «Фото: поддерживаются JPEG, PNG и WebP» |
| Загрузка фото: больше 5 МБ или больше 8000×8000 px | `InvalidArgument` (400) «Фото: не больше 5 МБ» / «Фото: не больше 8000×8000» |
| Загрузка фото: меньше 64×64 px | `InvalidArgument` (400) «Фото: минимум 64×64» |
| Удаление фото, которого нет | 200, текущий профиль (идемпотентно) |
| `GET /v1/avatars/{file}` с неизвестным именем | `NotFound` (404) «Фото не найдено» |
| Неверный логин/пароль | `Unauthenticated` (401) «Неверный логин или пароль» (одно сообщение на оба случая) |

### 2.4. Валидация (сервер — источник истины, фронт дублирует для UX)

| Поле | Правило |
|---|---|
| `username` | 3–32 символа, `^[a-z0-9_]+$`, приводится к lower-case |
| `password` | 8–72 символа |
| `display_name` | 1–80 |
| `headline` | ≤ 120 |
| `about` | ≤ 2000 |
| `skills` | ≤ 20 элементов, каждый 1–40 (после trim); пустые отбрасываются; дубли (сравнение без учёта регистра) → ошибка «Навыки: без повторов» |
| `telegram` | ≤ 64, ведущий `@` отбрасывается |
| `avatar` (`data`) | JPEG, PNG или WebP (определяется по содержимому, не по заголовку); ≤ 5 МБ исходник; 64×64 … 8000×8000 px (проверяется по заголовку изображения до полного декодирования). Сервер центрирует квадрат, уменьшает до 512×512, кодирует JPEG q85; оригинал не хранится |
| `title` | 3–120 |
| `description` | 10–5000 |
| `looking_for` | 1–1000 |
| `tags` | ≤ 10 элементов, каждый 1–40 (после trim); пустые отбрасываются; дубли (без учёта регистра) → ошибка «Теги: без повторов» |
| `stage` | для startup **обязательна** (не `UNSPECIFIED`) → иначе «Стадия: укажите стадию»; для vacancy должно быть `UNSPECIFIED` → иначе «Стадия: только для стартапа» |
| `compensation` | ≤ 120; только для vacancy; для startup должно быть пусто |
| `kind` | `STARTUP` или `VACANCY`, не `UNSPECIFIED` |
| `message` (отклик) | 1–2000 |
| `query` | ≤ 100 |
| `page` | ≥ 1, по умолчанию 1 |
| `limit` | 1–50, по умолчанию 20 |

Ошибки валидации — `InvalidArgument` (400) с сообщением на русском, понятным пользователю,
с указанием поля: «Название: от 3 до 120 символов». Подписи полей в сообщениях (единые для
бэка, mock и фронта):

| Поле | Подпись |
|---|---|
| `username` | Логин |
| `password` | Пароль |
| `display_name` | Имя |
| `headline` | Кратко о себе |
| `about` | О себе |
| `skills` | Навыки |
| `telegram` | Telegram |
| `avatar` | Фото |
| `kind` | Вид публикации |
| `title` | Название |
| `description` | Описание |
| `looking_for` | Кого ищете |
| `tags` | Теги |
| `stage` | Стадия |
| `compensation` | Условия |
| `message` | Сообщение |
| `query` | Поиск |
| `page` / `limit` | Страница / Размер страницы |

Ошибка разбора тела запроса (невалидный JSON, неизвестный enum) — `InvalidArgument` (400)
с текстом «Некорректный формат запроса» (gateway подменяет английский текст protojson).
Списки «мои публикации», отклики на публикацию, отправленные/полученные отклики — без
пагинации (объёмы прототипа малы; развитие — page/limit как в ленте).

---

## 3. Модель данных (PostgreSQL)

Миграции — goose SQL, применяются автоматически при старте бэкенда.

```sql
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL,
    headline      text NOT NULL DEFAULT '',
    about         text NOT NULL DEFAULT '',
    skills        text[] NOT NULL DEFAULT '{}',
    telegram      text NOT NULL DEFAULT '',
    avatar_key    text NOT NULL DEFAULT '',   -- '<uuid>.jpg' в бакете, '' = нет фото
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE listings (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    uuid NOT NULL REFERENCES users(id),
    kind         text NOT NULL CHECK (kind IN ('startup', 'vacancy')),
    title        text NOT NULL,
    description  text NOT NULL,
    looking_for  text NOT NULL,
    tags         text[] NOT NULL DEFAULT '{}',
    stage        text CHECK (stage IN ('idea', 'prototype', 'mvp', 'launched')),
    compensation text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    closed_at    timestamptz
);
CREATE INDEX listings_feed_idx   ON listings (status, created_at DESC);
CREATE INDEX listings_author_idx ON listings (author_id, created_at DESC);

CREATE TABLE applications (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id   uuid NOT NULL REFERENCES listings(id),
    applicant_id uuid NOT NULL REFERENCES users(id),
    message      text NOT NULL,
    status       text NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'accepted', 'declined', 'withdrawn')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz
);
-- один «живой» отклик на пару (публикация, участник); после withdraw можно откликнуться снова
CREATE UNIQUE INDEX applications_active_uniq
    ON applications (listing_id, applicant_id) WHERE status <> 'withdrawn';
CREATE INDEX applications_applicant_idx ON applications (applicant_id, created_at DESC);
CREATE INDEX applications_listing_idx   ON applications (listing_id, status, created_at DESC);
```

Статусы и виды хранятся как `text` с `CHECK` (проще мигрировать, чем native enum).
Маппинг на proto-enum: `startup ↔ LISTING_KIND_STARTUP` и т. д.

---

## 4. API

Контракт: **`back/api/hbc/v1/hbc.proto`** (gRPC + grpc-gateway, HTTP/JSON под `/v1/...`). Это
единственный источник; здесь — только сводка и соглашения. Генерируемый
`back/pkg/api/hbc/v1/hbc.swagger.json` коммитится и используется фронтом для генерации TS-клиента.

### 4.1. Сводка эндпоинтов

| Метод и путь | RPC | Кто | Назначение |
|---|---|---|---|
| `GET /v1/ping` | V1Ping | все | health |
| `POST /v1/auth/register` | V1Register | все | регистрация → `{token, user}` |
| `POST /v1/auth/login` | V1Login | все | вход → `{token, user}` |
| `GET /v1/me` | V1GetMe | auth | текущий пользователь |
| `PUT /v1/me` | V1UpdateMe | auth | обновить профиль (полная замена редактируемых полей) |
| `GET /v1/users/{id}` | V1GetUser | auth | публичный профиль |
| `POST /v1/me/avatar` | V1UploadAvatar | auth | загрузить/заменить фото: `{data: base64}` → `User` |
| `DELETE /v1/me/avatar` | V1DeleteAvatar | auth | удалить фото (идемпотентно) → `User` |
| `GET /v1/avatars/{file}` | V1GetAvatar | все | файл фото `image/jpeg`, `Cache-Control: public, max-age=31536000, immutable` |
| `GET /v1/listings?kind&query&page&limit` | V1ListListings | auth | лента (только active), новые первыми |
| `POST /v1/listings` | V1CreateListing | auth | создать публикацию |
| `GET /v1/listings/{id}` | V1GetListing | auth | подробности (active и closed) |
| `PUT /v1/listings/{id}` | V1UpdateListing | автор | редактировать (только active) |
| `POST /v1/listings/{id}/close` | V1CloseListing | автор | закрыть (идемпотентно) |
| `GET /v1/me/listings?status` | V1ListMyListings | auth | мои публикации + `stats` |
| `POST /v1/listings/{listing_id}/applications` | V1CreateApplication | auth | откликнуться |
| `GET /v1/listings/{listing_id}/applications?status` | V1ListListingApplications | автор | отклики на публикацию |
| `POST /v1/applications/{id}/accept` | V1AcceptApplication | автор | принять |
| `POST /v1/applications/{id}/decline` | V1DeclineApplication | автор | отклонить |
| `POST /v1/applications/{id}/withdraw` | V1WithdrawApplication | откликнувшийся | отозвать |
| `GET /v1/me/applications/sent?status` | V1ListSentApplications | auth | мои отправленные отклики |
| `GET /v1/me/applications/received?status` | V1ListReceivedApplications | auth | отклики на все мои публикации |

Сортировки: лента и «мои публикации» — `created_at DESC`; отклики на публикацию и
«полученные» — сначала `pending`, затем `created_at DESC`; «отправленные» — `created_at DESC`.

### 4.2. Соглашения JSON

- `ListingStats`: `total = pending + accepted + declined`; отозванные (`withdrawn`) отклики
  в статистику не входят.

- Сериализация protojson: поля в `lowerCamelCase` (`displayName`, `lookingFor`, `createdAt`),
  **все поля всегда присутствуют** (`EmitUnpopulated: true`): пустая строка, `[]`, `0`, `null`
  для вложенных сообщений.
- Enum — строковые имена: `"LISTING_KIND_STARTUP"`, `"APPLICATION_STATUS_PENDING"`.
- Timestamps — RFC 3339 в UTC: `"2026-10-19T18:00:00Z"`. Не заданные — `null`.
- Числа — только `int32` в контракте (у `int64` в protojson строковое представление).
- Аутентификация: `Authorization: Bearer <jwt>`. JWT HS256, срок 24 часа, claim `sub` = user id.
- `avatarUrl` — путь относительно корня API (`/v1/avatars/<uuid>.jpg`) или `""`. Фронт
  подставляет свой префикс прокси (`/api` + `avatarUrl`). Имя файла меняется при каждой
  загрузке, поэтому ответ `GET /v1/avatars/{file}` кэшируется как immutable.
- `V1GetAvatar` — единственный не-JSON ответ (`google.api.HttpBody`): тело — байты
  изображения, `Content-Type: image/jpeg`. В swagger он описан как объект, TS-клиент для
  него не используется — фронт ставит путь в `<img src>`.
- Загрузка фото — JSON с base64 (`{"data": "..."}`), не multipart: всё API идёт через
  grpc-gateway из proto. 5 МБ исходника → ~6,8 МБ JSON; лимит сообщения gRPC поднят до 16 МБ.
- Ошибки — стандартный формат grpc-gateway:

```json
{ "code": 3, "message": "Название: от 3 до 120 символов", "details": [] }
```

  Маппинг gRPC → HTTP: `InvalidArgument`→400, `FailedPrecondition`→400, `Unauthenticated`→401,
  `PermissionDenied`→403, `NotFound`→404, `AlreadyExists`→409, прочее→500 (`message`
  «Внутренняя ошибка», детали только в логах). `message` всегда готов к показу пользователю.

### 4.3. Примеры

`POST /v1/auth/login`

```json
{ "username": "anna", "password": "demo1234" }
```

```json
{
  "token": "eyJ...",
  "user": {
    "id": "11111111-1111-4111-8111-111111111111",
    "username": "anna",
    "displayName": "Анна Соколова",
    "headline": "Фаундер FinTrack, ex-продакт в финтехе",
    "about": "...",
    "skills": ["Продукт", "Финтех", "Питчинг"],
    "telegram": "anna_sokolova",
    "avatarUrl": "",
    "createdAt": "2026-10-01T10:00:00Z"
  }
}
```

`GET /v1/listings?kind=LISTING_KIND_STARTUP&page=1&limit=20` (от имени `boris`)

```json
{
  "items": [
    {
      "id": "a1111111-1111-4111-8111-111111111111",
      "kind": "LISTING_KIND_STARTUP",
      "title": "FinTrack — учёт личных финансов для студентов",
      "description": "...",
      "lookingFor": "Технический сооснователь (Go/Python), готовый взять на себя бэкенд и инфраструктуру",
      "tags": ["Go", "Backend", "CTO"],
      "stage": "STARTUP_STAGE_MVP",
      "compensation": "",
      "status": "LISTING_STATUS_ACTIVE",
      "author": { "id": "1111...", "username": "anna", "displayName": "Анна Соколова", "headline": "...", "about": "", "skills": [], "telegram": "", "createdAt": "...", "avatarUrl": "" },
      "createdAt": "2026-10-10T12:00:00Z",
      "updatedAt": "2026-10-10T12:00:00Z",
      "closedAt": null,
      "isMine": false,
      "myApplication": { "id": "...", "status": "APPLICATION_STATUS_PENDING", "message": "...", "listing": { "...": "..." }, "applicant": { "...": "..." }, "createdAt": "...", "updatedAt": "...", "decidedAt": null },
      "stats": null
    }
  ],
  "total": 3,
  "page": 1,
  "limit": 20
}
```

`POST /v1/listings/a111.../applications` повторно → `409`:

```json
{ "code": 6, "message": "Вы уже откликались на эту публикацию", "details": [] }
```

---

## 5. Безопасность

- Пароли — bcrypt (cost 10). В ответах API нет хешей и e-mail.
- JWT подписывается секретом из env `JWT_SECRET`; в compose задан демо-секрет, в README —
  как заменить.
- Браузер **не хранит JWT в JS**: фронт (Nitro-сервер Nuxt) кладёт токен в httpOnly-cookie и
  сам добавляет `Authorization` при проксировании запросов к бэкенду. В compose порты API и
  debug опубликованы только для удобства проверки (Swagger, smoke); в проде наружу смотрит
  только фронт.
- Все проверки прав — на сервере (п. 2.3), фронт лишь прячет недоступные действия.
- SQL — только параметризованные запросы (pgx). Лимиты длины на все строковые поля.
- CORS на бэкенде — список разрешённых origin из env (`CORS_ORIGINS`), по умолчанию
  `http://localhost:3000`.
- Единое сообщение при неверном логине/пароле; 404 вместо 403 для чужих откликов.
- Фото: тип определяется по содержимому, размеры проверяются до декодирования (защита от
  decompression bomb), файл перекодируется — исходник с метаданными (EXIF, геолокация) не
  хранится. Хранилище наружу не публикуется, отдача только через API по непредсказуемому
  имени; креды S3 — в env.

---

## 6. Демо-данные (сид)

Применяются при старте бэкенда, если `SEED_DEMO_DATA=true` и таблица `users` пуста
(идемпотентно). Сид идёт через сервисы контекстов (не сырым SQL), поэтому UUID генерируются;
стабильные идентификаторы для тестов и документации — `username` и `title`.

Пароль у всех: **`demo1234`**. Фото у демо-аккаунтов нет: у всех сгенерированные аватары
(так видно fallback); загрузка фото проверяется в e2e и smoke.

| username | display_name | headline | skills | telegram |
|---|---|---|---|---|
| `anna` | Анна Соколова | Фаундер FinTrack, ex-продакт в финтехе | Продукт, Финтех, Питчинг | anna_sokolova |
| `boris` | Борис Климов | Backend-разработчик (Go, Python), ищу проект | Go, Python, PostgreSQL | boris_dev |
| `vera` | Вера Лисицына | Продуктовый дизайнер, UX-исследования | Figma, UX, Прототипирование | vera_design |
| `dmitry` | Дмитрий Орлов | Маркетолог, growth и комьюнити | Маркетинг, Growth, SMM | dmitry_growth |

Публикации (все `active`, кроме №6):

| № | автор | kind | title | stage / compensation | looking_for | tags |
|---|---|---|---|---|---|---|
| 1 | anna | startup | FinTrack — учёт личных финансов для студентов | MVP | Технический сооснователь (Go/Python), готовый взять на себя бэкенд и инфраструктуру | Go, Backend, CTO |
| 2 | anna | vacancy | Backend-разработчик в FinTrack | Доля 3% + стипендия 40 000 ₽ | Разработчик на Go или Python на 15–20 часов в неделю | Go, PostgreSQL, API |
| 3 | boris | startup | CampusEats — доставка еды по кампусу Покровки | idea | Сооснователь по маркетингу и операционке | Маркетинг, Операции, Сооснователь |
| 4 | vera | vacancy | UX-исследователь на EdTech-проект | Pro bono, кейс в портфолио | Студент с интересом к UX-исследованиям на 2 месяца | UX, Исследования, EdTech |
| 5 | dmitry | startup | Volunteer Hub — платформа волонтёрских проектов ВШЭ | prototype | Frontend-разработчик и дизайнер | Frontend, Vue, Дизайн |
| 6 | dmitry | vacancy (**closed**) | Frontend-разработчик в Volunteer Hub | Без оплаты, доля | Vue/React-разработчик | Vue, Frontend |

Отклики:

| откликнувшийся | публикация | status | message |
|---|---|---|---|
| boris | 1 | pending | Привет! Делаю бэкенды на Go 3 года, финтех интересен. Готов обсудить роль CTO. |
| vera | 1 | accepted | Могу помочь с продуктом и UX на старте, есть опыт в финтех-дизайне. |
| dmitry | 2 | declined | Не разработчик, но могу помочь с ростом — вдруг актуально. |
| anna | 3 | pending | Интересная идея, могу помочь с продуктом и питчем инвесторам. |
| boris | 6 | accepted | Готов взять фронт на Vue. |
| vera | 5 | withdrawn | Хотела помочь с дизайном, но пока нет времени. |

Такой набор показывает все состояния: у Анны есть публикации со статистикой, входящие
(pending от Бориса), её собственный исходящий отклик; у Бориса — pending и accepted; у Веры —
accepted и withdrawn (может откликнуться повторно на №5); у Дмитрия — declined и закрытая
публикация с принятым откликом.

---

## 7. Экраны фронтенда (сводка; подробности в tasks/F*)

| Путь | Экран |
|---|---|
| `/login` | Вход: форма + кнопки «Войти как …» для демо-аккаунтов; ссылка на регистрацию |
| `/register` | Регистрация |
| `/` | Лента: переключатель Все / Стартапы / Вакансии, поиск, карточки, «Показать ещё» |
| `/listings/new` | Создание публикации |
| `/listings/:id` | Подробности + кнопка «Откликнуться» (лист/диалог с сообщением); для автора — статистика и ссылка на отклики |
| `/listings/:id/edit` | Редактирование (автор, только active) |
| `/listings/:id/applications` | Отклики на публикацию (автор): принять / отклонить |
| `/me` | Профиль: просмотр и редактирование; фото: загрузить/заменить/удалить (превью до загрузки, проверка типа и размера на клиенте) |
| `/me/listings` | Мои публикации (active/closed, статистика, закрыть) |
| `/me/applications` | Мои отклики: вкладки «Отправленные» (статус, отозвать) и «Полученные» (принять/отклонить) |
| `/users/:id` | Публичный профиль участника |

Навигация: на телефоне — нижняя панель (Лента · Мои публикации · Отклики · Профиль) и
кнопка «Создать»; на компьютере — верхняя панель с теми же пунктами и кнопкой «Создать
публикацию», контент в контейнере ≤ 1040px.

Аватары везде (карточки публикаций, отклики, профили, header): один компонент `avatar`:
`avatarUrl` непустой → `<img>`; иначе — `BlobAvatar` из собственной библиотеки
`front/lib/blob-avatar` (`@fe/blob-avatar`, TypeScript + Vue, без зависимостей; задача F7)
с `seed = username`: детерминированная абстрактная SVG-композиция в серьёзном стиле —
пастельная палитра, 2–3 геометрические/органические фигуры, без лиц и «персонажей»
(силуэт, палитра, компоновка выводятся из хеша строки). Один и тот же участник выглядит
одинаково на всех экранах, в SSR и в mock-режиме.

---

## 8. Репозиторий и процесс

```
hse-business-club/
├── README.md               — итоговый README для проверяющих (I2)
├── SPEC.md                 — этот файл (продукт, домен, данные, API)
├── PLAN.md                 — граф задач, волны, как запускать агентов
├── CLAUDE.md               — краткие указания агентам
├── tasks/                  — задания B0–B5 (бэк), F0–F6 (фронт), I0–I2 (интеграция)
│   └── reports/            — отчёты агентов по задачам
├── docker-compose.yaml     — db + minio + back + front (+ профиль observability) (I0)
├── back/                   — Go-бэкенд: модульный монолит (см. back/ARCHITECTURE.md)
│   └── api/hbc/v1/hbc.proto — контракт API (готов)
└── front/                  — Nuxt-фронтенд
```

Порядок и параллелизм — в `PLAN.md`. Архитектура бэкенда (контексты, порты,
наблюдаемость, путь к микросервисам) — в `back/ARCHITECTURE.md`.
