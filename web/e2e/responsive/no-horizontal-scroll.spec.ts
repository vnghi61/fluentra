import { newPassword } from "../helpers/auth";
import { expect, test, type Page } from "@playwright/test";

import {
  stubAccountApi,
  stubAuthenticated,
  stubLearningApi,
  stubRegistration,
} from "../helpers/stubs";

/**
 * R6 — no horizontal scroll at 320 px (web/AGENT.md §6b, ADR-0024).
 *
 * This project is the only one at 320 px. Before it existed the rule was
 * documented as "enforced by the Playwright device matrix" while the narrowest
 * project in that matrix was 390 px, so nothing was checking it.
 *
 * R1 is checked alongside it, because at 320 px the two rules pull against each
 * other: a row of controls that fits only by shrinking below 44 px has traded
 * one rule for the other rather than satisfied both.
 */

/** Fails if the document is wider than the viewport. */
async function expectNoHorizontalScroll(page: Page, where: string): Promise<void> {
  const overflow = await page.evaluate(() => {
    const doc = document.documentElement;
    return {
      scrollWidth: doc.scrollWidth,
      clientWidth: doc.clientWidth,
      // Which elements actually push the page wide, to make a failure
      // diagnosable rather than a bare number. An element inside a container
      // that scrolls on its own is not the culprit — its overflow is contained
      // — so the walk stops at the first clipping ancestor.
      culprits: (() => {
        const limit = doc.clientWidth;
        const escapes = (element: Element): boolean => {
          let parent = element.parentElement;
          while (parent && parent !== doc) {
            const overflow = getComputedStyle(parent).overflowX;
            if (overflow === "auto" || overflow === "scroll" || overflow === "hidden") {
              return false;
            }
            parent = parent.parentElement;
          }
          return true;
        };
        return Array.from(document.body.querySelectorAll("*"))
          .filter((element) => element.getBoundingClientRect().right > limit + 1)
          .filter(escapes)
          .slice(0, 5)
          .map((element) => {
            const box = element.getBoundingClientRect();
            return `${element.tagName.toLowerCase()}.${(element.className || "").toString().split(" ").slice(0, 4).join(".")} [w=${Math.round(box.width)} right=${Math.round(box.right)}]`;
          });
      })(),
    };
  });

  expect(
    overflow.scrollWidth,
    `${where} scrolls horizontally at 320 px: document is ${overflow.scrollWidth}px wide against a ${overflow.clientWidth}px viewport. Elements escaping the viewport: ${overflow.culprits.join(" | ") || "none — the overflow is on a scroll container or the body itself"}.`,
  ).toBeLessThanOrEqual(overflow.clientWidth + 1);
}

/** Fails if any visible interactive control is under 44×44 CSS px. */
async function expectTouchTargets(page: Page, where: string): Promise<void> {
  const undersized = await page.evaluate(() => {
    const selector = 'button, a[href], input:not([type="hidden"]), select, textarea, [role="button"]';
    return Array.from(document.querySelectorAll(selector))
      .filter((element) => {
        const box = element.getBoundingClientRect();
        // Hidden controls have no hit area to be wrong about.
        if (box.width === 0 || box.height === 0) return false;
        // A visually-hidden input (Tailwind `sr-only`) is not a target: the
        // styled element beside it is, and that one is measured on its own.
        const style = getComputedStyle(element);
        if (style.position === "absolute" && box.width <= 2 && box.height <= 2) {
          return false;
        }
        // WCAG 2.5.8's inline exception, which R1 inherits: a link inside a
        // sentence is sized by the text around it, and padding it to 44 px
        // would break the paragraph it lives in. Without this the rule reports
        // every "Sign in" link in a body of text and gets switched off, which
        // is worse than not having it.
        const inlineLink =
          element.tagName === "A" && style.display.startsWith("inline");
        return !inlineLink;
      })
      .map((element) => {
        const box = element.getBoundingClientRect();
        return {
          tag: element.tagName.toLowerCase(),
          label: (element.getAttribute("aria-label") || element.textContent || "").trim().slice(0, 30),
          width: Math.round(box.width),
          height: Math.round(box.height),
        };
      })
      .filter((box) => box.width < 44 || box.height < 44);
  });

  expect(
    undersized,
    `${where} has touch targets under 44×44 px at 320 px: ${JSON.stringify(undersized)}`,
  ).toEqual([]);
}

async function check(page: Page, where: string): Promise<void> {
  await expectNoHorizontalScroll(page, where);
  await expectTouchTargets(page, where);
}

test.describe("R6/R1 at 320 px", () => {
  test("the signed-out screens fit", async ({ page }) => {
    for (const [path, name] of [
      ["/login", "login"],
      ["/register", "register"],
      ["/forgot-password", "forgot password"],
    ] as const) {
      await page.goto(path);
      await check(page, name);
    }
  });

  test("the OTP screen fits, including with the keyboard open", async ({ page }) => {
    await stubRegistration(page);

    await page.goto("/register");
    await page.getByLabel(/Display name/i).fill("Layout Learner");
    await page.getByLabel(/Email address/i).fill("layout-learner@example.com");
    await page.getByLabel(/^Password/i).fill(newPassword());
    await page.getByRole("button", { name: /Create account/i }).click();

    await expect(
      page.getByRole("heading", { name: /Enter verification code/i }),
    ).toBeVisible();

    // Six 44 px boxes plus their gaps is the widest row in the product, which
    // is why the card names this screen specifically.
    await check(page, "OTP screen");

    // Focusing a digit is what opens the virtual keyboard on a real phone; the
    // layout must not reflow into an overflow when it does.
    await page.locator('input[inputmode="numeric"]').first().focus();
    await check(page, "OTP screen with a digit focused");
  });

  // P11.2's acceptance: the narrow-320 project passes in `vi` as well as `en`.
  // Vietnamese runs 20–30 % longer than English, and the four review grade
  // buttons are the tightest row in the app — P10.4 said to check that one
  // specifically, so it is checked in both locales rather than in the default.
  for (const locale of ["en", "vi"] as const) {
    test(`the learner screens fit in ${locale}`, async ({ page }) => {
      await stubAuthenticated(page);
      await stubAccountApi(page);
      await stubLearningApi(page);

      await page.addInitScript((value) => {
        window.localStorage.setItem("fluentra.locale", value);
      }, locale);

      for (const [path, name] of [
        ["/", "dashboard"],
        ["/learn", "learn"],
        ["/progress", "progress"],
        ["/practice/review", "review"],
      ] as const) {
        await page.goto(path);
        await check(page, `${name} — ${locale}`);
      }

      // The review card flipped: the grade row only exists after the reveal, and
      // it is the row this test was written for.
      await page.goto("/practice/review");
      await page.keyboard.press("Space");
      await check(page, `review graded — ${locale}`);
    });
  }

  test("the account screens fit", async ({ page }) => {
    await stubAuthenticated(page);
    await stubAccountApi(page);

    await page.goto("/settings");
    for (const tab of [
      /Profile & Avatar/i,
      /Learning Preferences/i,
      /Security & Devices/i,
      /Data & Privacy/i,
    ]) {
      await page.getByRole("button", { name: tab }).click();
      await check(page, `settings — ${tab.source}`);
    }
  });
});
