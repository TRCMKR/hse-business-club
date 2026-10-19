# F2 — Подробности публикации и отклик

**Зависимости:** F0. **Блокирует:** F6.
**Читать:** `SPEC.md` §2.3 (правила), §4.1 (`V1GetListing`, `V1CreateApplication`, `V1WithdrawApplication`), §7; `tasks/reports/F0.md`.

## Что сделать

`pages/listings/[id]/index.tsx` + `components/listing/*`:

1. `listing-header`: вид, статус («Публикация закрыта»), заголовок, автор (`user-summary`,
   ссылка `/users/:id`), относительная дата.
2. `listing-body`: описание (переносы строк), «Кого ищем» (`lookingFor` + теги), стадия /
   условия (по kind).
3. `apply-sheet`: `sheet` на мобильном / `dialog` на десктопе (один компонент), textarea
   1–2000 со счётчиком, кнопка «Отправить» с `loading`, серверная ошибка (400/409) под
   полем + toast; успех → закрыть, toast «Отклик отправлен», refresh деталей.
4. `my-application-block`: если `myApplication` — статус (бейдж), мой текст, дата; для
   pending — «Отозвать» с `confirm-dialog`; после withdraw — снова кнопка «Откликнуться».
5. `owner-panel` (для `isMine`): статистика `stats` (ожидают · приняты · отклонены),
   кнопки «Отклики (N)» → `/listings/:id/applications`, «Редактировать» (только active),
   «Закрыть» (confirm → `close` → refresh; идемпотентный повтор не показывает ошибку).
6. Панель действий: на мобильном sticky внизу над tab-bar; для чужой closed — кнопка
   disabled с подписью «Публикация закрыта».
7. Состояния: скелетон страницы; 404 → `error-state` «Публикация не найдена» + ссылка в
   ленту; ошибка сети → «Повторить». Повторные клики по «Откликнуться»/«Отозвать»
   блокируются `loading`.
8. Тесты: `apply-sheet` (валидация, блокировка повторной отправки, отображение 409),
   выбор панели действий по `isMine`/`status`/`myApplication` (табличный тест на
   чистую функцию `resolveActions(listing)`).

## Владение

`front/app/pages/listings/[id]/index.tsx`, `front/app/components/listing/**`.

## Параллелизация внутри

header+body ‖ apply-sheet+my-application ‖ owner-panel.

## Критерии приёмки

- [ ] На mock API от Бориса: публикация №1 показывает «Вы откликнулись · Ожидает», отзыв → снова «Откликнуться» → отклик → снова «Ожидает».
- [ ] От Анны: №1 показывает owner-panel со stats `1 · 1 · 0`, закрытие работает, повторное закрытие без ошибки.
- [ ] 375px: панель действий sticky, sheet снизу; 1280px: dialog по центру.
- [ ] `npm run lint`, `npm run test` зелёные.

## Отчёт `tasks/reports/F2.md`
