# F0 — Фундамент фронтенда

**Зависимости:** контракт `back/api/hbc/v1/hbc.proto` (есть). Бэкенд не нужен. **Блокирует:** F1–F6, I0.
**Читать:** `CLAUDE.md`, `SPEC.md` §2.3–2.4, §4, §5, §6, §7; `back/api/hbc/v1/hbc.proto`.
**Шаблон:** `/Users/trcmkr/Проекты/blanc-front` (копировать всё, кроме `node_modules`, `.nuxt`,
`.output`, `.git`, `.idea`, `release`, `.kamal`, `config/deploy.yml`). Стиль компонентов —
`blanc-front/app/components/ping` (TSX, `defineComponent`, `prop<T>()` из `@fe/prop-types`, CSS Modules).

## Цель

Приложение, в котором можно войти, выйти, увидеть каркас навигации на телефоне и
десктопе, а все экраны F1–F5 — заглушки с нужными маршрутами. Плюс всё общее: клиент API,
прокси с cookie-auth, mock-режим API, UI-кит, доменные карточки, подписи, утилиты, тесты.

## Что сделать

1. **Проект.** Копия шаблона в `front/`, имя `hbc-front`, `npm i`. Nuxt 4, SSR, Pinia,
   `vee-validate` + `yup`, `vue-sonner`, `date-fns`, `@vueuse/core`, `@nuxt/fonts` (Inter).
   Локальные пакеты: `@fe/prop-types` (из шаблона) и `@fe/blob-avatar`
   (`file:lib/blob-avatar`, задача F7) — если `front/lib/blob-avatar` уже в `main`,
   подключить сразу.
   `runtimeConfig.apiBaseUrl` (env `NUXT_API_BASE_URL`, дефолт `http://localhost:8080`),
   `runtimeConfig.mockApi` (env `NUXT_MOCK_API`, дефолт `false`).
2. **Клиент API.** Скрипт `api`: `swagger-typescript-api generate -p
   ../back/pkg/api/hbc/v1/hbc.swagger.json -o ./api/generated -n hbc.ts --axios
   --module-name-first-tag` (если swagger ещё не сгенерирован — `cd back && buf generate`,
   плагины `go install`; результат в `back/pkg` закоммить отдельным коммитом). Результат
   коммитится. `api/client.ts` → `getApi()` по SPEC §5: на клиенте `baseURL='/api'`, на
   сервере `apiBaseUrl` + `Authorization` из cookie запроса; без кэша между SSR-запросами.
   `api/services/{auth,users,listings,applications}.ts` — тонкие обёртки с понятными именами
   и типами из generated (`listings.list({kind, query, page, limit})`,
   `applications.accept(id)`, `users.uploadAvatar(file: File)` — читает файл в base64 и шлёт
   `{data}`, `users.deleteAvatar()`). Компоненты импортируют только services и типы.
   `V1GetAvatar` через клиент не вызывается: `utils/avatar.ts` → `avatarSrc(user)` =
   `user.avatarUrl ? '/api' + user.avatarUrl : null`.
3. **Сервер Nitro.** `server/api/auth/{login,register,logout}.post.ts` (cookie `hbc_token`
   httpOnly, sameSite=lax, path=/, maxAge=24h, secure в production; ошибки бэка
   пробрасываются с тем же статусом и `{code, message}`), `server/api/[...].ts` —
   прокси `/api/v1/**` → `${apiBaseUrl}/v1/**` с `Authorization` из cookie (`proxyRequest`);
   бинарные ответы и заголовки `Cache-Control`/`Content-Type` проходят как есть
   (проверить на `/api/v1/avatars/...`), лимит тела запроса ≥ 8 МБ.
4. **Mock API** (`server/mock/`): при `mockApi=true` прокси и auth-роуты вместо бэкенда
   вызывают in-memory реализацию всех 22 эндпоинтов с фикстурами SPEC §6 (пароль любой
   из демо-логинов → токен = username), правила из SPEC §2.3 (409 на повторный отклик,
   закрытая публикация, права), форма ответов и ошибок строго по SPEC §4.2–4.3. Состояние
   живёт в памяти процесса, сброс — перезапуск. Фото в mock: `POST /v1/me/avatar` кладёт
   байты в Map под `<uuid>.jpg` (без обработки), `GET /v1/avatars/{file}` отдаёт их с
   `Content-Type` исходника; у демо-аккаунтов фото нет. Это даёт F1–F5 рабочее приложение
   без бэкенда и фикстуры для тестов.
5. **Auth.** Pinia `stores/auth.ts` (`user`, `login`, `register`, `logout`, `fetchMe`);
   `plugins/auth.server.ts` — `fetchMe` при наличии cookie; `middleware/auth.global.ts`
   — редиректы по SPEC/TASK (без user → `/login?redirect=`; с user на login/register →
   `/`); обработчик 401 в `getApi()` (клиент): очистить стор, toast «Сессия истекла»,
   редирект.
