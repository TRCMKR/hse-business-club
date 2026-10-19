# B5 — Сборка, адаптеры, сид, e2e

**Зависимости:** B1, B2, B3, B4. **Блокирует:** F6, I1.
**Читать:** `SPEC.md` §1, §4, §6; `back/ARCHITECTURE.md`; отчёты `tasks/reports/B0..B4.md`.

## Что сделать

1. `internal/adapters/listings_reader.go`: `applications.ListingReader` поверх
   `listings.Service` (`IsOpen = status == active`).
2. `internal/app` + `cmd/hbc/main.go`: миграции трёх контекстов по порядку (`users`,
   `listings`, `applications`), blob-стор (`s3.New`, бакет), сборка сервисов и BFF, сид,
   сервер. `readyz` зелёный только после миграций и `blob.Ping`.
3. `internal/seed`: демо-данные SPEC §6, идемпотентно (если `users.users` не пуста —
   пропуск с логом). Сид идёт **через сервисы контекстов**, не через SQL, чтобы правила
   соблюдались: closed-публикация №6 — создать, принять отклик boris, затем `Close`;
   withdrawn — `Apply` + `Withdraw`; accepted/declined — `Accept`/`Decline` от автора.
   UUID генерируются; стабильные ключи — `username` и `title` (SPEC §6).
4. `tests/e2e` (`//go:build e2e`): поднимает приложение in-process на свободных портах с
   `TEST_POSTGRES_DSN` и `blob/memory` вместо S3, миграции, `TRUNCATE` всех таблиц между
   тестами, HTTP-клиент с
   хелперами (`login(username) token`, `do(method, path, token, body) (status, json)`).
   Сценарии 1–8 из SPEC §1 и §2.3:
   1. основной сценарий целиком (register ×2 → create → feed → apply → received → accept → sent ACCEPTED → `myApplication` в деталях);
   2. повторный отклик 409; withdraw → повторный 200 с новым id;
   3. свой/закрытый → 400 с текстами из SPEC;
   4. чужой accept → 404; чужой update → 403; без токена → 401; невалидный токен → 401;
   5. accept×2 → 200/200; decline после accept → 400;
   6. closed не в ленте, доступен по id, close×2 → 200;
   7. фильтры kind/query, пагинация, limit=51 → 400;
   8. регистрация: короткий пароль/плохой username → 400, дубль → 409, неверный пароль → 401;
   9. сид: запуск с `SEED_DEMO_DATA=true` на пустой БД → login anna → лента `total=5`;
      повторный сид не дублирует.
   10. фото: `POST /v1/me/avatar` с PNG 300×200 (сгенерировать в тесте) → `avatarUrl`
       непустой → `GET` по нему без токена → 200, `image/jpeg`, `Cache-Control` immutable,
       декодируется в 512×512; повторная загрузка → другой `avatarUrl`, старый → 404;
       `DELETE` → `avatarUrl == ""`, `DELETE` ещё раз → 200; мусор вместо картинки → 400
       «Фото: поддерживаются JPEG, PNG и WebP»; `avatarUrl` автора виден в ленте и в
       `applicant` откликов.
   Проверяй **форму JSON**: camelCase-ключи, enum-строки, `closedAt: null`, `stats: null`
   для чужих, формат ошибки `{code, message, details}`.
5. `Makefile`: `test` = unit; `test-integration` = тесты репозиториев; `test-e2e` =
   `tests/e2e`; `test-all` = всё. Цели с БД поднимают Postgres (пока
   `back/docker-compose.test.yaml`, порт 5433; I0 заменит на корневой compose), ждут
   `pg_isready`, запускают тесты, гасят контейнер.
6. Сверить `hbc.swagger.json` с реальным поведением (пути, методы); README бэка дополнить.
7. Gateway: ошибки разбора тела (невалидный JSON, неизвестный enum) должны отдаваться как
   400 с текстом «Некорректный формат запроса» (SPEC §2.4, ARCHITECTURE §3) — проверить в
   e2e, при необходимости доработать error handler в `internal/platform/server` (правка вне
   зоны — в отчёт).
8. `StatsForListings`: `total` без withdrawn (SPEC §4.2) — проверить в e2e на сиде (у
   публикации №5 withdrawn от Веры → `stats.total = 0`).

## Выходы

Рабочее приложение целиком; сид; e2e; обновлённые Makefile/README.

## Владение

`back/cmd/**`, `back/internal/adapters/**`, `back/internal/seed/**`, `back/tests/**`,
`back/Makefile`, `back/README.md`. Правки в контекстах/BFF — допустимы для починки стыков,
каждая — в отчёт.

## Параллелизация внутри

адаптер+main+seed ‖ каркас e2e (хелперы) → сценарии (можно раздать по 3 на субагента).

## Критерии приёмки

- [ ] `make lint`, `make test`, `make test-all` зелёные с нуля.
- [ ] `docker build ./back` и запуск контейнера с внешним Postgres и MinIO: `/readyz` 200, бакет создан, сид применён.
- [ ] Отчёт содержит список отклонений от SPEC, найденных e2e, и как исправлены.

## Отчёт `tasks/reports/B5.md`

Команды, время прогона e2e, правки в чужих зонах, замечания для фронта (точные тексты
ошибок, формы ответов).
