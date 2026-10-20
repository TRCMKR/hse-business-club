import { logoutSession } from '../../utils/backend-auth';

export default defineEventHandler(event => logoutSession(event, useRuntimeConfig(event)));
