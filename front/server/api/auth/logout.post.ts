export default defineEventHandler((event) => {
  clearAuthToken(event);
  return {};
});
