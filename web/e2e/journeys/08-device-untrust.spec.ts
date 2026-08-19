import { expect, test } from "@playwright/test";

import { newLearner, registerAndVerify, signIn, signOut } from "../helpers/auth";

/**
 * Journey 8 runs against the real API. The previous version mocked
 * `/api/v1/me/devices`, which is not an endpoint this application has — the
 * device list is `/api/v1/auth/devices` — so the mock never matched and the
 * journey asserted against a screen the app never rendered.
 */
test.describe("Journey 8: device list → untrust the current device", () => {
  test("warns before untrusting the device being read on, then signs the learner out", async ({
    page,
  }) => {
    const learner = newLearner("j8");
    await registerAndVerify(page, learner);

    // A trusted device only exists once somebody signs in with "stay signed
    // in" ticked, so the journey has to go through a real sign-in to have a
    // row to act on at all.
    await signOut(page);
    await signIn(page, learner, { rememberDevice: true });

    await page.goto("/settings");
    await page.getByRole("button", { name: /Security & Devices/i }).click();

    // The server marks the caller's own device `current`; that flag is what the
    // warning branch keys off, so asserting it here is asserting the thing the
    // warning depends on.
    await expect(page.getByText("This device")).toBeVisible({ timeout: 15_000 });

    await page.getByRole("button", { name: /Untrust device/i }).first().click();

    await expect(
      page.getByRole("heading", { name: /Stop trusting this device\?/i }),
    ).toBeVisible();
    await expect(
      page.getByText(/sign you out of this browser immediately/i),
    ).toBeVisible();

    // The warning has to be true, not merely present: untrusting the current
    // device ends the session, and the app must land on the login screen.
    await page.getByRole("button", { name: /Untrust & Sign Out/i }).click();
    await expect(page).toHaveURL(/\/login/, { timeout: 15_000 });
  });
});
