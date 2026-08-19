import { expect, test } from "@playwright/test";

import { newLearner, registerAndVerify } from "../helpers/auth";
import { waitForEmail } from "../helpers/mailpit";

/**
 * Journey 10 runs against the real API and the real worker. The previous
 * version mocked `GET /api/v1/me/export`, an endpoint whose real shape is
 * `/me/export/{id}`, and asserted on a `download_url` and a `ready` status that
 * are not in the schema — so it proved the mock matched itself.
 *
 * What the card asks for is the round trip: request → the worker builds the
 * archive → the learner is told by email.
 */
test.describe("Journey 10: GDPR data export", () => {
  test("requests an archive and the worker delivers the ready email", async ({
    page,
  }) => {
    const learner = newLearner("j10");
    await registerAndVerify(page, learner);

    await page.goto("/settings");
    await page.getByRole("button", { name: /Data & Privacy/i }).click();

    await expect(
      page.getByRole("heading", { name: /Export Your Personal Data/i }),
    ).toBeVisible();

    await page.getByRole("button", { name: /Request Data Export/i }).click();

    await expect(page.getByRole("status")).toContainText(
      /Data export requested/i,
      { timeout: 15_000 },
    );

    // The archive is built by the worker out of the outbox, so this is also
    // what proves the worker is running and reaching MinIO. Without the bucket
    // the job fails and no message ever arrives.
    // Matched by subject rather than by clearing the inbox: the registration
    // OTP for this same address is already sitting in it, and a global clear
    // would take other workers' messages with it.
    const message = await waitForEmail(learner.email, 60_000, 500, /export/i);
    expect(message.Subject).toMatch(/export/i);
  });
});
