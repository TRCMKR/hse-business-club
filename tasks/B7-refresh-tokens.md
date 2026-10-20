# B7 — Access + refresh токены (бэкенд)

**Зависимости:** B6, I1 (в main). **Блокирует:** B8 (mockery), финальную выкладку.
**Читать:** `back/api/hbc/v1/hbc.proto` (уже обновлён: `V1Refresh`, `V1Logout`, `V1AuthResponse`),
`SPEC.md` §2.3 (новые строки про refresh), §3 (`users.refresh_tokens`), §4.1–4.3, §5;
`back/ARCHITECTURE.md` §2 (порт users: `IssueRefresh/RotateRefresh/RevokeRefresh`, `RefreshToken`;
`auth`: `TTL()`), §5 (`ACCESS_TTL`, `REFRESH_TTL`); `tasks/DECISIONS.md` (запись 07:00);
код `back/internal/platform/auth`, `back/internal/platform/config`, `back/internal/users/**`,
`back/internal/bff/{auth.go,service.go}`, `back/migrations/`, `back/tests/e2e`, `scripts/smoke.sh`.

## Что сделать

1. `make generate` по обновлённому proto (новые RPC, поля `access_token/refresh_token/access_expires_in`);
   `pkg/api` закоммитить отдельно. Swagger обновится — фронт (F9) регенерирует клиент сам.
2. Конфиг: `JWT_TTL` → `ACCESS_TTL` (default 15m), новый `REFRESH_TTL` (default 720h);
   валидация > 0; `.env.example`, README бэка, корневой README (§Безопасность/§Архитектура —
   абзацы про JWT) — обновить; `docker-compose.yaml`/Kamal env не трогать (дефолты).
3. `platform/auth`: `JWT.TTL()`; `publicMethods` BFF += `V1Refresh`, `V1Logout`.
4. Контекст users: миграция `back/migrations/20261020000004_refresh_tokens.sql` (SPEC §3,
   схема `users`, FK на `users.users` допустим — та же схема); репозиторий
   `refresh_tokens` (`Create`, `GetByHash`, `RevokeByID`, `RevokeFamily`, `DeleteExpired`);
   сервис: `IssueRefresh` (32 случайных байта → base64url без паддинга, хранить sha256,
   `family_id` новый), `RotateRefresh` (в транзакции: найти по хешу; нет/истёк → 401
   «Сессия истекла»; `revoked_at != NULL` → отозвать всю семью и 401; иначе отозвать текущий,
   выдать новый той же семьи; попутно `DeleteExpired` пользователя), `RevokeRefresh`
   (идемпотентно). Токен наружу — только в `RefreshToken.Token`, хеш — никогда.
5. BFF: `V1Register`/`V1Login` → access (`Issuer.Issue`) + `IssueRefresh` + `access_expires_in`
   (из `TTL()`); `V1Refresh` → `RotateRefresh` → новый access + новый refresh + `user`
   (через `Users.Get`); `V1Logout` → `RevokeRefresh` → `{}`; пустой `refresh_token` → 401
   «Сессия истекла». Валидация формы — в `validate.go`.
6. Тесты: unit (сервис users: выдача/ротация/повторное использование → семья отозвана/
   истёкший/отзыв идемпотентен; BFF: четыре RPC с кодами и текстами), integration репозитория
   (unique по хешу, `RevokeFamily`, `DeleteExpired`), e2e: login → `accessToken` работает →
   `ACCESS_TTL=1s` в харнесе → через 2 с 401 «Требуется вход» → `V1Refresh` → новая пара
   работает → старый refresh повторно → 401 и новый refresh тоже отозван (семья) → login →
   logout → refresh после logout → 401; `/v1/auth/refresh` и `/logout` без Authorization — ок.
   Форма JSON: `accessToken`, `refreshToken`, `accessExpiresIn`, `user`.
7. `scripts/smoke.sh`: `.token` → `.accessToken`; добавить шаг refresh (`POST /v1/auth/refresh`
   → новый `accessToken` работает) и logout (→ 200, повторный refresh → 401). Корневой
   `make smoke` — без изменений.
8. `tests/e2e`: все 24 операции swagger маршрутизируются (тест на количество обновить).

## Владение

`back/**` (кроме `back/api/**` — контракт уже обновлён координатором), `scripts/smoke.sh`,
корневой `README.md` (абзацы про токены/cookie), `.env.example`. Не трогать `front/**`
(параллельно идёт F9) и `config/deploy*.yml` (дефолты TTL достаточны).

## Критерии приёмки

- [ ] `make lint`, `make test`, `make test-all` зелёные; `make reset && make up && make smoke` (override портов) зелёный с новым smoke.
- [ ] Повторное использование ротированного refresh отзывает семью (e2e).
- [ ] В БД нет открытых refresh-токенов (только хеши); в логах токены не печатаются.
- [ ] `grep -rn "JWT_TTL"` по репо пуст (кроме DECISIONS/отчётов).

## Отчёт `tasks/reports/B7.md`

Порт, таблица, решения, точные тексты/формы для фронта (F9) и для README.
