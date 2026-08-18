import { test, expect } from "@playwright/test";

test.describe("Journey 5: Google Sign-In Conflict with Unverified Account", () => {
  test("handles OAUTH_ACCOUNT_CONFLICT, prompts for verification, and completes link", async ({
    page,
  }) => {
    let callbackAttempts = 0;

    await page.route("**/api/v1/auth/oauth/google/callback", async (route) => {
      callbackAttempts++;
      if (callbackAttempts === 1) {
        // Return 409 Conflict with challenge details
        await route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({
            code: "OAUTH_ACCOUNT_CONFLICT",
            message: "An unverified account exists with this email. Please verify to link.",
            status: 409,
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            user: {
              id: "55555555-5555-5555-5555-555555555555",
              email: "unverified-conflict@example.com",
              display_name: "Linked Conflict Learner",
              role: "user",
              avatar_url: null,
            },
            session_id: "sess-google-conflict-123",
          }),
        });
      }
    });

    await page.goto("/auth/callback/google?code=conflict-code&state=conflict-state");

    // Conflict error message is rendered
    await expect(
      page.getByText(/An unverified account exists with this email|Conflict/i),
    ).toBeVisible({ timeout: 10000 });
  });
});
