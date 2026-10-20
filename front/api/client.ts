import axios, { AxiosError } from 'axios';
import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import { Api } from './generated/hbc';
import type { BackendCallResponse, BackendContext } from './backend-context';
import { BACKEND_CONTEXT_KEY } from './backend-context';
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
 * SSR: calls go through the backend caller Nitro attaches to the request
 * (`server/plugins/backend-context.ts`): it adds the access token from the httpOnly
 * cookies and refreshes the session once on a 401. In mock mode it serves the in-memory
 * mock in-process. Cookie names and tokens never appear in app code.
 */
const createContextAdapter = (backend: BackendContext | undefined): AxiosAdapter => {
  return async (config: InternalAxiosRequestConfig) => {
    if (!backend) {
      throw new AxiosError('Backend context is not available', AxiosError.ERR_NETWORK, config);
    }
    const headers: Record<string, string> = {};
    if (config.data !== undefined && config.data !== null) {
      headers['content-type'] = 'application/json';
    }
    let res: BackendCallResponse;
    try {
      res = await backend.request({
        method: (config.method ?? 'get').toUpperCase(),
        url: axios.getUri(config),
        body: typeof config.data === 'string' ? config.data : config.data == null ? undefined : JSON.stringify(config.data),
        headers,
      });
    } catch (cause) {
      const error = new AxiosError('Network Error', AxiosError.ERR_NETWORK, config);
      error.cause = cause;
      throw error;
    }
    const response: AxiosResponse = {
      data: res.data,
      status: res.status,
      statusText: res.statusText,
      headers: res.headers,
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
  const backend = useRequestEvent()?.context[BACKEND_CONTEXT_KEY] as BackendContext | undefined;
  return new Api<null>({
    baseURL: '',
    adapter: createContextAdapter(backend),
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
 * - Server (SSR): a new instance per call (never cached between SSR requests) whose adapter
 *   goes through the request-bound backend caller (cookies → `Authorization`, one refresh
 *   on 401, new cookies on the SSR response).
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
