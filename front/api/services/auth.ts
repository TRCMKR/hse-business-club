import { ContentType } from '../generated/hbc';
import type { LoginInput, RegisterInput, User } from '../types';
import { BaseService } from './base';

/**
 * Auth goes through the Nitro routes `/api/auth/*` (not the `/v1/auth/*` proxy):
 * they keep the access and refresh tokens in httpOnly cookies and return only `{ user }`.
 * Client-only: never call these during SSR.
 */
export class AuthService extends BaseService {
  async login(input: LoginInput): Promise<User> {
    const { data } = await this.api.request<{ user: User }>({
      path: '/auth/login',
      method: 'POST',
      body: input,
      type: ContentType.Json,
      format: 'json',
    });
    return data.user;
  }

  async register(input: RegisterInput): Promise<User> {
    const { data } = await this.api.request<{ user: User }>({
      path: '/auth/register',
      method: 'POST',
      body: input,
      type: ContentType.Json,
      format: 'json',
    });
    return data.user;
  }

  async logout(): Promise<void> {
    await this.api.request({ path: '/auth/logout', method: 'POST' });
  }
}
