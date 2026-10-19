import { defineVitestConfig } from '@nuxt/test-utils/config';

export default defineVitestConfig({
  test: {
    environment: 'nuxt',
    environmentOptions: {
      nuxt: {
        domEnvironment: 'jsdom',
      },
    },
    include: ['tests/**/*.test.ts', 'app/**/*.test.ts', 'server/**/*.test.ts'],
    globals: true,
  },
});
