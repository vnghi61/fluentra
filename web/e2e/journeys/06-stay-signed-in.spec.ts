import { test, expect } from "@playwright/test";

test.describe("Journey 6: Stay Signed In (Silent Refresh on Boot)", () => {
  test("reopening application boots straight to dashboard with zero login flash", async ({
    browser,
  }) => {
    // 1. Create context with authenticated storage state / refresh cookies
    const context = await browser.newContext();

    // Mock silent refresh endpoint returning valid user session
    await context.route("**/api/v1/auth/refresh", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "66666666-6666-6666-6666-666666666666",
            email: "persisted-learner@example.com",
            display_name: "Persisted Learner",
            role: "user",
            avatar_url: null,
          },
          session_id: "sess-persisted-123",
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        }),
      });
    });

    const page = await context.newPage();

    // Track all navigated URLs to prove login screen never flashes
    const visitedUrls: string[] = [];
    page.on("framenavigated", (frame) => {
      if (frame === page.mainFrame()) {
        visitedUrls.push(frame.url());
      }
    });

    // 2. Open root page directly
    await page.goto("/");

    // 3. User is immediately on dashboard
    await expect(page).toHaveURL("/", { timeout: 10000 });
    await expect(page.getByText(/Persisted Learner/i)).toBeVisible();

    // 4. Assert login route was NEVER visited during boot resolution
    expect(visitedUrls.some((url) => url.includes("/login"))).toBe(false);

    await context.close();
  });
});
