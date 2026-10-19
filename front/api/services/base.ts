import type { Api } from '../generated/hbc';

/**
 * All services share one generated `Api` instance (configured axios: baseURL, auth
 * header, interceptors) created by `getApi()`.
 */
export abstract class BaseService {
  constructor(protected readonly api: Api<null>) {}
}

/** Drops `UNSPECIFIED` enum values and empty strings so they don't end up in the query. */
export const cleanQuery = <T extends Record<string, unknown>>(query: T): Partial<T> => {
  const out: Partial<T> = {};
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue;
    if (typeof value === 'string' && value.endsWith('_UNSPECIFIED')) continue;
    (out as Record<string, unknown>)[key] = value;
  }
  return out;
};
