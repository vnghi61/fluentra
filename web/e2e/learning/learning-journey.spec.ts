import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

import {
  expectSignedIn,
  makeReviewsDue,
  newLearner,
  registerAndVerify,
} from "../helpers/auth";

/**
 * The Phase 2 learner journeys, against the real stack from `make dev`.
 *
 * Every selector here is a label read off the screen it appears on, and every
 * step asserts something that can be false. The first version of this file did
 * neither: it wrapped each interaction in `if (await x.isVisible())`, navigated
 * to `/lessons/:id` and `/review` — neither of which the router declares — and
 * asserted `page.locator("h1, h2").first()` was visible, which is true of an
 * error page. It passed without completing a single activity.
 */

const routes = {
  dashboard: "/",
  learn: "/learn",
  progress: "/progress",
  // The router declares /practice/review, not /review. A journey that navigates
  // to the wrong path still satisfies toHaveURL, because the URL is whatever was
  // asked for — which is how the first version "passed" this step on a 404.
  review: "/practice/review",
} as const;

/** Works through one activity of whichever of the three kinds is on screen. */
async function completeActivity(page: Page): Promise<void> {
  const flipCard = page.getByRole("button", { name: "Flip Card", exact: true });
  const gapInput = page.getByRole("textbox").first();
  const options = page.getByRole("radio");

  if (await flipCard.isVisible()) {
    // Flashcard: flip, say whether the word came back, then continue. The card
    // used to advance without submitting, which left the attempt the runner had
    // opened stuck in `in_progress` and kept the activity out of progress — so
    // this loop "finished" a three-activity lesson that counted as two.
    await flipCard.click();
    await page.getByRole("button", { name: "I knew it", exact: true }).click();
    await page.getByRole("button", { name: "Continue", exact: true }).click();
    return;
  }

  if ((await options.count()) > 0) {
    await options.first().click();
  } else {
    await expect(gapInput).toBeVisible();
    await gapInput.fill("habit");
  }

  await page.getByRole("button", { name: "Check Answer", exact: true }).click();
  await page.getByRole("button", { name: "Continue", exact: true }).click();
}

