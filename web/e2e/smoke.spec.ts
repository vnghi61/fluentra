import { test, expect } from "@playwright/test";

test.describe("App Smoke Test", () => {
  test("loads the root page and renders the application header", async ({
    page,
  }) => {
    await page.goto("/");
    await expect(page.locator("h1")).toHaveText(/Fluentra/i);
  });
});
