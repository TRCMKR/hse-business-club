export default defineEventHandler(event => proxyAuth(event, '/v1/auth/login'));
