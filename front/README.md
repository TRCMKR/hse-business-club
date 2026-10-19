# hbc-front

Фронтенд модуля «Стартапы и вакансии» Бизнес-клуба НИУ ВШЭ: Nuxt 4 (SSR), Vue 3, TSX,
CSS Modules, Pinia, vee-validate + yup, vue-sonner. Продукт и правила — `../SPEC.md`,
контракт API — `../back/api/hbc/v1/hbc.proto`.

## Запуск

```bash
npm i
npm run dev                      # http://localhost:3000, бэкенд на http://localhost:8080
NUXT_MOCK_API=true npm run dev   # без бэкенда: in-memory mock API с демо-данными
```

Демо-аккаунты (SPEC §6): `anna`, `boris`, `vera`, `dmitry`, пароль `demo1234`
(на странице входа есть кнопки «Войти как …»).

Проверки: `npm run lint` (eslint + stylelint + tsc), `npm run test` (vitest),
`npm run build`, `docker build .`.

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `NUXT_API_BASE_URL` | `http://localhost:8080` | адрес бэкенда для Nitro-прокси и SSR |
| `NUXT_MOCK_API` | `false` | `true` — mock API вместо бэкенда |
| `NUXT_COOKIE_SECURE` | — | `false` отключает флаг `Secure` у cookie в production (HTTP не на localhost) |
| `PORT` / `HOST` | `3000` / `0.0.0.0` в Docker | порт Nitro |

## Как устроено общение с API

```
браузер ── /api/auth/{login,register,logout} ──► Nitro: кладёт JWT в httpOnly cookie hbc_token
браузер ── /api/v1/** ──► Nitro-прокси (server/api/[...].ts) ── Authorization: Bearer <cookie> ──► бэкенд /v1/**
SSR ──────────────────────────────────────────── Authorization из cookie запроса ──► бэкенд /v1/**
```

- JWT никогда не попадает в JS: `/api/auth/*` возвращают только `{ user }`, а
  `/api/v1/auth/*` через прокси закрыты (404).
- Ошибки бэкенда приходят как есть: HTTP-статус + `{ code, message, details }`;
  `message` готов к показу (`getErrorMessage(error)` из `~/utils/errors`).
- Фото: `avatarSrc(user)` = `'/api' + user.avatarUrl` — файл отдаётся прокси с исходными
  `Content-Type`/`Cache-Control`.
- 401 на клиенте (кроме `/auth/*`) → стор очищается, тост «Сессия истекла», редирект на
  `/login?redirect=…` (`plugins/api.client.ts`).

### Mock-режим

`NUXT_MOCK_API=true`: прокси и `/api/auth/*` вызывают `server/mock` (все 22 эндпоинта,
фикстуры SPEC §6, правила SPEC §2.3, форма ответов и ошибок SPEC §4.2). Токен = username,
пароль демо-аккаунтов — `demo1234` (как на бэкенде). Состояние в памяти процесса —
сброс перезапуском. SSR в mock-режиме ходит в тот же прокси in-process.
Основной сценарий SPEC §1 через curl: `scripts/mock-smoke.sh [http://localhost:3000]`.

### Генерация клиента

```bash
cd ../back && buf generate     # back/pkg/api/hbc/v1/hbc.swagger.json
cd ../front && npm run api     # api/generated/hbc.ts (коммитится)
```

Компоненты и страницы импортируют только `~~/api/client` (`getApi()`), сервисы и типы
из `~~/api/types` — не `api/generated` напрямую.

### Локальные пакеты `@fe/*`

`lib/prop-types` и `lib/blob-avatar` подключены как `file:` с `install-links=true`
(`.npmrc`): npm копирует пакет (только `dist/`), а не делает симлинк — без второй копии
Vue и без dev-зависимостей пакета. После изменения `lib/<pkg>/dist`:
`rm -rf node_modules/@fe/<pkg> && npm i`.

## Структура

```
api/
  generated/hbc.ts        — сгенерированный клиент (не править)
  types.ts                — доменные типы (User, Listing, Application, enum'ы)
  client.ts               — getApi(): { auth, users, listings, applications }
  services/*.ts           — тонкие обёртки над generated
server/
  api/auth/*.post.ts      — login/register/logout + cookie
  api/[...].ts            — прокси /api/v1/** (или mock)
  mock/                   — in-memory mock API
app/
  layouts/{default,auth}.tsx
  middleware/auth.global.ts
  plugins/{auth.server,api.client}.ts
  stores/auth.ts
  composables/            — useApplicationsBadge (F4)
  components/ui/*         — UI-кит
  components/states/*     — empty-state, error-state
  components/cards/*      — listing-card, application-card, user-summary
  components/nav/*        — app-header, tab-bar, create-fab
  utils/                  — labels, errors, dates, validation, avatar, redirect
  pages/                  — экраны (SPEC §7)
lib/prop-types            — @fe/prop-types
tests/                    — vitest
```

## Соглашения для экранов (F1–F5)

- Компоненты — TSX, `defineComponent((props: Props, ctx) => () => <...>, { props, name })`,
  props через `prop<T>()` из `@fe/prop-types`, стили — `index.module.css` рядом.
  Строки в JSX — `{'Текст'}` (eslint `jsx-curly-brace-presence`).
- Данные: `const api = getApi()` синхронно в начале `setup()`, затем
  `useAsyncData('key', () => api.listings.get(id))` — SSR получает данные с cookie
  пользователя, клиент гидрируется без повторного запроса.
- Мутации: `try { await api.applications.accept(id) } catch (e) { toast.error(getErrorMessage(e)) }`.
- Нет объекта → `throw createError({ statusCode: 404 })` — покажет `app/error.tsx`.
- Текущий пользователь — `useAuthStore().user`; после изменения профиля — `setUser(user)`.
- Новые маршруты по умолчанию требуют входа (`middleware/auth.global.ts`), публичные —
  только `/login` и `/register`.
