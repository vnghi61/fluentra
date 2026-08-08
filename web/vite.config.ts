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
    css: false,
  },
});
