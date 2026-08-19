import { test } from "@playwright/test";

import {
  expectSignedIn,
  newLearner,
  registerAndVerify,
} from "../helpers/auth";

test.describe("Journey 1: Register → OTP → Dashboard", () => {
  test("registers, reads the OTP from Mailpit, verifies, and lands on the dashboard", async ({
    page,
  }) => {
    const learner = newLearner("j1");

    await registerAndVerify(page, learner);

    // AppShell renders the role and a sign-out control, never the display name,
    // so "signed in" is asserted on the control that only exists when signed in.
    await expectSignedIn(page);
  });
});
