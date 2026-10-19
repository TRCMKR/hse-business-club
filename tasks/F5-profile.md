# F5 — Профиль, публичный профиль, страница ошибки

**Зависимости:** F0. **Блокирует:** F6.
**Читать:** `SPEC.md` §2.3–2.4 (фото), §4.1 (`V1GetMe`, `V1UpdateMe`, `V1GetUser`,
`V1UploadAvatar`, `V1DeleteAvatar`), §7; `tasks/reports/F0.md`.

## Что сделать

1. `pages/me/index.tsx` + `components/profile/*`: карточка профиля (`avatar` из F0, имя,
   `@username`, headline, about, навыки-чипы, telegram как ссылка `https://t.me/<handle>`),
   «Редактировать» → inline-форма (vee-validate, схемы из `utils/validation.ts`:
   displayName, headline, about, skills через `chips-input` ≤ 20, telegram), «Сохранить»
   с `loading`, «Отмена». «Выйти» → `auth.logout` → `/login`. После сохранения —
   обновить стор `auth.user`.
1a. `components/profile/avatar-editor`: клик по аватару/кнопка «Загрузить фото» → скрытый
   `<input type=file accept="image/jpeg,image/png,image/webp">`; на клиенте проверить тип и
   размер ≤ 5 МБ (toast с теми же текстами, что в SPEC §2.3, без запроса), показать превью
   (`URL.createObjectURL`) с кнопками «Сохранить»/«Отмена»; «Сохранить» →
   `users.uploadAvatar(file)` с `loading`, ответ `User` → в стор `auth.user` (новый
   `avatarUrl` обновляет `<img>` везде, включая header); ошибки 400 — toast с `message`.
   «Удалить фото» → `confirm-dialog` → `users.deleteAvatar()`. На телефоне `capture` не
   ставить (пусть выбирает из галереи или камеры сам).
2. `pages/users/[id].tsx`: тот же `profile-card` без редактирования; 404 → «Участник не
   найден».
3. `error.tsx` (Nuxt error page): 404 — «Страница не найдена» + ссылка в ленту; прочее —
   «Что-то пошло не так» + «Повторить»/«В ленту». В layout по умолчанию.
4. Тесты: схема профиля, `profile-card` (telegram-ссылка, пустые поля не рендерят
   секции), `avatar-editor` (файл 6 МБ и `.gif` отклоняются без вызова service; после
   успешной загрузки вызван обновление стора).

## Владение

`front/app/pages/me/index.tsx`, `front/app/pages/users/[id].tsx`,
`front/app/components/profile/**`, `front/app/error.tsx`.

## Критерии приёмки

- [ ] Редактирование профиля на mock API сохраняется и отражается в header (имя).
- [ ] Загрузка фото на mock API: аватар меняется в header и карточке без перезагрузки; удаление возвращает сгенерированный аватар.
- [ ] `/users/<несуществующий uuid>` → состояние «не найден», `/listings/<не uuid>` →
      error page 404 (проверить вместе с F2 после merge, иначе отметить).
- [ ] `npm run lint`, `npm run test` зелёные.

## Отчёт `tasks/reports/F5.md`
