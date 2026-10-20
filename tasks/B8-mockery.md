# B8 — Unit-тесты бэкенда на моках mockery

**Зависимости:** B7 (в main). **Блокирует:** —.
**Читать:** `back/ARCHITECTURE.md` §6a (соглашение), `tasks/DECISIONS.md` (07:00), все
`*_test.go` в `back/internal/**` (особенно `internal/bff/fakes_test.go`,
`internal/*/service/fakes_test.go`), `back/Makefile`, `back/.golangci.yaml`.

## Что сделать

1. Mockery v3 (последняя стабильная) через `go tool`/`go run …@vX` в Makefile (`make
   generate-mocks`), конфиг `back/.mockery.yml`: `with-expecter: true`, `dir:
   "{{.InterfaceDir}}/mocks"`, `mockname: "Mock{{.InterfaceName}}"`, `filename:
   "mock_{{.InterfaceName | snakecase}}.go"`, `outpkg: mocks`; интерфейсы: `users.Service`,
   `listings.Service`, `applications.Service`, `applications.ListingReader`, `blob.Store`,
   `auth.Issuer`, `auth.Verifier`, `users/service.{Repository,Hasher,RefreshRepository}` (как
   назвал B7), `listings/service.Repository`, `applications/service.Repository`. Моки
   коммитятся; `.golangci.yaml` исключает `**/mocks/**`; `make generate-mocks` без диффа.
   `github.com/stretchr/testify` в `go.mod`.
2. Переписать unit-тесты на моки с `EXPECT()`: `internal/bff/*_test.go` (удалить
   `fakes_test.go`; счётчики вызовов → `.Times(n)`/`AssertNumberOfCalls`; батч-критерий
   «число вызовов не зависит от N» сохранить), `internal/{users,listings,applications}/service/*_test.go`
   (удалить фейковые репозитории; транзакционные `Update(ctx,id,fn)` мокать с `Run`, вызывая
   `fn`), `internal/seed` (если есть unit), `internal/adapters`. Табличные тесты domain,
   integration (`dbtest`), e2e и httptest-тесты gateway/server — без изменений (там нужны
   реальные реализации).
3. Покрытие не ниже прежнего (`go test -cover` по пакетам: зафиксировать до/после в отчёте);
   все проверки правил SPEC §2.3 из прежних тестов сохранены (составить таблицу
   «проверка → тест»).
4. `make lint`, `make test`, `make test-all` зелёные; README бэка — раздел «Тесты» про моки.

## Владение

`back/**` (кроме `back/api/**`). Код приложения не менять, кроме случаев, когда интерфейс
нужно выделить для мока (минимально, в отчёт).

## Критерии приёмки

- [ ] В `*_test.go` нет рукописных фейков портов/репозиториев (`grep -rn "type fake" back/internal` пуст); все моки — из `mocks/`.
- [ ] `make generate-mocks` без диффа; моки с `EXPECT()`; `go vet`/lint чисты.
- [ ] Покрытие пакетов bff и service не упало; таблица «правило SPEC → тест» в отчёте.

## Отчёт `tasks/reports/B8.md`
