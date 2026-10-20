import { forwardToBackend } from '../utils/proxy';

/**
 * BFF proxy: /api/v1/** → ${apiBaseUrl}/v1/** with `Authorization: Bearer <hbc_access>`,
 * refreshing the session once on a 401 (see `server/utils/proxy.ts`).
 */
export default defineEventHandler(event => forwardToBackend(event, useRuntimeConfig(event)));
