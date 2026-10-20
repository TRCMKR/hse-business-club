import { proxyAuth } from '../../utils/backend-auth';

export default defineEventHandler(event => proxyAuth(event, useRuntimeConfig(event), '/v1/auth/login'));
