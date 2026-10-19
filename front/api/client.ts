import axios, { AxiosError } from 'axios';
import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import { Api } from './generated/hbc';
import { AUTH_COOKIE } from './constants';
import { ApplicationsService } from './services/applications';
import { AuthService } from './services/auth';
import { ListingsService } from './services/listings';
import { UsersService } from './services/users';

type UnauthorizedHandler = () => void;

let unauthorizedHandler: UnauthorizedHandler | null = null;

/** Registered by `plugins/api.client.ts`: clears the store, shows a toast, redirects to /login. */
export const setUnauthorizedHandler = (handler: UnauthorizedHandler | null) => {
  unauthorizedHandler = handler;
};

/**
 * SSR in mock mode: there is no backend, so route requests in-process through the
 * Nitro proxy (`/api/v1/**` → mock) with the incoming request's cookies.
 */
const createLocalAdapter = (fetcher: ReturnType<typeof useRequestFetch>): AxiosAdapter => {
  return async (config: InternalAxiosRequestConfig) => {
    const url = axios.getUri(config);
    const res = await fetcher.raw(url, {
      method: (config.method ?? 'get').toUpperCase() as 'GET',
      body: config.data,
      headers: config.data ? { 'content-type': 'application/json' } : undefined,
      ignoreResponseError: true,
      responseType: 'json',
    });
    const response: AxiosResponse = {
      data: res._data,
      status: res.status,
      statusText: res.statusText,
      headers: Object.fromEntries(res.headers.entries()),
      config,
      request: {},
    };
    if (!config.validateStatus || config.validateStatus(res.status)) {
      return response;
    }
    throw new AxiosError(
      `Request failed with status code ${res.status}`,
      res.status >= 500 ? AxiosError.ERR_BAD_RESPONSE : AxiosError.ERR_BAD_REQUEST,
      config,
      null,
      response,
    );
  };
};

const createServerApi = () => {
  const config = useRuntimeConfig();
  const token = useCookie(AUTH_COOKIE).value;

  if (config.mockApi) {
    return new Api<null>({
      baseURL: '/api',
      timeout: 10_000,
      adapter: createLocalAdapter(useRequestFetch()),
    });
  }

  return new Api<null>({
    baseURL: config.apiBaseUrl,
    timeout: 10_000,
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
};

const createClientApi = () => {
  const api = new Api<null>({ baseURL: '/api', timeout: 15_000 });

  api.instance.interceptors.response.use(
    response => response,
    (error: unknown) => {
      const url = axios.isAxiosError(error) ? error.config?.url ?? '' : '';
      // 401 from /auth/login is "wrong password", not an expired session.
      if (axios.isAxiosError(error) && error.response?.status === 401 && !url.startsWith('/auth/')) {
        unauthorizedHandler?.();
      }
      return Promise.reject(error);
    },
  );

  return api;
};

const createServices = (api: Api<null>) => ({
  auth: new AuthService(api),
  users: new UsersService(api),
  listings: new ListingsService(api),
  applications: new ApplicationsService(api),
});

export type ApiServices = ReturnType<typeof createServices>;

let clientServices: ApiServices | null = null;

/**
 * Entry point for all API calls.
 *
 * - Client: `baseURL = '/api'` (Nitro proxy adds `Authorization` from the httpOnly cookie);
 *   one shared instance.
 * - Server (SSR): `baseURL = runtimeConfig.apiBaseUrl` + `Authorization` from the request
 *   cookie; a new instance per call — never cached between SSR requests.
 *
 * On the server it must be called synchronously inside a Nuxt context (setup, plugin,
 * middleware) — call it at the top of `setup()` and use the result inside `useAsyncData`.
 */
export const getApi = (): ApiServices => {
  if (import.meta.server) {
    return createServices(createServerApi());
  }
  if (!clientServices) {
    clientServices = createServices(createClientApi());
  }
  return clientServices;
};
