import js from '@eslint/js';
import boundaries from 'eslint-plugin-boundaries';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

/**
 * The boundaries plugin is the frontend's answer to go-arch-lint: it is what
 * stops a route reaching into another route's internals, or a shared library
 * importing a feature. Without it "feature-sliced" is a folder naming
 * convention, not a constraint.
 */
export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'coverage'] },

  js.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,

  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      globals: { ...globals.browser },
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      boundaries,
    },
    settings: {
      // Without a resolver the plugin cannot follow the `@/` alias, so every
      // aliased import looks like an unknown external and no rule fires. That
      // failure mode is silent: lint passes and the boundary is unguarded.
      'import/resolver': {
        typescript: { project: './tsconfig.app.json' },
      },
      'boundaries/include': ['src/**/*'],
      // The composition root and the ambient type declaration belong to no
      // slice, exactly like cmd/ on the Go side: their whole job is to wire
      // the slices together, so no inward rule can apply to them.
      'boundaries/ignore': ['src/main.tsx', 'src/vite-env.d.ts'],
      // mode: 'file' matters. The default matches an element's *folder*, so a
      // file sitting directly in src/test/ matched nothing and every boundary
      // rule silently skipped it.
      'boundaries/elements': [
        { type: 'app', pattern: 'src/app/**/*', mode: 'file' },
        { type: 'routes', pattern: 'src/routes/**/*', mode: 'file' },
        { type: 'pages', pattern: 'src/pages/**/*', mode: 'file' },
        { type: 'features', pattern: 'src/features/**/*', mode: 'file' },
        { type: 'components', pattern: 'src/components/**/*', mode: 'file' },
        { type: 'api', pattern: 'src/api/**/*', mode: 'file' },
        { type: 'stores', pattern: 'src/stores/**/*', mode: 'file' },
        { type: 'hooks', pattern: 'src/hooks/**/*', mode: 'file' },
        { type: 'lib', pattern: 'src/lib/**/*', mode: 'file' },
        { type: 'i18n', pattern: 'src/i18n/**/*', mode: 'file' },
        { type: 'types', pattern: 'src/types/**/*', mode: 'file' },
        { type: 'test', pattern: 'src/test/**/*', mode: 'file' },
      ],
    },
    rules: {
      ...reactHooks.configs.recommended.rules,

      // Direction is one-way: app composes routes/pages, pages use features,
      // features use components/api/stores/lib.
      'boundaries/no-unknown-files': 'error',
      'boundaries/no-unknown': 'error',
      'boundaries/element-types': [
        'error',
        {
          default: 'disallow',
          rules: [
            { from: 'app', allow: ['routes', 'pages', 'features', 'components', 'api', 'stores', 'hooks', 'lib', 'i18n', 'types'] },
            { from: 'routes', allow: ['pages', 'features', 'components', 'api', 'stores', 'hooks', 'lib', 'i18n', 'types'] },
            { from: 'pages', allow: ['features', 'components', 'api', 'stores', 'hooks', 'lib', 'i18n', 'types'] },
            { from: 'features', allow: ['features', 'components', 'api', 'stores', 'hooks', 'lib', 'i18n', 'types'] },
            { from: 'components', allow: ['lib', 'i18n', 'types'] },
            { from: 'api', allow: ['lib', 'types'] },
            { from: 'stores', allow: ['lib', 'types'] },
            { from: 'hooks', allow: ['stores', 'lib', 'api', 'types'] },
            { from: 'lib', allow: ['lib', 'types'] },
            { from: 'i18n', allow: ['i18n', 'types'] },
            { from: 'types', allow: ['types'] },
            { from: 'test', allow: ['api', 'lib', 'components', 'routes', 'pages', 'features', 'stores', 'hooks', 'app', 'i18n', 'types', 'test'] },
          ],
        },
      ],

      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // TanStack Router uses thrown Redirect objects in beforeLoad navigation guards
      '@typescript-eslint/only-throw-error': [
        'error',
        { allow: ['Redirect'] },
      ],
      // The API client throws ApiError; floating promises hide failures from
      // both the user and the error boundary.
      '@typescript-eslint/no-floating-promises': 'error',
    },
  },

  // Build tooling runs in Node and is outside the app's tsconfig, so the
  // type-aware rules have no program to consult.
  {
    files: ['vite.config.ts', 'eslint.config.js', 'playwright.config.ts', 'scripts/**/*.mjs', 'e2e/**/*.ts'],
    ...tseslint.configs.disableTypeChecked,
    languageOptions: { globals: { ...globals.node } },
  },
);
