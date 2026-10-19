/**
 * Builds gallery.html: a 10x10 grid (demo accounts, every shape, every accent,
 * random seeds) plus a palette readability block at 96 and 24 px. The page also renders
 * every avatar again in the browser and reports whether it matches the Node output.
 *
 *   npm run gallery               # writes gallery.html
 *   npm run screenshot            # also writes docs/gallery.png via headless Chrome
 */
import { execFileSync } from 'node:child_process';
import { existsSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { build } from 'vite';
import { ACCENTS, blobAvatar, createRng, DETAILS, PALETTES, SHAPES } from '../src/index';
import type { BlobAvatarOptions } from '../src/index';

const root = resolve(import.meta.dirname, '..');
const output = resolve(root, 'gallery.html');
const screenshot = resolve(root, 'docs/gallery.png');

interface Cell {
  seed: string;
  label: string;
  options: BlobAvatarOptions;
}

function randomSeeds(count: number): string[] {
  const rng = createRng(20261020);
  const syllables = ['ka', 'ri', 'mo', 'le', 'sa', 'tu', 'vi', 'na', 'go', 'de', 'zo', 'pe', 'lu', 'xi', 'ar', 'en'];
  const seeds: string[] = [];
  for (let i = 0; i < count; i++) {
    let name = '';
    const parts = rng.int(2, 3);
    for (let j = 0; j < parts; j++) {
      name += rng.pick(syllables);
    }
    if (rng.next() < 0.4) {
      name += `_${rng.int(1, 99)}`;
    }
    seeds.push(name);
  }
  return seeds;
}

const mainGrid: Cell[] = [
  ...['anna', 'boris', 'vera', 'dmitry'].map((seed) => ({ seed, label: seed, options: {} })),
  ...SHAPES.map((shape) => ({ seed: `shape-${shape}`, label: shape, options: { traits: { shape } } })),
  ...ACCENTS.map((accent) => ({ seed: `accent-${accent}`, label: accent, options: { traits: { accent } } })),
  ...DETAILS.map((detail) => ({ seed: `detail-${detail}`, label: detail, options: { traits: { detail } } })),
];
for (const seed of randomSeeds(100 - mainGrid.length)) {
  mainGrid.push({ seed, label: seed, options: {} });
}

/** Every palette once large and with every shape small, to check contrast at both sizes. */
const paletteRows: { name: string; large: Cell; small: Cell[] }[] = PALETTES.map((palette, index) => ({
  name: palette.name,
  large: { seed: `palette-${index}`, label: palette.name, options: { traits: { palette: index } } },
  small: SHAPES.map((shape, i) => ({
    seed: `palette-${index}-${i}`,
    label: '',
    options: { traits: { palette: index, shape, accent: ACCENTS[(index + i) % ACCENTS.length] } },
  })),
}));

async function browserBundle(): Promise<string> {
  const result = await build({
    configFile: false,
    logLevel: 'silent',
    build: {
      write: false,
      minify: false,
      lib: { entry: resolve(root, 'src/index.ts'), formats: ['iife'], name: 'BlobAvatarLib' },
    },
  });
  const first = Array.isArray(result) ? result[0] : result;
  const chunk = first && 'output' in first ? first.output[0] : undefined;
  if (!chunk || chunk.type !== 'chunk') {
    throw new Error('failed to bundle src/index.ts');
  }
  return chunk.code.replace(/<\/script/gi, '<\\/script');
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

/** Node-rendered SVGs, embedded as JSON so the browser can compare strings byte for byte. */
const expected: { seed: string; options: BlobAvatarOptions; svg: string }[] = [];

function avatarHtml(cell: Cell, size: number, background: BlobAvatarOptions['background'] = 'circle'): string {
  const options = { ...cell.options, size, background };
  const svg = blobAvatar(cell.seed, options);
  expected.push({ seed: cell.seed, options, svg });
  return `<span class="avatar" data-index="${expected.length - 1}">${svg}</span>`;
}

function cellHtml(cell: Cell, background: BlobAvatarOptions['background'] = 'circle'): string {
  return `<figure class="cell">${avatarHtml(cell, 72, background)}<figcaption>${escapeHtml(cell.label)}</figcaption></figure>`;
}

function paletteHtml(row: (typeof paletteRows)[number]): string {
  const small = row.small.map((cell) => avatarHtml(cell, 24)).join('');
  return `<figure class="palette">${avatarHtml(row.large, 96)}<div class="small">${small}</div><figcaption>${escapeHtml(row.name)}</figcaption></figure>`;
}

const bundle = await browserBundle();
const sections = {
  main: mainGrid.map((cell) => cellHtml(cell)).join(''),
  palettes: paletteRows.map(paletteHtml).join(''),
  backgrounds: (['none', 'circle', 'squircle', 'square'] as const)
    .flatMap((bg) => ['anna', 'boris'].map((seed) => cellHtml({ seed, label: `${seed} ${bg}`, options: {} }, bg)))
    .join(''),
};
const expectedJson = JSON.stringify(expected).replace(/</g, '\\u003c');
const html = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<title>@fe/blob-avatar — галерея</title>
<style>
  body { margin: 0; padding: 28px 32px; font: 13px/1.4 -apple-system, system-ui, sans-serif; background: #fafafa; color: #222; width: 1000px; }
  h1 { font-size: 20px; margin: 0 0 4px; }
  h2 { font-size: 15px; margin: 28px 0 10px; }
  p { margin: 0; color: #666; }
  .grid { display: grid; grid-template-columns: repeat(10, 1fr); gap: 10px 8px; }
  .cell { margin: 0; display: flex; flex-direction: column; align-items: center; }
  .cell figcaption { font-size: 11px; color: #777; margin-top: 2px; max-width: 92px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .avatar { display: inline-flex; }
  .palettes { display: grid; grid-template-columns: repeat(6, 1fr); gap: 16px 8px; }
  .palette { margin: 0; display: flex; flex-direction: column; align-items: center; gap: 6px; }
  .palette .small { display: flex; gap: 4px; }
  .palette figcaption { font-size: 11px; color: #777; }
  #check { margin-top: 6px; font-weight: 600; }
  #check.ok { color: #1a7f37; } #check.fail { color: #c62828; }
  .fail svg, .avatar.fail svg { outline: 2px solid #c62828; }
</style>
</head>
<body>
<h1>@fe/blob-avatar</h1>
<p>Детерминированные аватары из строки: пастельная фигура и акцент. Демо-аккаунты, все фигуры, акценты, детали и случайные seed'ы.</p>
<p id="check">Проверка в браузере…</p>
<h2>10 × 10: anna, boris, vera, dmitry · 6 фигур · 6 акцентов · 3 детали · случайные</h2>
<div class="grid">${sections.main}</div>
<h2>Палитры: 96 px и 24 px (все фигуры)</h2>
<div class="palettes">${sections.palettes}</div>
<h2>Фоны: none · circle · squircle · square</h2>
<div class="grid">${sections.backgrounds}</div>
<script>${bundle}</script>
<script>
  const expected = ${expectedJson};
  let mismatches = 0;
  expected.forEach((item, index) => {
    if (BlobAvatarLib.blobAvatar(item.seed, item.options) !== item.svg) {
      mismatches++;
      document.querySelector('[data-index="' + index + '"]').classList.add('fail');
    }
  });
  const check = document.getElementById('check');
  check.textContent = mismatches === 0
    ? 'Браузер: все ' + expected.length + ' SVG совпадают с Node байт в байт'
    : 'Браузер: ' + mismatches + ' из ' + expected.length + ' SVG отличаются от Node';
  check.className = mismatches === 0 ? 'ok' : 'fail';
</script>
</body>
</html>
`;

writeFileSync(output, html);
console.log(`gallery: ${output}`);

if (process.argv.includes('--screenshot')) {
  const chrome = process.env.CHROME_PATH ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  if (!existsSync(chrome)) {
    throw new Error(`Chrome not found at ${chrome}; set CHROME_PATH`);
  }
  execFileSync(chrome, [
    '--headless=new',
    '--disable-gpu',
    '--hide-scrollbars',
    '--force-device-scale-factor=1',
    `--screenshot=${screenshot}`,
    '--window-size=1064,1720',
    '--virtual-time-budget=2000',
    pathToFileURL(output).href,
  ], { stdio: 'inherit' });
  console.log(`screenshot: ${screenshot}`);
}
