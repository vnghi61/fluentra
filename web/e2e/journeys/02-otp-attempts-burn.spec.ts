import { expect, test } from "@playwright/test";

import { enterOtp, newLearner } from "../helpers/auth";
import { extractOtpCode, waitForEmail } from "../helpers/mailpit";

/**
 * Journey 2 against the real stack.
 *
 * Two things about this flow are not what the card's one-line summary suggests,
 * and the journey asserts what the system actually does:
 *
 *  - The screen submits on its own once the sixth digit lands, so there is no
 *    button to click, and the boxes are NOT cleared after a refusal. Each wrong
 *    attempt therefore has to be a different code, or filling the same value
 *    changes nothing and never re-submits.
 *  - Once burned, resend is refused — `Challenge.Usable` returns
 *    ErrChallengeAttemptsExceeded before the cooldown is even consulted, and the
 *    UI disables the control and says to start over. So the recovery path is a
 *    fresh challenge, not a resend on the dead one. "burned -> resend" in the
 *    card is "burned -> get a new code" in the product.
 */
test.describe("Journey 2: OTP wrong 5× → burned → fresh code → success", () => {
  test("burns the challenge after five refusals and recovers with a new one", async ({
    page,
  }) => {
    // Genuinely long: six verifications, two registrations and two emails read
    // out of Mailpit. On WebKit that exceeds the default 30 s and the run is
    // killed mid-keystroke, which reads as a browser crash rather than as a
    // journey that needs more time.
    test.slow();

    const learner = newLearner("j2");

    await page.goto("/register");
    await page.getByLabel(/Display name/i).fill(learner.displayName);
    await page.getByLabel(/Email address/i).fill(learner.email);
    await page.getByLabel(/^Password/i).fill(learner.password);
    await page.getByRole("button", { name: /Create account/i }).click();

    await expect(
      page.getByRole("heading", { name: /Enter verification code/i }),
    ).toBeVisible({ timeout: 15_000 });

    const first = await waitForEmail(learner.email, 30_000);
    const realCode = extractOtpCode(first.Text || first.HTML || "");

    // Five codes that are all wrong and all different from each other, derived
    // from the real one so a coincidence cannot make an attempt succeed.
    const wrongCodes = Array.from({ length: 5 }, (_, index) =>
      String((Number(realCode) + index + 1) % 1_000_000).padStart(6, "0"),
    );

    await expect(page.getByText(/Attempts left: 5\/5/i)).toBeVisible();

    for (const [index, wrong] of wrongCodes.entries()) {
      await enterOtp(page, wrong);

      // The count is server-reported, so watching it fall is what proves the
      // attempt reached the challenge. Asserting the exact remaining number —
      // rather than that some counter is on screen — is what caught five
      // attempts being collapsed into one.
      const remaining = 4 - index;
      if (remaining > 0) {
        await expect(
          page.getByText(new RegExp(`Attempts left: ${remaining}/5`)),
        ).toBeVisible({ timeout: 15_000 });
      }
    }

    await expect(
      page.getByText(/burned after 5 incorrect attempts/i),
    ).toBeVisible({ timeout: 15_000 });

    // Resend is refused on a burned challenge, by the server and by the button.
    await expect(page.getByRole("button", { name: /Resend/i })).toBeDisabled();

    // Recovery: back to the form, register the same address again, and the
    // server issues a fresh challenge (the per-subject cap allows three an hour).
    const beforeSecondCode = new Date();
    await page.getByRole("button", { name: /Back/i }).click();
    await page.getByLabel(/Display name/i).fill(learner.displayName);
    await page.getByLabel(/Email address/i).fill(learner.email);
    await page.getByLabel(/^Password/i).fill(learner.password);
    await page.getByRole("button", { name: /Create account/i }).click();

    await expect(
      page.getByRole("heading", { name: /Enter verification code/i }),
    ).toBeVisible({ timeout: 15_000 });

    // Bounded by the moment the second registration was asked for, so the first
    // code — still in the shared inbox — cannot be read back as the new one.
    const second = await waitForEmail(
      learner.email,
      30_000,
      500,
      undefined,
      beforeSecondCode,
    );
    const freshCode = extractOtpCode(second.Text || second.HTML || "");
    expect(freshCode).not.toBe(realCode);

    await enterOtp(page, freshCode);
    await expect(page).toHaveURL("/", { timeout: 15_000 });
  });
});
