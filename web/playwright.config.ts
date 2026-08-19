import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright E2E configuration for Fluentra web.
 *
 * Part A Task 1 sets up one desktop project (1280x800).
 * P5.3b adds mobile-ios, mobile-android, and tablet device projects.
 *
 * The four device projects run every journey in `e2e/` except `e2e/google/`.
 * Those three drive real Google, need a person at the consent screen, and would
 * need external network in CI — see e2e/google/README.md. They live behind
 * E2E_GOOGLE=1 so they are opt-in rather than skipped: a project that is not
 * selected reports nothing, whereas a skipped test reports a pass it did not
 * earn.
 */
const googleManual = process.env.E2E_GOOGLE === "1";

/**
 * The E2E dev server runs on its own port, beside `make dev` rather than on top
 * of it.
 *
 * 5173 belongs to the web container, and two things went wrong when the tests
 * shared it: Playwright could not bind the port and reported "Timed out waiting
 * from config.webServer", and when it did reuse the container's server every
 * `page.goto` crawled — Vite inside the container reads the source over a
 * Windows bind mount, which is slow enough to blow a 30 s navigation timeout.
 * A host-run Vite on 5174 reads the same files natively and proxies to the same
 * containerised API.
 *
 * 127.0.0.1 rather than localhost: on Windows `localhost` resolves to ::1 first
 * and the server listens on the IPv4 loopback.
 */
const E2E_PORT = process.env.E2E_PORT ?? "5174";
const E2E_ORIGIN = process.env.E2E_BASE_URL ?? `http://127.0.0.1:${E2E_PORT}`;

// The device projects run the journeys. The manual Google folder and the
// 320 px responsive spec each belong to exactly one project, so they are
// ignored here rather than run four more times at the wrong width.
const testIgnore = ["google/**", "responsive/**"];

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  ...(process.env.CI ? { workers: 1 } : {}),
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    // 127.0.0.1, not localhost: on Windows `localhost` resolves to ::1 first,
    // and the dev server — whether the container from `make dev` or a host
    // `pnpm dev` — is published on the IPv4 loopback only. The reuse check then
    // fails, Playwright tries to start a second Vite on a port already held,
    // and the run dies with "Timed out waiting from config.webServer".
    baseURL: E2E_ORIGIN,
    // Retries are 0 (P5.4 asks for zero flakes, and a retry hides exactly the
    // flake that acceptance is about), so "on-first-retry" would never fire and
    // no trace would ever be written. "retain-on-failure" is what actually
    // produces the artefact the card asks to keep.
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "desktop",
      testIgnore,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 800 },
      },
    },
    {
      name: "mobile-ios",
      testIgnore,
      use: {
        ...devices["iPhone 13"],
      },
    },
    {
      name: "mobile-android",
      testIgnore,
      use: {
        ...devices["Pixel 7"],
      },
    },
    {
      name: "tablet",
      testIgnore,
      use: {
        ...devices["iPad (gen 7)"],
        // P5.3b specifies a 768×1024 tablet, not the 810×1080 of the iPad gen 7
        // descriptor. The viewport is pinned so the "no horizontal scroll at
        // 320px" matrix covers the width the plan names.
        viewport: { width: 768, height: 1024 },
      },
    },
    // R6 is "no horizontal scroll at 320 px", and none of the four device
    // projects above is 320 px wide — so until this one existed the rule was
    // documented and unenforced. It runs the responsive spec only.
    {
      name: "narrow-320",
      testMatch: "responsive/**",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 320, height: 640 },
        isMobile: false,
      },
    },
    // Opt-in, and headed: a person has to complete Google's consent screen.
    ...(googleManual
      ? [
          {
            name: "google-manual",
            testMatch: "google/**",
            timeout: 240_000,
            use: {
              ...devices["Desktop Chrome"],
              viewport: { width: 1280, height: 800 },
              headless: false,
            },
          },
        ]
      : []),
  ],
  webServer: {
    // `pnpm exec vite`, not `pnpm run dev -- …`: the `--` form does not forward
    // the flags reliably through pnpm, and the server then starts on 5173 —
    // where the container already is — so Playwright waits for a port nothing
    // will ever answer on.
    command: `pnpm exec vite --port ${E2E_PORT} --strictPort`,
    url: E2E_ORIGIN,
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000,
  },
});
