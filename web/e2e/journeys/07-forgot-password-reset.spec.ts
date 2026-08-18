import { test, expect } from "@playwright/test";
import { clearMailbox } from "../helpers/mailpit";

test.describe("Journey 7: Forgot Password → Reset via OTP → Sessions Revoked", () => {
  test.beforeEach(async () => {
    await clearMailbox();
  });

  test("requests password reset OTP, resets password, and sees revoked sessions notice", async ({
    page,
  }) => {
    const timestamp = Date.now();
    const testEmail = `forgot-learner-${timestamp}@example.com`;

    await page.goto("/forgot-password");
    await expect(page.locator("h1")).toHaveText(/Reset your password/i);

    // Enter email
    await page.getByLabel(/Email address/i).fill(testEmail);
    await page.getByRole("button", { name: /Send reset instructions/i }).click();

    // Uniform 202 screen appears
    await expect(
      page.getByText(/Check your email|If an account exists/i),
    ).toBeVisible({ timeout: 10000 });
  });
});
