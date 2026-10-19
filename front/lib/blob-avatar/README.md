# @fe/blob-avatar

Детерминированные SVG-аватары из любой строки (в приложении — `username`): спокойная
абстрактная композиция в пастельных тонах — крупная фигура и 1–2 акцентных элемента, без лиц.
Чистый TypeScript без runtime-зависимостей и Vue-компонент (`vue` — peer). Один и тот же
seed даёт байт в байт одинаковый SVG в Node (SSR) и в браузере. Код свой.

![Галерея](docs/gallery.png)

## API

```ts
import { blobAvatar, blobAvatarTree, deriveTraits } from '@fe/blob-avatar';

blobAvatar('anna');                                   // '<svg xmlns=... role="img" aria-label="anna">…</svg>'
blobAvatar('anna', { size: 48, background: 'squircle', aria: 'Анна Соколова' });
blobAvatar('anna', { traits: { shape: 'round', palette: 3 } }); // закрепить оси
blobAvatarTree('anna');                               // SvgNode { tag, attrs, children }
deriveTraits('anna');                                 // Traits
```

- `blobAvatar(seed, options?) → string` — SVG-строка.
- `blobAvatarTree(seed, options?) → SvgNode` — дерево элементов для любого view-слоя.
- `deriveTraits(seed, overrides?) → Traits` — трейты из seed; `overrides` закрепляет любые оси
  (каждая ось тратит ровно один вызов rng в фиксированном порядке, поэтому закрепление одной
  оси не меняет остальные).
- Также экспортируются `buildTree(traits, options)`, `toSvgString(tree)`, `hashString`,
  `createRng`, `normalizeSeed`, наборы вариантов (`SHAPES`, `ACCENTS`, `DETAILS`, `PALETTES`)
  и типы.

Опции (`BlobAvatarOptions`):

| опция | тип | по умолчанию |
|---|---|---|
| `size` | `number` — `width`/`height` в px | нет атрибутов, SVG занимает 100% контейнера |
| `background` | `'none' \| 'circle' \| 'squircle' \| 'square'` | `'circle'` |
| `aria` | `string` — `aria-label` | нормализованный seed (`trim().toLowerCase()`), пустой → `'avatar'` |
| `traits` | `Partial<Traits>` | — |

Seed нормализуется: `trim().toLowerCase()` — `'Anna '` и `'anna'` дают байт в байт одинаковый
SVG (включая метку по умолчанию). `aria-label` обрезается до 64 символов (`MAX_LABEL_LENGTH`),
чтобы длинный seed не раздувал SVG. Значения `traits`, не входящие в наборы вариантов (или
вне `0..1` для непрерывных осей, `palette` — не целое из `0..11`), игнорируются — берётся выведенное из seed.

### Vue

```tsx
import { BlobAvatar } from '@fe/blob-avatar/vue';

<BlobAvatar seed={username} size={40} background="circle" />
<BlobAvatar seed="vera" traits={{ palette: 3 }} label="Вера Лисицына" />
```

Пропсы: `seed: string` (обязательный), `size?: number`, `background?`, `traits?:
Partial<Traits>`, `label?: string`. Рендер через `h()` по дереву (без `innerHTML`): SSR-safe,
гидрация совпадает с `blobAvatar()`.

## Оси (`Traits`)

Композиция: фон палитры → базовая фигура (цвет `figure`) → акцент (цвет `accent`) → деталь.

| ось | варианты |
|---|---|
| `shape` | базовая фигура: `round`, `squircle`, `blob` (гладкая кривая Catmull-Rom → Безье через 5–7 точек с небольшим джиттером), `droplet` (круг с одним острым углом), `hexagon`, `capsule` |
| `shapeSeed` | uint32: контур `blob` |
| `palette` | индекс в `PALETTES`, `0..11` |
| `accent` | `orbit` (кружок на краю фигуры), `ring` (кольцо внутри), `arc` (полукольцо внутри), `inset` (уменьшенная копия фигуры), `overlap` (полупрозрачный круг внахлёст), `split` (полукруг-срез внутри) |
| `detail` | `none`, `satellite` (маленькая точка рядом с фигурой), `trail` (тонкая дуга вдоль края) — всегда с противоположной от акцента стороны |
| `rotation` | `0..1` — поворот фигуры (доля полного оборота) |
| `accentAngle` | `0..1` — направление акцента от центра; фигура слегка смещается в противоположную сторону для баланса |
| `scale` | `0..1` — размер фигуры (0.88–1.02 от номинала) |

Итого 6 × 12 × 6 × 3 = 1 296 дискретных комбинаций плюс непрерывные поворот, направление и
масштаб (на 200 случайных seed — 200 разных SVG).

### Палитры (`PALETTES`)

Пары «фон / фигура / акцент» подобраны вручную: низкая насыщенность, высокая светлота, без
чистого белого и чёрного; фигура заметно, но мягко контрастирует с фоном и читается при 24 px.

| `palette` | имя | фон | фигура | акцент |
|---|---|---|---|---|
| 0 | rose | `#f5e6e4` | `#dba8a3` | `#a86f6c` |
| 1 | sage | `#e7eee3` | `#a9c1a0` | `#647f62` |
| 2 | sand | `#f3ece0` | `#d1b98f` | `#9a7f55` |
| 3 | lavender | `#ebe8f3` | `#b8afd6` | `#73699c` |
| 4 | sky | `#e4edf4` | `#a2c0d9` | `#5f84a6` |
| 5 | mint | `#e2f1ec` | `#9ecfbf` | `#558d7d` |
| 6 | peach | `#f8eae0` | `#ebba9b` | `#b57859` |
| 7 | graphite | `#e8e9eb` | `#a9adb4` | `#5f646d` |
| 8 | butter | `#f5efda` | `#dfcd8c` | `#9a8644` |
| 9 | mauve | `#f1e5ec` | `#d0a8c0` | `#916582` |
| 10 | denim | `#e4e7f0` | `#a0aac9` | `#5c688f` |
| 11 | clay | `#f3e5de` | `#d2a08c` | `#965e4f` |

## Гарантии вывода

- `viewBox="0 0 100 100"`, `role="img"`, `aria-label`; числа округлены до 2 знаков.
- Нет `id`, `<style>`, `<script>`, внешних ссылок — SVG можно вставлять на страницу много раз.
- Размер одного аватара ~0.3–1 КБ (тест держит < 4 КБ, в т. ч. для seed длиной 100 000 символов).
- Цвета — только из `PALETTES` (hex); не больше 4 элементов поверх корня (фон, фигура, акцент, деталь).

## Разработка

```sh
npm ci
npm test            # vitest: детерминизм, уникальность, все комбинации, палитры, снапшоты демо-аккаунтов, Vue/SSR
npm run typecheck
npm run build       # vite (lib, preserveModules) + tsc (d.ts) → dist/, коммитится
npm run gallery     # gallery.html (в .gitignore)
npm run screenshot  # gallery.html + docs/gallery.png через headless Chrome (CHROME_PATH — путь к Chrome)
```

Галерея: сетка 10 × 10 (демо-аккаунты, все фигуры, акценты, детали, случайные seed'ы), все
12 палитр при 96 и 24 px, четыре варианта фона. Она же повторно рендерит все аватары в браузере
(бандл `src/index.ts`) и сравнивает строки с Node-версией — результат в шапке страницы.

Снапшоты `tests/__snapshots__` фиксируют аватары демо-аккаунтов (`anna`, `boris`, `vera`,
`dmitry`): любая правка, меняющая внешность пользователей, роняет тест — обновлять снапшот
(`npx vitest run -u`) только осознанно.
