import { defineConfig } from "@playwright/test";
import base from "./playwright.config";

// Review the user's running app only. Never start a second dev/preview server.
export default defineConfig({
  ...base,
  webServer: [],
  projects: [{ name: "dashboard-review", testMatch: "responsive/dashboard-layout.spec.ts", use: { browserName: "chromium" } }],
  use: { ...base.use, baseURL: process.env.E2E_BASE_URL ?? "http://localhost:5173" },
  workers: 1,
});
