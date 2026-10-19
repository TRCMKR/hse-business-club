/** Client-side photo checks before upload (SPEC §2.3–2.4); the server re-checks by content. */

export const AVATAR_ACCEPT = 'image/jpeg,image/png,image/webp';
export const AVATAR_MIME_TYPES: readonly string[] = ['image/jpeg', 'image/png', 'image/webp'];
export const AVATAR_MAX_BYTES = 5 * 1024 * 1024;

export const AVATAR_TYPE_MESSAGE = 'Фото: поддерживаются JPEG, PNG и WebP';
export const AVATAR_SIZE_MESSAGE = 'Фото: не больше 5 МБ';

const EXTENSIONS = /\.(?:jpe?g|png|webp)$/i;

/**
 * Returns the error message for an unacceptable file or `null`. The MIME type is
 * checked first; an empty type (some mobile pickers) falls back to the extension.
 */
export function validateAvatarFile(file: Pick<File, 'name' | 'size' | 'type'>): string | null {
  const typeOk = file.type
    ? AVATAR_MIME_TYPES.includes(file.type.toLowerCase())
    : EXTENSIONS.test(file.name);
  if (!typeOk) {
    return AVATAR_TYPE_MESSAGE;
  }
  if (file.size > AVATAR_MAX_BYTES) {
    return AVATAR_SIZE_MESSAGE;
  }
  return null;
}
