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
`npm run e2e` (Playwright: основной сценарий SPEC §1, состояния, refresh-сессия, axe — против mock API с access TTL 1s на
prod-сборке, порт 13100, системный Chrome; в `npm run test` не входит), `npm run build`,
`docker build .`.

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `NUXT_API_BASE_URL` | `http://localhost:8080` | адрес бэкенда для Nitro-прокси и SSR |
| `NUXT_MOCK_API` | `false` | `true` — mock API вместо бэкенда |
| `NUXT_COOKIE_SECURE` | `false` | `true` ставит флаг `Secure` у cookie `hbc_access` и `hbc_refresh`. Включать, когда фронт отдаётся через HTTPS-прокси; по умолчанию выключено, потому что compose открывает фронт по `http://localhost:3000`, а Safari не сохраняет Secure-cookie на HTTP |
| `NUXT_MOCK_ACCESS_TTL` | `15m` | срок access-токена в mock-режиме (`500ms`, `1s`, `15m`, `2h`, число — секунды); e2e запускаются с `1s` |
| `PORT` / `HOST` | `3000` / `0.0.0.0` в Docker | порт Nitro |

## Как устроено общение с API

```
браузер ── /api/auth/{login,register} ──► Nitro: access + refresh в httpOnly cookie hbc_access / hbc_refresh, в ответе {user}
браузер ── /api/auth/logout ──► Nitro: V1Logout(refresh) + очистка обеих cookie
браузер ── /api/v1/** ──► Nitro-прокси ── Authorization: Bearer <hbc_access> ──► бэкенд /v1/**
                                         └─ 401 → V1Refresh(hbc_refresh) → новые cookie → повтор запроса
SSR ── event.context.hbc (server/plugins/backend-context.ts) ── тот же механизм ──► бэкенд /v1/**
```

- Токены никогда не попадают в JS: `/api/auth/*` возвращают только `{ user }`, в
  клиентском бандле нет имён cookie (код приложения знает только тип
  `api/backend-context.ts`). Обе cookie — `HttpOnly; SameSite=Lax; Path=/; Max-Age=30 дней`,
  `Secure` по `NUXT_COOKIE_SECURE`. Срок access контролирует бэкенд (401), а не cookie.
- **Refresh** — `server/utils/session-refresh.ts#withRefresh(event, config, call)`, общий
  для прокси (`server/utils/proxy.ts`) и SSR:
  - запрос с access из cookie; при 401 и наличии `hbc_refresh` — один `POST /v1/auth/refresh`,
    новые cookie, повтор исходного запроса один раз (тело буферизуется — фото до ~7 МБ JSON);
  - есть только refresh-cookie → сразу refresh, без лишнего 401;
  - single-flight: результат refresh запоминается по значению старого refresh-токена на 5 с
    в рамках процесса — параллельные запросы страницы (и SSR, и клиент) получают одну и ту же
    новую пару, бэкенд не видит повторного использования и не отзывает семью;
  - refresh отклонён → обе cookie очищаются, клиенту 401 `{code:16, message:'Сессия истекла'}`;
    бэкенд недоступен → 502, cookie не трогаются;
  - внутри одного запроса новая пара берётся из `event.context`, поэтому все вызовы одного
    SSR-рендера после refresh идут уже с новым access.
- Прокси канонизирует путь (percent-decode сегментов, схлопывание `//`, без завершающего `/`,
  `.`/`..` и битый `%` → 404) и закрывает всё поддерево `/v1/auth/**` (404) —
  `server/utils/proxy-path.ts`. Cookie браузера и его `Authorization` на бэкенд не уходят.
- Ошибки бэкенда приходят как есть: HTTP-статус + `{ code, message, details }`;
  `message` готов к показу (`getErrorMessage(error)` из `~/utils/errors`).
- Фото: `avatarSrc(user)` = `'/api' + user.avatarUrl` — файл отдаётся прокси с исходными
  `Content-Type`/`Cache-Control`.
- 401 на клиенте (кроме `/auth/*`) → стор очищается, тост «Сессия истекла», редирект на
  `/login?redirect=…` (`plugins/api.client.ts`).

### Mock-режим

