/** httpOnly cookie with the JWT; set/cleared only by Nitro routes `server/api/auth/*`. */
export const AUTH_COOKIE = 'hbc_token';

/** 24h — same as the JWT lifetime (SPEC §4.2). */
export const AUTH_COOKIE_MAX_AGE = 60 * 60 * 24;
