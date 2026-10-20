// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  modules: [
    '@nuxt/eslint',
    '@nuxt/fonts',
    '@nuxt/test-utils/module',
    '@pinia/nuxt',
  ],

  ssr: true,

  // Components are imported explicitly (named exports), no auto-registration.
  components: { dirs: [] },

  devtools: {
    enabled: true,
  },

  app: {
    head: {
      htmlAttrs: { lang: 'ru' },
      title: 'Бизнес-клуб ВШЭ — стартапы и вакансии',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
        // Matches --color-bg of each theme (assets/css/main.css).
        { name: 'theme-color', content: '#f4f6fb', media: '(prefers-color-scheme: light)' },
        { name: 'theme-color', content: '#06081c', media: '(prefers-color-scheme: dark)' },
      ],
    },
  },

  css: ['~/assets/css/main.css'],

  runtimeConfig: {
    // Server-only. Overridden by NUXT_API_BASE_URL / NUXT_MOCK_API at runtime.
    apiBaseUrl: 'http://localhost:8080',
    mockApi: false,
  },

  experimental: {
    decorators: true,
  },

  compatibilityDate: '2025-07-15',

  vite: {
    resolve: {
      // Local @fe/* packages must share the app's Vue instance.
      dedupe: ['vue'],
    },
  },

  eslint: {
    config: {
      stylistic: true,
      formatters: true,
      tooling: true,
      typescript: true,
    },
  },

  fonts: {
    defaults: {
      weights: [400, 500, 600, 700],
      subsets: ['cyrillic', 'latin'],
    },
    families: [
      { name: 'Montserrat', provider: 'google' },
    ],
  },
});
