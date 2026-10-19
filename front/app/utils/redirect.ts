/**
 * Sanitizes the `?redirect=` query value: only same-origin absolute paths are allowed
 * (no `//host`, no `http:`), otherwise falls back to `/`.
 */
export const safeRedirect = (value: unknown, fallback = '/'): string => {
  const raw = Array.isArray(value) ? value[0] : value;
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) {
    return fallback;
  }
  if (raw === '/login' || raw.startsWith('/login?') || raw === '/register' || raw.startsWith('/register?')) {
    return fallback;
  }
  return raw;
};
