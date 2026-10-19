/**
 * Contract between the Nitro routes (server/api/[...].ts, server/api/auth/*) and the
 * in-memory mock API. Pure TS, no h3 — unit-testable without a server.
 */
export interface MockRequest {
  /** Upper-case HTTP method. */
  method: string
  /** Backend path without the `/api` prefix and without the query string: `/v1/listings/abc`. */
  path: string
  /** Parsed query string (single values). */
  query: Record<string, string | undefined>
  /** Bearer token from the cookie (in mock mode token = username), or null. */
  token: string | null
  /** Parsed JSON body (undefined for GET / empty body). */
  body: unknown
}

export interface MockResponse {
  status: number
  /** JSON-serializable object, or raw bytes (avatars). */
  body: unknown
  /** Extra response headers (e.g. Content-Type / Cache-Control for avatars). */
  headers?: Record<string, string>
}
