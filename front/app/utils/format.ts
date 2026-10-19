/** Telegram handle without a leading `@` (as stored by the backend). */
function normalizeTelegram(handle: string | null | undefined): string {
  return (handle ?? '').trim().replace(/^@+/, '');
}

/** `displayTelegram('anna')` → '@anna'; empty → ''. */
export function displayTelegram(handle: string | null | undefined): string {
  const h = normalizeTelegram(handle);
  return h ? `@${h}` : '';
}

/** `telegramUrl('anna')` → 'https://t.me/anna'; empty → null. */
export function telegramUrl(handle: string | null | undefined): string | null {
  const h = normalizeTelegram(handle);
  return h ? `https://t.me/${encodeURIComponent(h)}` : null;
}