`NUXT_MOCK_API=true`: прокси и `/api/auth/*` вызывают `server/mock` (все 22 эндпоинта,
фикстуры SPEC §6, правила SPEC §2.3, форма ответов и ошибок SPEC §4.2) in-process.
Access и refresh — случайные строки; access живёт `NUXT_MOCK_ACCESS_TTL` (по умолчанию 15m),
refresh — 30 дней с ротацией: повтор уже использованного refresh отзывает всю семью
(SPEC §2.3), logout отзывает refresh. Пароль демо-аккаунтов — `demo1234` (как на бэкенде).
Состояние в памяти процесса — сброс перезапуском. Для тестов: `issueMockSession(username)`.
Основной сценарий SPEC §1 через curl: `scripts/mock-smoke.sh [http://localhost:3000]`;
с `SMOKE_ACCESS_TTL=1` (сервер с `NUXT_MOCK_ACCESS_TTL=1s`) скрипт проверяет ещё refresh,
обнаружение повторного использования и logout.

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
  api/[...].ts            — прокси /api/v1/** (или mock), refresh при 401
  plugins/backend-context.ts — event.context.hbc для SSR (тот же refresh)
  utils/                  — auth-cookie, backend, session-refresh, proxy, proxy-path
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

## Дизайн и токены

Стиль — в духе сайта Бизнес-клуба (hsebusinessclub.ru), но не копия: navy-навигация,
синий акцент для действий, жёлтый дозированно, шрифт Montserrat (`@nuxt/fonts`, Google,
веса 400–700, cyrillic + latin, `font-display: swap`). Логотип и блоки сайта не используются.

- Цвета, градиенты, тени, радиусы и размеры шрифта — CSS-переменные в
  `app/assets/css/main.css`; в `*.module.css` нет hex/`rgb()` и числовых `font-size`
  (проверка: `grep -rnE 'rgb\(|#[0-9a-f]{3,8}\b|font-size: *[0-9]' app --include='*.module.css'`
  → пусто). Геометрия (высоты, отступы мелких элементов) может быть в px. Тёмная тема —
  `prefers-color-scheme: dark` переопределяет те же переменные (navy-поверхности);
  `theme-color` задан двумя `<meta media>` (`#f4f6fb` / `#06081c`). Тосты `vue-sonner`
  перекрашены токенами в `main.css`.
- Шрифт: `--font-family` = `'Montserrat', 'Montserrat Fallback: Arial', …` — второе
  семейство генерирует `@nuxt/fonts` (метрически подогнанный Arial), без сдвига вёрстки
  при подмене шрифта.
- Рамка полей ввода `--color-border-strong` — ≥ 3:1 к фону и поверхности в обеих темах
  (WCAG 1.4.11); декоративная рамка при hover карточки — `--color-border-hover`.
- Фокус: глобальный `:focus-visible` (2px `--color-accent-text`); на navy-панелях —
  белый (`--color-nav-focus`); элементы с `outline: none` используют `--shadow-focus-ring`
  (поля — `--shadow-focus` + синяя рамка).
- `prefers-reduced-motion: reduce` гасит анимации и переходы; skeleton вместо
  «перелива» мягко пульсирует прозрачностью (индикатор загрузки остаётся).
- Текст на фоне: `--color-text*` (≥ 4.5:1 в обеих темах). Синий текст и ссылки —
  `--color-accent-text`, а не `--color-accent` (тот — для заливок кнопок и рамок).
- Primary-кнопка — `--gradient-accent`; secondary — прозрачная с синей рамкой.
- Жёлтый `--color-highlight` — **только** маркер активного пункта навигации и счётчик
  «Отклики» в навигации. Не использовать в кнопках и заливках.
- Радиусы: чипы `--radius-full`, кнопки/поля `--radius-md` (12), карточки `--radius-lg`
  (16), диалоги/sheet `--radius-xl` (20).
- `Card` не задаёт `display`: раскладку задаёт класс страницы, либо проп `gap`
  (`sm`/`md`/`lg` → flex-колонка с отступами).

Скриншоты (375 и 1280 px, светлая и тёмная тема): `docs/screenshots/`, «до» F8 —
`docs/screenshots/before/`. Сняты на живом бэкенде со свежим сидом, пользователь `anna`:
`login`, `feed`, `listing` (чужая), `listing-owner`, `listing-applications`, `listing-new`,
`me`, `me-listings`, `me-applications` (полученные), `user`, `not-found`; тёмная тема —
`login-dark`, `feed-dark`, `listing-dark`, `me-applications-dark`.

## Соглашения для экранов (F1–F5)

- Компоненты — TSX, `defineComponent((props: Props, ctx) => () => <...>, { props, name })`,
  props через `prop<T>()` из `@fe/prop-types`, стили — `index.module.css` рядом.
  Строки в JSX — `{'Текст'}` (eslint `jsx-curly-brace-presence`).
- Данные: `const api = getApi()` синхронно в начале `setup()`, затем
  `useAsyncData('key', () => api.listings.get(id))` — SSR получает данные с cookie
  пользователя, клиент гидрируется без повторного запроса.
- Мутации: `try { await api.applications.accept(id) } catch (e) { toast.error(getErrorMessage(e)) }`.
- Нет объекта → `throw createError({ statusCode: 404, statusMessage: 'Публикация не найдена', fatal: true })`
  в `async setup()` после `await` данных — покажет `app/error.tsx` (заголовок — `statusMessage`,
  если он на русском), и SSR сразу ответит HTTP 404, без скелетона и «прыжка» после гидрации.
- **В `async setup()` все композаблы вызываются до первого `await`**: `useHead`, `useRoute`,
  `useRouter`, `useAsyncData`, `onMounted`, `watch`, сторы. После `await` контекст Nuxt/Vue
  потерян: на SSR это 500 («nuxt instance unavailable»), на клиенте — молча не
  зарегистрированные хуки. Образец — `pages/listings/[id]/applications.tsx`:
  `const a = useAsyncData(...); …; await a; if (404) throw createError(...)`.
- Ошибки `useAsyncData` — обёртка Nuxt (`statusCode` 500 при обрыве сети) с исходной ошибкой
  в `cause`; `getErrorMessage`/`getErrorStatus` разворачивают её сами — передавайте `error.value` как есть.
- Текущий пользователь — `useAuthStore().user`; после изменения профиля — `setUser(user)`.
- `login`/`register`/`logout` и истёкшая сессия вызывают `clearNuxtData()`: кэш
  `useAsyncData` не переживает смену пользователя. Не храните пользовательские данные в
  других глобальных местах (модульные переменные, `useState`) — иначе их тоже нужно чистить.
  Исключение — общий `useState`, который сам привязан к `userId` и сбрасывается при его смене
  (бейдж откликов `composables/useApplicationsBadge.ts`: в состоянии хранится `userId`, и при
  несовпадении с `useAuthStore().user?.id` значение считается пустым).
- Новые маршруты по умолчанию требуют входа (`middleware/auth.global.ts`), публичные —
  только `/login` и `/register`.
