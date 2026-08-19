import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

/**
 * Warning threshold for a single minified chunk. This is advisory only — Vite
 * has no way to fail a build on size. The enforced budget lives in
 * scripts/check-bundle.mjs and runs as part of `pnpm build`.
 */
const INITIAL_CHUNK_BUDGET_KB = 400;

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    // A bind-mounted source tree does not deliver inotify events to the
    // container on Windows or macOS, so Vite never learns a file changed and
    // keeps serving the module it transformed at startup. The app then looks
    // stale in the browser while the file on disk is right, and the usual
    // conclusion is that the edit was wrong. Polling is the cost of noticing.
    //
    // Off by default: a host-run `pnpm dev` gets real filesystem events and
    // should not pay for a poll loop. compose.dev.yaml switches it on.
    //
    // Scoped, because polling costs what it watches: an unbounded poll over the
    // mounted tree kept a core busy and slowed the container enough to blow
    // navigation timeouts. Only `src` changes need to be noticed this way.
    ...(process.env.VITE_USE_POLLING === 'true'
      ? {
          watch: {
            usePolling: true,
            interval: 1000,
            ignored: [
              '**/node_modules/**',
              '**/.git/**',
              '**/dist/**',
              '**/test-results/**',
              '**/playwright-report/**',
              '**/.pnpm-store/**',
              '**/e2e/**',
            ],
          },
        }
      : {}),
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  // `vite preview` serves the built bundle, and it needs the same API proxy the
  // dev server has or every request 404s against the static server. The E2E
  // suite runs against this rather than against `vite dev`: WebKit spends ~25s
  // per navigation transforming modules on demand, which is most of a 30s test
  // timeout before the journey has done anything.
  preview: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    // The manifest is what scripts/check-bundle.mjs walks to find the chunks a
    // first visit actually downloads.
    manifest: true,
    chunkSizeWarningLimit: INITIAL_CHUNK_BUDGET_KB,
    rollupOptions: {
      output: {
        manualChunks: {
          // React and the router change rarely and are cached across deploys;
          // splitting them keeps a content change from invalidating them.
          react: ['react', 'react-dom'],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    exclude: ['e2e/**', 'node_modules/**'],
    pool: 'threads',
    fileParallelism: false,
    css: false,
  },
});
