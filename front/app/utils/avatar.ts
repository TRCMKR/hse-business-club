import type { User } from '~~/api/types';

/** Prefix of the Nitro proxy in front of the backend API (SPEC §5). */
export const API_PROXY_PREFIX = '/api';

/**
 * `<img src>` for the user's uploaded photo, or `null` when there is none
 * (then the generated blob avatar is shown). `avatarUrl` is relative to the API root
 * (`/v1/avatars/<uuid>.jpg`, SPEC §4.2), so the proxy prefix is prepended.
 */
export function avatarSrc(user: Pick<User, 'avatarUrl'> | null | undefined): string | null {
  return user?.avatarUrl ? API_PROXY_PREFIX + user.avatarUrl : null;
}
