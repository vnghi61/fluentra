import { expect, test } from "@playwright/test";

import {
  newLearner,
  promoteToAdmin,
  registerAndVerify,
  signIn,
  signOut,
} from "../helpers/auth";

/**
 * Journey 9 against the real stack.
 *
 * The card names this one as impossible to fake: an admin suspension has to
 * change what the learner's *next* request returns, and a browser-level mock
 * can only change what the mock returns. The previous version stubbed
 * `/admin/users`, `/admin/users/{id}` and the suspend call, so it asserted that
 * three fixtures agreed with each other.
 *
 * Here the admin is a real account promoted through db/seeds/rbac.sql, the
 * suspension is a real 200, and the learner's own browser is the witness.
 */
test.describe("Journey 9: admin suspends a learner → the learner is locked out", () => {
  test("suspends with an audited reason and the learner's next request is refused", async ({
    browser,
  }) => {
    // Two accounts registered and verified end to end, a role grant through
    // psql, and a second sign-in to pick the role up. Slow on purpose, not
    // slow by accident.
    test.slow();

    const admin = newLearner("j9-admin");
    const learner = newLearner("j9-learner");

    // The learner, signed in and staying that way.
    const learnerContext = await browser.newContext();
    const learnerPage = await learnerContext.newPage();
    await registerAndVerify(learnerPage, learner);

    // The admin: registered like anybody, then granted the role out of band.
    const adminContext = await browser.newContext();
    const adminPage = await adminContext.newPage();
    await registerAndVerify(adminPage, admin);
    promoteToAdmin(admin.email);

    // The role is carried in the access token, so it takes a fresh sign-in to
    // pick it up — which is also what proves the grant reached the database.
    await signOut(adminPage);
    await signIn(adminPage, admin);

    await adminPage.goto("/admin");
    await adminPage
      .getByPlaceholder(/Search by name or email/i)
      .fill(learner.email);
    // The list does not search as you type; the button is the trigger.
    await adminPage.getByRole("button", { name: /^Search$/i }).click();
    await expect(adminPage.getByText(learner.email)).toBeVisible({
      timeout: 15_000,
    });

    await adminPage.getByRole("button", { name: /Inspect/i }).first().click();
    await adminPage.getByRole("button", { name: /Suspend User/i }).click();

    // The server enforces a ten-character minimum and answers 422 below it; the
    // form is expected to refuse first.
    await adminPage.getByLabel(/Audit Reason/i).fill("too short");
    await adminPage.getByRole("button", { name: /Confirm Suspension/i }).click();
    await expect(
      adminPage.getByText(/at least 10 characters/i),
    ).toBeVisible();

    await adminPage
      .getByLabel(/Audit Reason/i)
      .fill("Repeated abuse reports from other learners");
    await adminPage.getByRole("button", { name: /Confirm Suspension/i }).click();

    // The enforcement, in the learner's own browser: the next request their app
    // makes is refused, and the session is gone.
    //
    // Asserted through a guarded route rather than through the landing page.
    // This used to reload `/` and expect `/login`, which stopped being the
    // signed-out destination when ADR-0025 opened the curriculum — a suspended
    // learner now lands on the catalogue, the same as any visitor, because the
    // catalogue is public and the frontend has no way to know they were
    // suspended rather than simply signed out.
    //
    // `/progress` is built from the caller's own data and still refuses them,
    // which is the thing this journey is actually about: the session no longer
    // opens anything that belongs to a person.
    await expect(async () => {
      await learnerPage.goto("/progress");
      await expect(learnerPage).toHaveURL(/\/login/, { timeout: 5_000 });
    }).toPass({ timeout: 30_000 });

    // And the chrome agrees: no account menu, because there is no account.
    await expect(
      learnerPage.getByRole("button", { name: /Account/i }),
    ).toHaveCount(0);

    await learnerContext.close();
    await adminContext.close();
  });
});
