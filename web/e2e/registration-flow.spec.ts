import { test } from "@playwright/test";

import {
  expectSignedIn,
  newLearner,
  registerAndVerify,
} from "./helpers/auth";

/**
 * The Stage 4 proving journey: it exists to fail loudly when the E2E job has no
 * real backend behind it. It registers through the UI, reads the code out of
 * Mailpit and signs in — none of which can pass against a dev-server proxy
 * answering ECONNREFUSED, which is how this job used to go green with nothing
 * running.
 */
test.describe("Registration & Email OTP Verification Journey", () => {
  test("registers a learner, verifies with the Mailpit OTP, and reaches the dashboard", async ({
    page,
  }) => {
    const learner = newLearner("proving");

    await registerAndVerify(page, learner);

    await expectSignedIn(page);
  });
});
