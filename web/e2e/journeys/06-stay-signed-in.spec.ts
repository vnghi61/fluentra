import { expect, test } from "@playwright/test";

import {
  expectSignedIn,
  newLearner,
  registerAndVerify,
} from "../helpers/auth";

/**
 * Journey 6 proves the headline feature, so it runs against the real API: a
 * mocked `/auth/refresh` would prove only that the mock returns a user, which
 * is the one thing nobody doubts. What is under test is that the real refresh
 * cookie survives a fresh browser context and that boot resolves to the
 * dashboard without passing through the login screen.
 */
test.describe("Journey 6: Stay Signed In (silent refresh on boot)", () => {
  test("reopening the browser lands on the dashboard with no login screen in the route sequence", async ({
    browser,
  }) => {
    const context = await browser.newContext();
    const page = await context.newPage();

    const learner = newLearner("j6");
    await registerAndVerify(page, learner);

    // "Close the browser" without losing the cookie jar: the storage state is
    // what a returning learner actually has. A brand-new context would be a
    // different browser, which is a different requirement.
    const state = await context.storageState();
    await context.close();

    const reopened = await browser.newContext({ storageState: state });
    const reopenedPage = await reopened.newPage();

    // The card's trap: assert on the route sequence, not a screenshot. A login
    // screen that flashes for 200 ms and disappears still fails the
    // requirement, and a screenshot taken afterwards reports success.
    const resolvedRoutes: string[] = [];
    reopenedPage.on("framenavigated", (frame) => {
      if (frame === reopenedPage.mainFrame()) {
        resolvedRoutes.push(new URL(frame.url()).pathname);
      }
    });

    await reopenedPage.goto("/");

    await expect(reopenedPage).toHaveURL("/", { timeout: 15_000 });
    await expectSignedIn(reopenedPage);

    expect(
      resolvedRoutes,
      `boot passed through the login screen: ${resolvedRoutes.join(" -> ")}`,
    ).not.toContain("/login");

    await reopened.close();
  });
});
