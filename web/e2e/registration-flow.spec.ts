import { test, expect } from "@playwright/test";
import { waitForEmail, extractOtpCode, clearMailbox } from "./helpers/mailpit";

test.describe("Registration & Email OTP Verification Journey", () => {
  test.beforeEach(async () => {
    await clearMailbox();
  });

  test("registers new learner, verifies email with Mailpit OTP, and reaches dashboard", async ({
    page,
  }) => {
    const timestamp = Date.now();
    const testEmail = `learner-e2e-${timestamp}@example.com`;
    const displayName = `Learner ${timestamp.toString().slice(-4)}`;
    const password = "Password123!@#";

    // 1. Navigate to Register page
    await page.goto("/register");
    await expect(page.locator("h1")).toHaveText(/Create your account/i);

    // 2. Fill registration form
    await page.getByLabel(/Display name/i).fill(displayName);
    await page.getByLabel(/Email address/i).fill(testEmail);
    await page.getByLabel(/^Password$/i).fill(password);
    await page.getByLabel(/Confirm password/i).fill(password);

    // 3. Submit registration
    await page.getByRole("button", { name: /Create account/i }).click();

    // 4. Verification screen appears
    await expect(page.getByRole("heading", { name: /Verify your email/i })).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByText(testEmail)).toBeVisible();

    // 5. Read OTP from real Mailpit email
    const email = await waitForEmail(testEmail, 15000);
    const otpCode = extractOtpCode(email.Text || email.HTML || "");
    expect(otpCode).toHaveLength(6);

    // 6. Enter the 6-digit OTP
    const otpInputs = page.locator('input[inputmode="numeric"]');
    await expect(otpInputs.first()).toBeVisible();

    // Fill each digit into segmented inputs
    for (let i = 0; i < 6; i++) {
      await otpInputs.nth(i).fill(otpCode[i]!);
    }

    // 7. Click confirm / verify button
    await page.getByRole("button", { name: /Verify Email/i }).click();

    // 8. Reaches authenticated dashboard
    await expect(page).toHaveURL("/", { timeout: 10000 });
    await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
    await expect(page.getByText(displayName)).toBeVisible();
  });
});
