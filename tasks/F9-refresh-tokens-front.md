# F9 — Access + refresh токены (фронтенд)

**Зависимости:** F6 (в main); контракт `hbc.proto` обновлён (B7 делает бэк параллельно —
работать на mock API и по SPEC). **Блокирует:** финальную выкладку.
**Читать:** `SPEC.md` §4.1–4.3 (`V1Refresh`, `V1Logout`, форма `V1AuthResponse`), §5
(cookie `hbc_access`/`hbc_refresh`, поведение прокси при 401), `back/api/hbc/v1/hbc.proto`,
`tasks/DECISIONS.md` (07:00), `tasks/reports/F0.md` (прокси, auth-роуты, mock), `F6.md`,
код `front/server/{api,mock,utils}/**`, `front/app/{api,stores,middleware,plugins}/**`,
`front/tests/**`, `front/e2e/**`.

## Что сделать

1. **Клиент:** регенерировать `api/generated/hbc.ts` из swagger (`cd back && buf generate` в
   worktree локально, как делал F0; `back/pkg` не коммитить — B7 закоммитит свой);
   `api/types.ts`, services `auth.*` → новые поля.
2. **Cookie:** `server/utils/auth-cookie.ts` — две httpOnly-cookie `hbc_access` и
   `hbc_refresh` (`SameSite=Lax`, `Path=/`, `Secure` по `NUXT_COOKIE_SECURE`, `Max-Age` =
   30 дней у обеих — срок access контролирует сервер); `hbc_token` удалить везде
   (middleware, plugin, тесты, README, smoke-скрипт фронта).
3. **Auth-роуты Nitro:** `login`/`register` ставят обе cookie из `V1AuthResponse`, в ответе
   только `{user}`; `logout` → `POST /v1/auth/logout` с refresh (ошибку бэка глотать) →
   очистить обе cookie.
4. **Прокси `/api/v1/**` и SSR `getApi()`:** общий helper `withRefresh(event, fn)`: запрос с
   access; при 401 от бэкенда и наличии `hbc_refresh` — один `POST /v1/auth/refresh`
   (single-flight на запрос; параллельные запросы одной страницы не должны ротировать токен
   дважды — дедуплицировать по значению refresh в рамках процесса на ~5 с), поставить новые
   cookie, повторить исходный запрос один раз; неудача refresh → очистить cookie → 401
   клиенту (существующий сценарий «Сессия истекла»). Тело исходного запроса для повтора
   буферизовать (фото до 8 МБ — допустимо). `/api/v1/auth/**` по-прежнему закрыт в прокси.
5. **SSR-плагин/middleware:** `fetchMe` при наличии `hbc_refresh` (даже если access истёк) —
   через тот же helper; без обеих cookie → `/login?redirect=`.
6. **Mock API:** `V1Refresh`/`V1Logout`, access с TTL (`NUXT_MOCK_ACCESS_TTL`, default 15m;
   в тестах 1s), ротация и отзыв семьи как в SPEC §2.3, форма ответа по §4.3.
7. **Тесты:** unit на helper (401 → refresh → retry; параллельные запросы — один refresh;
   неудачный refresh → cookie очищены), auth-роуты, mock (ротация/семья/logout); Playwright:
   сценарий «access истёк посреди сессии → действие проходит без редиректа» (mock с TTL 1s)
   и «refresh отозван → «Сессия истекла» → /login». `npm run lint/test/e2e/build` зелёные.
8. README фронта и корневой README (абзацы про cookie) — обновить; `scripts/mock-smoke.sh`.

## Владение

`front/**`; корневой `README.md` — только абзацы про cookie фронта. Не трогать `back/**`
(кроме локальной генерации swagger без коммита) и `scripts/smoke.sh` (B7).

## Критерии приёмки

- [ ] Токены в JS недоступны: `document.cookie` пуст, в бандле нет имён cookie/токенов.
- [ ] На mock с `NUXT_MOCK_ACCESS_TTL=1s`: после паузы > 1 с любое действие проходит без видимого сбоя (один refresh в логе), повторный refresh старым токеном → 401.
- [ ] Выход отзывает refresh: после logout старая cookie не оживляет сессию.
- [ ] `npm run lint`, `npm run test`, `npm run e2e`, `npm run build` зелёные.

## Отчёт `tasks/reports/F9.md`

Схема cookie/refresh, helper, что не сделано, замечания для I1-повтора (живой бэк).
