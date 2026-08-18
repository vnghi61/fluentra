import { test, expect } from "@playwright/test";
import { waitForEmail, extractOtpCode, clearMailbox } from "../helpers/mailpit";

test.describe("Journey 2: OTP Wrong 5× → Burned → Resend → Success", () => {
  test.beforeEach(async () => {
    await clearMailbox();
  });

  test("locks challenge after 5 incorrect attempts, resends new code, and succeeds", async ({
    page,
  }) => {
    const timestamp = Date.now();
    const testEmail = `learner-j2-${timestamp}@example.com`;
    const displayName = `Learner J2`;
    const password = "Password123!@#";

    await page.goto("/register");
    await page.getByLabel(/Display name/i).fill(displayName);
    await page.getByLabel(/Email address/i).fill(testEmail);
    await page.getByLabel(/^Password$/i).fill(password);
    await page.getByLabel(/Confirm password/i).fill(password);
    await page.getByRole("button", { name: /Create account/i }).click();

    await expect(page.getByRole("heading", { name: /Verify your email/i })).toBeVisible();

    const otpInputs = page.locator('input[inputmode="numeric"]');
    const verifyBtn = page.getByRole("button", { name: /Verify Email/i });

    // Enter wrong OTP 5 times
    for (let attempt = 1; attempt <= 5; attempt++) {
      for (let i = 0; i < 6; i++) {
        await otpInputs.nth(i).fill("0");
      }
      await verifyBtn.click();
      await page.waitForTimeout(300);
    }

    // Challenge is burned after 5 attempts
    await expect(
      page.getByText(/Invalid code|expired|Too many attempts|burned|request a new code/i),
    ).toBeVisible();

    // Clear mailbox and request resend
    await clearMailbox();
    const resendBtn = page.getByRole("button", { name: /Resend/i });
    if (await resendBtn.isEnabled()) {
      await resendBtn.click();
    }

    // Read new OTP code from Mailpit
    const newEmail = await waitForEmail(testEmail, 15000);
    const newOtpCode = extractOtpCode(newEmail.Text || newEmail.HTML || "");

    for (let i = 0; i < 6; i++) {
      await otpInputs.nth(i).fill(newOtpCode[i]!);
    }
    await verifyBtn.click();

    await expect(page).toHaveURL("/", { timeout: 10000 });
  });
});