6. **Layout и навигация.** `layouts/default.tsx` (header ≥ 768px, tab-bar < 768px,
   safe-area, контейнер 1040px), `layouts/auth.tsx`. `components/nav/app-header`,
   `components/nav/tab-bar`, `components/nav/create-fab`. Бейдж на пункте «Отклики»
   читает `composables/useApplicationsBadge.ts` — в F0 возвращает `ref(0)`, реализует F4.
7. **UI-кит** `components/ui/*`: `button` (primary/secondary/ghost/danger; md/sm; `loading`
   → disabled + спиннер; `as` link), `input`, `textarea` (счётчик символов), `select`,
   `segmented` (сегмент-контрол), `chip`, `chips-input` (Enter/запятая, ≤ N), `badge`,
   `card`, `dialog` (desktop) и `sheet` (mobile) с общим `modal` (Esc, focus trap,
   возврат фокуса, `aria-modal`), `confirm-dialog`, `skeleton`, `avatar`, `tabs`,
   `page-header`. Состояния `components/states/{empty-state,error-state}`.
   `avatar`: props `user: Pick<User,'username'|'avatarUrl'|'displayName'>`, `size` (24–128);
   `avatarUrl` → `<img>` (lazy, `alt=displayName`, круг, при ошибке загрузки — fallback);
   иначе `<BlobAvatar seed={username} size={size} background="circle" />` из
   `@fe/blob-avatar/vue` (F7). Если F7 ещё не смержен — временно инициалы на цветном круге
   (цвет из хеша `username`) и пометка `TODO(F7)` в коде и отчёте; подключение сделает F7
   (шаг 8) или F6. Один и тот же `username` → одна и та же картинка.
8. **Доменные карточки** `components/cards/`: `listing-card` (все бейджи SPEC §7:
   вид, «Ваша публикация», статус моего отклика, закрыта; теги ≤ 3 + «+N»; `stats` если
   есть; слот действий), `application-card` (аватар, имя, headline, сообщение, статус,
   время; режимы `incoming`/`outgoing`; слот действий), `user-summary`.
9. **Утилиты** `app/utils/`: `labels.ts` (все enum → русские подписи + цвета бейджей),
   `errors.ts` (`getErrorMessage`), `dates.ts` (относительное время, ru), `validation.ts`
   (общие yup-схемы по SPEC §2.4: username, password, displayName, title, description,
   lookingFor, tags, message, headline, about, skills, telegram).
10. **Экраны входа.** `pages/login.tsx` (форма + 4 кнопки «Войти как …» + ссылка на
    регистрацию; 401 под формой), `pages/register.tsx`. Заглушки с `page-header`:
    `pages/index.tsx`, `listings/new.tsx`, `listings/[id]/{index,edit,applications}.tsx`,
    `me/{index,listings,applications}.tsx`, `users/[id].tsx`, `error.tsx` (404/500).
11. **Тесты и качество.** vitest + `@vue/test-utils`; тесты на `labels`, `errors`,
    `validation`, `button` (loading блокирует клик), `avatar` (img при `avatarUrl`, SVG без;
    детерминизм), `listing-card`, `application-card`, mock API (повторный отклик → 409,
    загрузка фото → `avatarUrl`). `npm run lint` чисто. `Dockerfile` из шаблона
    (проверить сборку). `README.md` фронта: запуск, env, mock-режим, генерация клиента,
    структура, соглашения для F1–F5.

## Выходы

Всё выше; `NUXT_MOCK_API=true npm run dev` → вход «как Анна» → каркас с навигацией и
заглушками, выход. При выключенном mock — то же против живого бэка (проверить, если B0/B5
уже доступны; иначе отметить в отчёте).

## Владение

`front/**`. После F0 общие зоны (`api/`, `server/`, `stores/`, `layouts/`, `components/{ui,nav,cards,states}`,
`utils/`) меняют только с записью в отчёт.

## Параллелизация внутри

После шага 1: (a) клиент + services + Nitro-прокси + auth-роуты, (b) mock API,
(c) UI-кит + tokens, (d) utils + тесты на них. Затем (e) store/plugin/middleware/layout
(нужны a, c), (f) карточки (нужны c, d), (g) login/register + заглушки (нужны e).

## Критерии приёмки

- [ ] `npm run lint`, `npm run test`, `npm run build`, `docker build ./front` зелёные.
- [ ] JWT нет в JS: cookie httpOnly, в бандле и `localStorage` токена нет.
- [ ] Прямой заход на `/me/listings` без cookie → `/login?redirect=/me/listings`; с cookie — SSR без моргания.
- [ ] Mock API проходит основной сценарий SPEC §1 через curl к `localhost:3000/api/...`.
- [ ] Навигация корректна на 375px и 1280px (скриншоты в отчёт).

## Отчёт `tasks/reports/F0.md`

Список компонентов UI-кита с props, services API, как включить mock, соглашения по
страницам (как получать данные: `useAsyncData` + `getApi()`), что не сделано.
