import type { BackendCallRequest, BackendCallResponse, BackendContext } from '~~/api/backend-context';
import { BACKEND_CONTEXT_KEY } from '~~/api/backend-context';
import { hasAuthSession } from '../utils/auth-cookie';
import { backendFetch } from '../utils/backend';
import { withRefresh } from '../utils/session-refresh';

const SSR_TIMEOUT_MS = 10_000;

const toResponse = async (res: Response): Promise<BackendCallResponse> => {
  const headers = Object.fromEntries(res.headers.entries());
  const text = await res.text();
  let data: unknown = text;
  if ((headers['content-type'] ?? '').includes('json') && text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  return { status: res.status, statusText: res.statusText, headers, data };
};

/**
 * Gives SSR code (`api/client.ts`) a backend caller bound to the current request:
 * the access token comes from the cookies, a 401 is refreshed once (new cookies land on
 * the SSR response) and all calls of one render share the refreshed pair.
 */
export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('request', (event) => {
    const config = useRuntimeConfig(event);
    const context: BackendContext = {
      hasSession: () => hasAuthSession(event),
      request: async (req: BackendCallRequest) => {
        const url = new URL(req.url, 'http://backend.invalid');
        const res = await withRefresh(event, config, access => backendFetch(config, {
          method: req.method,
          path: url.pathname,
          search: url.search,
          headers: { accept: 'application/json', ...req.headers },
          body: req.body,
          timeoutMs: SSR_TIMEOUT_MS,
        }, access));
        return toResponse(res);
      },
    };
    event.context[BACKEND_CONTEXT_KEY] = context;
  });
});
