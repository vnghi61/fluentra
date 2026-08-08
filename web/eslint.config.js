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
  { ignores: ['dist', 'node_modules', 'src/types/api.ts', 'coverage'] },

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
        { type: 'components', pattern: 'src/components/**/*', mode: 'file' },
        { type: 'api', pattern: 'src/api/**/*', mode: 'file' },
        { type: 'lib', pattern: 'src/lib/**/*', mode: 'file' },
        { type: 'i18n', pattern: 'src/i18n/**/*', mode: 'file' },
        { type: 'test', pattern: 'src/test/**/*', mode: 'file' },
      ],
    },
    rules: {
      ...reactHooks.configs.recommended.rules,

      // Direction is one-way: app composes routes, routes use components and
      // api, everything may use lib. Nothing lower reaches back up, which is
      // what keeps a shared component from depending on one screen's data.
      // Loud about anything the element patterns fail to classify: an
      // unclassified file is one the boundary rules silently skip.
      'boundaries/no-unknown-files': 'error',
      'boundaries/no-unknown': 'error',
      'boundaries/element-types': [
        'error',
        {
          default: 'disallow',
          rules: [
            { from: 'app', allow: ['routes', 'components', 'api', 'lib', 'i18n'] },
            { from: 'routes', allow: ['components', 'api', 'lib', 'i18n'] },
            { from: 'components', allow: ['lib', 'i18n'] },
            { from: 'api', allow: ['lib'] },
            { from: 'lib', allow: ['lib'] },
            { from: 'i18n', allow: ['i18n'] },
            { from: 'test', allow: ['api', 'lib', 'components', 'routes', 'app', 'i18n', 'test'] },
          ],
        },
      ],

      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // The API client throws ApiError; floating promises hide failures from
      // both the user and the error boundary.
      '@typescript-eslint/no-floating-promises': 'error',
    },
  },

  // Build tooling runs in Node and is outside the app's tsconfig, so the
  // type-aware rules have no program to consult.
  {
    files: ['vite.config.ts', 'eslint.config.js', 'scripts/**/*.mjs'],
    ...tseslint.configs.disableTypeChecked,
    languageOptions: { globals: { ...globals.node } },
  },
);