test.describe("Phase 2 learning journeys", () => {
  test("a learner enrols, finishes a lesson, and sees progress move", async ({ page }) => {
    const learner = newLearner("learn-road");

    await registerAndVerify(page, learner);
    await expectSignedIn(page);

    // 1. A brand-new learner is told what to do, not shown a zero.
    await expect(page.getByText("Start Your English Journey")).toBeVisible({
      timeout: 10_000,
    });

    // 2. The syllabus, reached from the dashboard rather than by typing a URL.
    await page.getByRole("link", { name: "Explore Syllabus" }).click();
    await expect(page).toHaveURL(/\/learn$/);

    const seededCourse = page.getByRole("heading", {
      name: /Everyday English: A2–B1 Foundations/,
    });
    await expect(seededCourse).toBeVisible({ timeout: 15_000 });

    // 3. Start the first lesson. The route carries the lesson id, so a click that
    //    went nowhere fails here rather than three steps later.
    await page.getByRole("link", { name: "Start Lesson" }).first().click();
    await expect(page).toHaveURL(/\/learn\/lesson\/[0-9a-f-]{36}$/, {
      timeout: 15_000,
    });

    // 4. Work through every activity in the lesson. The step counter is read from
    //    the runner rather than assumed, so a seed with a different lesson length
    //    changes the loop instead of breaking it.
    const counter = page.getByTestId("runner-step-counter");
    await expect(counter).toBeVisible({ timeout: 15_000 });
    const total = Number((await counter.getAttribute("data-total")) ?? "0");
    expect(total).toBeGreaterThan(0);

    for (let step = 0; step < total; step++) {
      await completeActivity(page);
    }

    // 5. The completion screen is the proof the lesson was finished.
    await expect(page.getByText("Lesson Completed!")).toBeVisible({ timeout: 15_000 });

    // 6. Progress has moved: the activities the learner just completed are counted.
    await page.goto(routes.progress);
    const completed = page.getByTestId("activities-completed");
    await expect(completed).toBeVisible({ timeout: 10_000 });
    expect(Number(await completed.innerText())).toBeGreaterThanOrEqual(total);

    // 7. Finishing a vocabulary activity schedules review cards, and FSRS puts a
    //    card answered correctly three days out. Measured against a live stack,
    //    not assumed: both of this lesson's cards landed on due_at = now + 3d,
    //    and a card answered wrong still landed ten minutes away. Nothing is due
    //    the instant the lesson ends, so the honest assertion is the queue's real
    //    empty state; demanding a full queue asserted a scheduler that does not
    //    exist here.
    await page.goto(routes.review);
    await expect(
      page.getByRole("heading", { name: /Nothing due right now/i }),
    ).toBeVisible({ timeout: 15_000 });
  });

  test("a learner clears the review queue with the keyboard alone", async ({ page }) => {
    const learner = newLearner("learn-review");

    await registerAndVerify(page, learner);
    await expectSignedIn(page);

    await page.getByRole("link", { name: "Explore Syllabus" }).click();
    await page.getByRole("link", { name: "Start Lesson" }).first().click();
    await expect(page).toHaveURL(/\/learn\/lesson\/[0-9a-f-]{36}$/, { timeout: 15_000 });

    const counter = page.getByTestId("runner-step-counter");
    await expect(counter).toBeVisible({ timeout: 15_000 });
    const total = Number((await counter.getAttribute("data-total")) ?? "0");
    for (let step = 0; step < total; step++) {
      await completeActivity(page);
    }

    // The lesson just scheduled this learner's cards, three days out. Nothing a
    // test can do inside its own run makes that time pass, so the clock moves
    // instead: `make due-reviews` brings the learner's own cards forward, the
    // same real path db/seeds/due_reviews.sql documents. Without it there is no
    // queue here at all, and P10.4's acceptance -- a full queue clears from the
    // keyboard -- has nothing to assert against.
    makeReviewsDue(learner.email);

    await page.goto(routes.review);
    const progress = page.getByTestId("review-progress");
    await expect(progress).toBeVisible({ timeout: 15_000 });

    // Space reveals, a digit grades. No mouse: P10.4's acceptance is that a full
    // queue clears from the keyboard.
    //
    // Each grade is a round trip, and the card only advances once it lands, so
    // the loop waits for the answer rather than firing four keypresses into a
    // screen still showing the previous card. Without the wait it graded one
    // card and dropped the rest on the floor -- and then failed at the summary,
    // three steps from the cause.
    const queue = Number((await progress.getAttribute("data-total")) ?? "0");
    expect(queue).toBeGreaterThan(0);
    for (let card = 0; card < queue; card++) {
      const graded = page.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          /\/api\/v1\/reviews\/[0-9a-f-]{36}\/answer$/.test(response.url()),
        { timeout: 15_000 },
      );
      await page.keyboard.press("Space");
      await page.keyboard.press("3");
      await graded;
    }

    await expect(page.getByText("Review Session Summary")).toBeVisible({
      timeout: 15_000,
    });
  });

  test("a brand-new learner reaches a dashboard that tells them what to do", async ({
    page,
  }) => {
    const learner = newLearner("learn-empty");

    await registerAndVerify(page, learner);
    await expectSignedIn(page);

    // Three cards, and not one of them a zero dressed as a statistic.
    await expect(page.getByText("Start Your English Journey")).toBeVisible();
    await expect(page.getByText("Reviews Due")).toBeVisible();
    await expect(page.getByText("Skill Progress")).toBeVisible();

    // No gamification anywhere: P10 §7 cut the streak, XP and achievements, and
    // a dashboard that grows one later should fail here first.
    await expect(page.getByText(/streak/i)).toHaveCount(0);
    await expect(page.getByText(/\bXP\b/)).toHaveCount(0);
    await expect(page.getByText(/achievement/i)).toHaveCount(0);

    // The empty state is a route to somewhere, not a dead end.
    await page.getByRole("link", { name: "Explore Syllabus" }).click();
    await expect(page).toHaveURL(/\/learn$/);
    await expect(
      page.getByRole("heading", { name: /Everyday English: A2–B1 Foundations/ }),
    ).toBeVisible({ timeout: 15_000 });
  });
});
