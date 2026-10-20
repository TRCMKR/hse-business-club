/**
 * Contract between the Nitro side (`server/plugins/backend-context.ts`) and SSR code in
 * the app (`api/client.ts`, `plugins/auth.server.ts`). Types only: the app never imports
 * server code, so cookie names and tokens stay out of the client bundle.
 *
 * Every SSR request gets `event.context.hbc`; its `request()` calls the backend (or the
 * mock) with the access token from the cookies and transparently refreshes it once on a
 * 401 (see `server/utils/session-refresh.ts`).
 */
export interface BackendCallRequest {
  /** Upper-case HTTP method. */
  method: string
  /** Backend path with the query string: `/v1/listings?page=1`. */
  url: string
  /** Serialized body (JSON string) or undefined. */
  body?: string
  headers?: Record<string, string>
}

export interface BackendCallResponse {
  status: number
  statusText: string
  headers: Record<string, string>
  /** Parsed JSON when the response is JSON, otherwise text. */
  data: unknown
}

export interface BackendContext {
  /** True when the request carries an access or a refresh cookie. */
  hasSession: () => boolean
  request: (req: BackendCallRequest) => Promise<BackendCallResponse>
}

export const BACKEND_CONTEXT_KEY = 'hbc';
