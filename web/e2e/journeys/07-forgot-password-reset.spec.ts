import { expect, test } from "@playwright/test";

import {
  enterOtp,
  newLearner,
  newPassword,
  registerAndVerify,
  signIn,
  signOut,
} from "../helpers/auth";
import { extractOtpCode, waitForEmail } from "../helpers/mailpit";

/**
 * Journey 7 against the real stack.
 *
 * The previous version never created an account: it typed an address nobody had
 * registered and asserted the uniform "check your email" screen, which the
 * server returns whether or not the address exists. That proved the anti-
 * enumeration response and nothing the card asks for.
 *
 * What the card asks for is the consequence: after a reset, the sessions and
 * devices that existed before it are dead.
 */
test.describe("Journey 7: forgot → reset → old sessions and devices dead", () => {
  test("resets the password by OTP, reports the revoked sessions, and kills the old browser", async ({
    browser,
  }) => {
    // Two browsers, a registration, a sign-in, a reset and two emails. On
    // WebKit that is past the default 30 s.
    test.slow();

    const learner = newLearner("j7");

    // The "old" browser: signed in and trusted, so there is a session and a
    // device for the reset to invalidate.
    const oldContext = await browser.newContext();
    const oldPage = await oldContext.newPage();
    await registerAndVerify(oldPage, learner);
    await signOut(oldPage);
    await signIn(oldPage, learner, { rememberDevice: true });

    // A second browser does the reset, the way somebody locked out would.
    const resetContext = await browser.newContext();
    const resetPage = await resetContext.newPage();

    await resetPage.goto("/forgot-password");
    await expect(
      resetPage.getByRole("heading", { name: /Reset your password/i }),
    ).toBeVisible();

    await resetPage.getByLabel(/Email address/i).fill(learner.email);
    await resetPage
      .getByRole("button", { name: /Send recovery code/i })
      .click();

    // Matched by subject so the registration code already in the inbox is not
    // mistaken for the recovery one.
    const message = await waitForEmail(learner.email, 30_000, 500, /reset|recover|password/i);
    await enterOtp(resetPage, extractOtpCode(message.Text || message.HTML || ""));

    const replacement = newPassword();
    await resetPage.getByLabel(/New Password/i).fill(replacement);
    await resetPage.getByRole("button", { name: /Reset password/i }).click();

    await expect(
      resetPage.getByRole("heading", { name: /Password reset complete/i }),
    ).toBeVisible({ timeout: 15_000 });

    // The card's actual requirement: the reset says how many sessions it ended,
    // and it has to be a real count, not a reassuring sentence.
    await expect(resetPage.getByRole("status")).toContainText(
      /signed out across your devices/i,
    );

    // And the old browser is genuinely dead: its refresh token no longer works,
    // so a reload lands on the login screen rather than the dashboard.
    await oldPage.reload();
    await expect(oldPage).toHaveURL(/\/login/, { timeout: 15_000 });

    // The replacement password is the one that works now.
    await signIn(resetPage, { ...learner, password: replacement });

    await oldContext.close();
    await resetContext.close();
  });
});
