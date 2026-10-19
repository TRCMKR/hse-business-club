/**
 * Turning anything thrown by the API layer into a message ready for the user.
 *
 * Understands:
 * - AxiosError with a grpc-gateway body `{ code, message }` (SPEC §4.2: `message` is
 *   always ready to show);
 * - ofetch `FetchError` (`error.data` is the parsed body, `error.status`/`statusCode`);
 * - H3 / Nuxt `createError` (`statusCode`, `statusMessage`, `data.message`);
 * - the same wrapper made by `useAsyncData` around an API error: the original error is in
 *   `cause` and is used instead (a network failure is not «Внутренняя ошибка»);
 * - network errors without a response and timeouts;
 * - plain `Error` and strings.
 */
import { isAxiosError } from 'axios';

export const DEFAULT_ERROR_MESSAGE = 'Что-то пошло не так. Попробуйте ещё раз.';
export const NETWORK_ERROR_MESSAGE = 'Нет соединения с сервером. Проверьте интернет и попробуйте ещё раз.';
export const TIMEOUT_ERROR_MESSAGE = 'Сервер не ответил вовремя. Попробуйте ещё раз.';
export const INTERNAL_ERROR_MESSAGE = 'Внутренняя ошибка';

type AnyRecord = Record<string, unknown>;

function isRecord(value: unknown): value is AnyRecord {
  return typeof value === 'object' && value !== null;
}

function nonEmptyString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

/** Original error behind the H3 / Nuxt wrapper that `useAsyncData` puts around a rejection. */
function isApiCause(value: unknown): boolean {
  return isAxiosError(value) || (isRecord(value) && value.name === 'FetchError');
}

function unwrap(error: unknown): unknown {
  let current = error;
  for (let depth = 0; depth < 3 && isRecord(current); depth++) {
    const isWrapper = current.__nuxt_error === true
      || (current.constructor as { __h3_error__?: boolean } | undefined)?.__h3_error__ === true;
    if (!isWrapper || !isApiCause(current.cause)) {
      break;
    }
    current = current.cause;
  }
  return current;
}

/** Response body of the error, if any (axios `response.data`, ofetch/H3 `data`). */
function getErrorBody(error: unknown): unknown {
  if (isAxiosError(error)) {
    return error.response?.data;
  }
  if (isRecord(error)) {
    return error.data;
  }
  return undefined;
}

/** `message` from a grpc-gateway / Nitro error body; H3 nests the backend body in `data.data`. */
function getBodyMessage(body: unknown): string | undefined {
  if (typeof body === 'string') {
    // Plain-text bodies are usually HTML error pages from proxies — do not show them.
    return undefined;
  }
  if (!isRecord(body)) {
    return undefined;
  }
  return nonEmptyString(body.message)
    ?? (isRecord(body.data) ? nonEmptyString(body.data.message) : undefined);
}

function isTimeout(error: unknown): boolean {
  if (isAxiosError(error)) {
    return error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT';
  }
  return isRecord(error) && (error.name === 'TimeoutError'
    || (isRecord(error.cause) && error.cause.name === 'TimeoutError'));
}

function isNetworkError(error: unknown): boolean {
  if (isAxiosError(error)) {
    return !error.response;
  }
  if (!isRecord(error)) {
    return false;
  }
  // ofetch FetchError without a response (fetch() itself rejected).
  if (error.name === 'FetchError' && error.response === undefined && getErrorStatus(error) === undefined) {
    return true;
  }
  return error.name === 'TypeError' && typeof error.message === 'string'
    && /failed to fetch|network|load failed/i.test(error.message);
}

/** HTTP status of the error response, if there was one. */
export function getErrorStatus(rawError: unknown): number | undefined {
  const error = unwrap(rawError);
  if (isAxiosError(error)) {
    return error.response?.status;
  }
  if (!isRecord(error)) {
    return undefined;
  }
  for (const key of ['status', 'statusCode'] as const) {
    const value = error[key];
    if (typeof value === 'number' && value > 0) {
      return value;
    }
  }
  if (isRecord(error.response) && typeof error.response.status === 'number') {
    return error.response.status;
  }
  return undefined;
}

/** gRPC status code from the grpc-gateway body (`3` InvalidArgument, `6` AlreadyExists, …). */
export function getErrorCode(error: unknown): number | undefined {
  const body = getErrorBody(unwrap(error));
  if (!isRecord(body)) {
    return undefined;
  }
  if (typeof body.code === 'number') {
    return body.code;
  }
  if (isRecord(body.data) && typeof body.data.code === 'number') {
    return body.data.code;
  }
  return undefined;
}

/**
 * True when the error came from the API with an HTTP response (optionally with the given
 * status): `isApiError(e, 409)`.
 */
export function isApiError(error: unknown, status?: number): boolean {
  const actual = getErrorStatus(error);
  if (actual === undefined) {
    return false;
  }
  return status === undefined || actual === status;
}

/** A message ready to show to the user (toast, form error). Never throws. */
export function getErrorMessage(rawError: unknown, fallback: string = DEFAULT_ERROR_MESSAGE): string {
  const error = unwrap(rawError);
  if (typeof error === 'string') {
    return nonEmptyString(error) ?? fallback;
  }
  if (error === null || error === undefined) {
    return fallback;
  }

  const bodyMessage = getBodyMessage(getErrorBody(error));
  if (bodyMessage) {
    return bodyMessage;
  }

  if (isTimeout(error)) {
    return TIMEOUT_ERROR_MESSAGE;
  }
  if (isNetworkError(error)) {
    return NETWORK_ERROR_MESSAGE;
  }

  const status = getErrorStatus(error);
  if (status !== undefined) {
    if (status >= 500) {
      return INTERNAL_ERROR_MESSAGE;
    }
    // H3 errors carry a human-readable statusMessage; axios/ofetch messages are technical.
    if (isRecord(error) && !isAxiosError(error) && error.name !== 'FetchError') {
      const statusMessage = nonEmptyString(error.statusMessage);
      if (statusMessage) {
        return statusMessage;
      }
    }
    return fallback;
  }

  if (isAxiosError(error)) {
    return fallback;
  }
  if (error instanceof Error) {
    return nonEmptyString(error.message) ?? fallback;
  }
  if (isRecord(error)) {
    return nonEmptyString(error.message) ?? fallback;
  }
  return fallback;
}

/**
 * `statusMessage` set by our pages (`createError({ statusCode: 404, statusMessage:
 * 'Публикация не найдена' })`) for the error page. Messages made by Nuxt / Nitro
 * themselves are English (`Page not found: /x`, `Internal Server Error`) — not shown.
 */
export function getUserStatusMessage(error: { statusMessage?: string }): string | undefined {
  const message = error.statusMessage?.trim();
  return message && /\p{Script=Cyrillic}/u.test(message) ? message : undefined;
}
