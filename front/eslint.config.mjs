// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs';
import stylistic from '@stylistic/eslint-plugin';

export default withNuxt(
  stylistic.configs.recommended,
  {
    // files: ['app/**/*'],
    ignores: [
      '.nuxt/',
      'api/generated/',
      'node_modules/',
    ],
    rules: {
      '@stylistic/jsx-curly-brace-presence': ['error', 'always'],
      '@stylistic/indent': ['error', 2],
      '@stylistic/jsx-indent-props': ['error', 2],
      '@stylistic/no-tabs': ['off'],
      '@stylistic/block-spacing': ['error'],
      '@stylistic/semi': ['error', 'always'],
      '@stylistic/brace-style': ['error', '1tbs'],
      'unicorn/throw-new-error': 'off',
      '@typescript-eslint/no-unused-vars': 'off',
      'no-unused-vars': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
);
