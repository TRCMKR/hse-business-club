import type { ProfileInput, User } from '../types';
import type { V1V1UploadAvatarRequest } from '../generated/hbc';
import { BaseService } from './base';

/** Reads a File as base64 without the `data:...;base64,` prefix. */
export const fileToBase64 = async (file: Blob): Promise<string> => {
  const buffer = new Uint8Array(await file.arrayBuffer());
  let binary = '';
  const chunk = 0x8000;
  for (let i = 0; i < buffer.length; i += chunk) {
    binary += String.fromCharCode(...buffer.subarray(i, i + chunk));
  }
  return btoa(binary);
};

export class UsersService extends BaseService {
  /** GET /v1/me */
  async me(): Promise<User> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1GetMe();
    return data as User;
  }

  /** PUT /v1/me — full replacement of editable fields. */
  async updateMe(input: ProfileInput): Promise<User> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1UpdateMe(input);
    return data as User;
  }

  /** GET /v1/users/{id} */
  async get(id: string): Promise<User> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1GetUser(id);
    return data as User;
  }

  /** POST /v1/me/avatar — `{ data: base64 }`; returns the updated profile with a new `avatarUrl`. */
  async uploadAvatar(file: File | Blob): Promise<User> {
    const body = { data: await fileToBase64(file) } as unknown as V1V1UploadAvatarRequest;
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1UploadAvatar(body, { timeout: 60_000 });
    return data as User;
  }

  /** DELETE /v1/me/avatar — idempotent. */
  async deleteAvatar(): Promise<User> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1DeleteAvatar();
    return data as User;
  }
}
