import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import type { ProxyOptions } from 'vite';
import { defineConfig } from 'vitest/config';

const apiProxy: ProxyOptions = {
  target: process.env.VITE_API_TARGET || 'http://localhost:8080',
  changeOrigin: true,
  // Vite answers a refused connection with a bare 500, which the client reads
  // as "the server answered with an error" and never treats as a cold start.
  // A stopped API is a gateway failure: say 503, as Render's proxy does, so
  // src/api/wake.ts shows the waking and unreachable states. Registered here,
  // before Vite's own handler, which skips a response already sent.
  configure: (proxy) => {
    proxy.on('error', (_err, _req, res) => {
      if ('writeHead' in res && !res.headersSent && !res.writableEnded) {
        res.writeHead(503, { 'Content-Type': 'text/plain' });
        res.end('API unavailable');
      }
    });
  },
};

/**
 * The object store, reached through the dev server from a phone on the LAN.
 * See src/lib/storage-url.ts. `changeOrigin` sends the store the host the API
 * signed the URL for, and the prefix is stripped so the signed path is intact.
 */
const storageProxy: ProxyOptions = {
  target: process.env.VITE_STORAGE_TARGET || 'http://localhost:9000',
  changeOrigin: true,
  rewrite: (path) => path.replace(/^\/__storage/, ''),
};

/**
 * HTTPS for testing on a phone. A browser opens the microphone only on a secure
 * origin — https, or localhost — so over `http://192.168.x.x:5173` recording
 * cannot start at all. Point these at a certificate for the LAN address (mkcert
 * makes one) and the dev server serves HTTPS. Unset, it stays plain HTTP.
 */
function devHttps() {
  const cert = process.env.VITE_DEV_HTTPS_CERT;
  const key = process.env.VITE_DEV_HTTPS_KEY;
  if (!cert || !key) return {};
  return { https: { cert: readFileSync(cert), key: readFileSync(key) } };
}

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
    allowedHosts: true,
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
    ...devHttps(),
    proxy: {
      '/api': apiProxy,
      '/__storage': storageProxy,
    },
  },
  // `vite preview` serves the built bundle, and it needs the same API proxy the
  // dev server has or every request 404s against the static server. The E2E
  // suite runs against this rather than against `vite dev`: WebKit spends ~25s
  // per navigation transforming modules on demand, which is most of a 30s test
  // timeout before the journey has done anything.
  preview: {
    host: '0.0.0.0',
    ...devHttps(),
    proxy: {
      '/api': apiProxy,
      '/__storage': storageProxy,
    },
  },
  build: {
    // The manifest is what scripts/check-bundle.mjs walks to find the chunks a
    // first visit actually downloads.
    manifest: true,
    chunkSizeWarningLimit: INITIAL_CHUNK_BUDGET_KB,
    rollupOptions: {
      output: {
        // React and the TanStack libraries change rarely and stay cached across
        // deploys; splitting them keeps a content change from invalidating
        // them. A function, not a name list: `react-dom` as a name matched only
        // its index, so `react-dom/client` and `scheduler` (most of React DOM)
        // landed in the entry chunk instead.
        manualChunks(id) {
          if (/\/node_modules\/(?:react|react-dom|scheduler)\//.test(id)) {
            return 'react';
          }
          if (id.includes('/node_modules/@tanstack/')) {
            return 'tanstack';
          }
          return undefined;
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
