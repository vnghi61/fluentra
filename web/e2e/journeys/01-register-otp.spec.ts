import { test, expect } from "@playwright/test";
import { waitForEmail, extractOtpCode, clearMailbox } from "../helpers/mailpit";

test.describe("Journey 1: Register → OTP → Dashboard", () => {
  test.beforeEach(async () => {
    await clearMailbox();
  });

  test("registers, reads OTP from Mailpit, verifies, and lands on dashboard", async ({
    page,
  }) => {
    const timestamp = Date.now();
    const testEmail = `learner-j1-${timestamp}@example.com`;
    const displayName = `Learner J1`;
    const password = "Password123!@#";

    await page.goto("/register");
    await page.getByLabel(/Display name/i).fill(displayName);
    await page.getByLabel(/Email address/i).fill(testEmail);
    await page.getByLabel(/^Password$/i).fill(password);
    await page.getByLabel(/Confirm password/i).fill(password);
    await page.getByRole("button", { name: /Create account/i }).click();

    await expect(page.getByRole("heading", { name: /Verify your email/i })).toBeVisible();

    const email = await waitForEmail(testEmail, 15000);
    const otpCode = extractOtpCode(email.Text || email.HTML || "");

    const otpInputs = page.locator('input[inputmode="numeric"]');
    for (let i = 0; i < 6; i++) {
      await otpInputs.nth(i).fill(otpCode[i]!);
    }

    await page.getByRole("button", { name: /Verify Email/i }).click();

    await expect(page).toHaveURL("/", { timeout: 10000 });
    await expect(page.getByText(displayName)).toBeVisible();
  });
});
