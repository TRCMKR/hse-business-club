// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { canonicalProxyPath } from '../server/utils/proxy-path';

describe('canonicalProxyPath', () => {
  it.each([
    '/v1/auth/login',
    '/v1/auth/register',
    '/v1/auth/login/',
    '/v1//auth/login',
    '//v1/auth//login//',
    '/v1/auth/%6Cogin',
    '/v1/%61uth/login',
    '/v1/AUTH/login',
    '/v1/auth',
    '/v1/auth/',
    '/v1/x/../auth/login',
    '/v1/x/%2e%2e/auth/login',
    '/v1/auth%2Flogin',
    '/v1/listings/%E0%A4%A',
    '/v2/listings',
    '/',
    '',
    '/v1',
  ])('rejects %s', (raw) => {
    expect(canonicalProxyPath(raw)).toBeNull();
  });

  it.each([
    ['/v1/listings', '/v1/listings'],
    ['/v1/listings/', '/v1/listings'],
    ['/v1//me/applications/sent', '/v1/me/applications/sent'],
    ['/v1/listings/%61bc', '/v1/listings/abc'],
    ['/v1/listings/a%20b', '/v1/listings/a%20b'],
    ['/v1/listings/%256Cogin', '/v1/listings/%256Cogin'],
    ['/v1/avatars/0b1c.jpg', '/v1/avatars/0b1c.jpg'],
  ])('canonicalizes %s → %s', (raw, expected) => {
    expect(canonicalProxyPath(raw)).toBe(expected);
  });
});
