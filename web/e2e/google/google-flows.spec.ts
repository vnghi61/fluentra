import { expect, test } from "@playwright/test";

import {
  enterOtp,
  expectSignedIn,
  newLearner,
  newPassword,
  registerAndVerify,
  signOut,
} from "../helpers/auth";
import { clearMailbox, extractOtpCode, waitForEmail } from "../helpers/mailpit";

/**
 * P5.4 journeys 3, 4 and 5, against real Google. See README.md in this folder
 * for why they are not in CI and how to run them.
 *
 * Each spec drives the app up to Google's consent screen and then waits for a
 * person to complete it. The wait is a plain assertion with a long timeout
 * rather than a `test.skip` or a `page.pause()`: if nobody completes the
 * consent the journey fails, which is the honest outcome.
 */

const CONSENT_TIMEOUT_MS = 180_000;

function googleAddress(): string {
  const address = process.env.E2E_GOOGLE_EMAIL;
  if (!address) {
    throw new Error(
      "E2E_GOOGLE_EMAIL is not set. It must be the address of the Google account you will sign in with — journeys 4 and 5 create a local account on that same address first.",
    );
  }
  return address;
}

async function startGoogleSignIn(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/login");
  await page.getByRole("button", { name: /Continue with Google/i }).click();
}

test.describe("Journey 3: Google sign-in, new account", () => {
  test("a Google account with no local match is signed in and lands on the dashboard", async ({
    page,
  }) => {
    await startGoogleSignIn(page);

    // Complete the consent screen by hand; the app finishes the rest.
    await expect(page).toHaveURL("/", { timeout: CONSENT_TIMEOUT_MS });
    await expectSignedIn(page);
  });
});

test.describe("Journey 4: Google sign-in links a verified local account", () => {
  test("an existing verified account on the same address is linked, not duplicated", async ({
    page,
  }) => {
    const learner = {
      ...newLearner("j4"),
      email: googleAddress(),
    };

    await registerAndVerify(page, learner);
    await signOut(page);
    await expect(page).toHaveURL(/\/login/);

    await startGoogleSignIn(page);

    await expect(page).toHaveURL("/", { timeout: CONSENT_TIMEOUT_MS });

    // The link is what distinguishes this from journey 3: the same account is
    // now reachable by both methods, so Connected Accounts shows Google.
    await page.goto("/settings");
    await page.getByRole("button", { name: /Security & Devices/i }).click();
    await expect(page.getByText(/Connected as/i)).toBeVisible({
      timeout: 15_000,
    });
  });
});

test.describe("Journey 5: Google sign-in against an unverified local account", () => {
  test("the conflict is refused, the address is verified by OTP, then the link completes", async ({
    page,
  }) => {
    const address = googleAddress();
    const password = newPassword();

    await clearMailbox();

    // Register but deliberately do NOT verify: an unverified local account is
    // exactly the case the server refuses to link silently.
    await page.goto("/register");
    await page.getByLabel(/Display name/i).fill("Conflict Learner");
    await page.getByLabel(/Email address/i).fill(address);
    await page.getByLabel(/^Password/i).fill(password);
    await page.getByRole("button", { name: /Create account/i }).click();
    await expect(
      page.getByRole("heading", { name: /Enter verification code/i }),
    ).toBeVisible();

    await startGoogleSignIn(page);

    // The server answers OAUTH_ACCOUNT_CONFLICT and the app asks for the code
    // rather than handing over an account nobody proved they own.
    await expect(
      page.getByRole("heading", { name: /Enter verification code/i }),
    ).toBeVisible({ timeout: CONSENT_TIMEOUT_MS });

    const message = await waitForEmail(address);
    await enterOtp(page, extractOtpCode(message.Text || message.HTML || ""));
    await page.getByRole("button", { name: /Verify & continue/i }).click();

    await expect(page).toHaveURL("/", { timeout: 30_000 });
  });
});
