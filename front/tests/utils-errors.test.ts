// @vitest-environment node
import { AxiosError, AxiosHeaders } from 'axios';
import type { AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';
import {
  DEFAULT_ERROR_MESSAGE,
  getErrorCode,
  getErrorMessage,
  getErrorStatus,
  INTERNAL_ERROR_MESSAGE,
  isApiError,
  NETWORK_ERROR_MESSAGE,
  TIMEOUT_ERROR_MESSAGE,
} from '~/utils/errors';

function axiosError(status?: number, data?: unknown, code?: string): AxiosError {
  const config = { headers: new AxiosHeaders() };
  const response: AxiosResponse | undefined = status === undefined
    ? undefined
    : { status, statusText: '', headers: {}, config, data };
  return new AxiosError('Request failed', code, config, {}, response);
}

describe('getErrorMessage', () => {
  it('uses grpc-gateway message from an AxiosError', () => {
    const error = axiosError(409, { code: 6, message: 'Вы уже откликались на эту публикацию', details: [] });
    expect(getErrorMessage(error)).toBe('Вы уже откликались на эту публикацию');
    expect(getErrorStatus(error)).toBe(409);
    expect(getErrorCode(error)).toBe(6);
    expect(isApiError(error)).toBe(true);
    expect(isApiError(error, 409)).toBe(true);
    expect(isApiError(error, 404)).toBe(false);
  });

  it('network error without response', () => {
    const error = axiosError(undefined, undefined, AxiosError.ERR_NETWORK);
    expect(getErrorMessage(error)).toBe(NETWORK_ERROR_MESSAGE);
    expect(getErrorStatus(error)).toBeUndefined();
    expect(isApiError(error)).toBe(false);
  });

  it('timeout', () => {
    expect(getErrorMessage(axiosError(undefined, undefined, AxiosError.ECONNABORTED))).toBe(TIMEOUT_ERROR_MESSAGE);
  });

  it('500 without a message', () => {
    expect(getErrorMessage(axiosError(500, '<html>Bad gateway</html>'))).toBe(INTERNAL_ERROR_MESSAGE);
  });

  it('4xx without a message → fallback', () => {
    expect(getErrorMessage(axiosError(400, {}))).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(axiosError(400, {}), 'Не удалось сохранить')).toBe('Не удалось сохранить');
  });

  it('ofetch FetchError-like', () => {
    const error = Object.assign(new Error('[POST] "/api/auth/login": 401'), {
      name: 'FetchError',
      status: 401,
      statusCode: 401,
      data: { code: 16, message: 'Неверный логин или пароль' },
    });
    expect(getErrorMessage(error)).toBe('Неверный логин или пароль');
    expect(getErrorCode(error)).toBe(16);
    expect(isApiError(error, 401)).toBe(true);
  });

  it('H3 / Nuxt createError-like', () => {
    expect(getErrorMessage({ statusCode: 404, statusMessage: 'Публикация не найдена' })).toBe('Публикация не найдена');
    expect(getErrorMessage({ statusCode: 409, data: { code: 6, message: 'Логин уже занят' } })).toBe('Логин уже занят');
  });

  it('plain Error and string', () => {
    expect(getErrorMessage(new Error('Файл слишком большой'))).toBe('Файл слишком большой');
    expect(getErrorMessage('Готово не будет')).toBe('Готово не будет');
  });

  it('unknown → fallback', () => {
    expect(getErrorMessage(undefined)).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(42)).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage({})).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage('')).toBe(DEFAULT_ERROR_MESSAGE);
  });
});
