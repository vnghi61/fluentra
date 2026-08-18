import { test, expect } from "@playwright/test";

test.describe("Journey 4: Google Sign-In Linking to Verified Existing Account", () => {
  test("links Google identity to existing verified learner account", async ({
    page,
  }) => {
    // Intercept Google callback endpoint returning linked verified user
    await page.route("**/api/v1/auth/oauth/google/callback", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "44444444-4444-4444-4444-444444444444",
            email: "verified-learner@example.com",
            display_name: "Verified Learner",
            role: "user",
            avatar_url: null,
          },
          session_id: "sess-google-verified-123",
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        }),
      });
    });

    await page.goto("/auth/callback/google?code=link-code&state=link-state");

    await expect(page).toHaveURL("/", { timeout: 10000 });
    await expect(page.getByText(/Verified Learner/i)).toBeVisible();
  });
});
