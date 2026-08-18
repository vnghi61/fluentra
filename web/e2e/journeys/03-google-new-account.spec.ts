import { test, expect } from "@playwright/test";

test.describe("Journey 3: Google Sign-In (New Account)", () => {
  test("authenticates new Google user via callback and reaches dashboard", async ({
    page,
  }) => {
    // Intercept Google callback endpoint with test payload
    await page.route("**/api/v1/auth/oauth/google/callback", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "33333333-3333-3333-3333-333333333333",
            email: "google-new@example.com",
            display_name: "Google Learner",
            role: "user",
            avatar_url: null,
          },
          session_id: "sess-google-123",
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        }),
      });
    });

    // Navigate to Google OAuth callback route
    await page.goto("/auth/callback/google?code=valid-test-code&state=valid-test-state");

    // Reaches dashboard
    await expect(page).toHaveURL("/", { timeout: 10000 });
    await expect(page.getByText(/Google Learner/i)).toBeVisible();
  });
});
