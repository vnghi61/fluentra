import { test, expect } from "@playwright/test";

test.describe("App Smoke Test", () => {
  /**
   * The root page loads and the app frame is around it.
   *
   * This asserted `h1` contained "Fluentra", which was only ever true because a
   * visitor with no session was redirected to the login screen and *its*
   * heading reads "Sign in to Fluentra". Since ADR-0025 they land on the public
   * catalogue instead, whose h1 names the page rather than the product — so the
   * old assertion was testing the redirect, not the header it claimed to.
   *
   * The brand link is the application header, and it is on every framed screen
   * whether or not anyone is signed in.
   */
  test("loads the root page and renders the application header", async ({
    page,
  }) => {
    await page.goto("/");

    // Two brand links exist — the sidebar's above `md`, the header's below it —
    // and exactly one is displayed at any width. `.first()` would pick the
    // sidebar's, which is `display: none` on the three mobile projects, so the
    // filter is what makes this one assertion work across the whole matrix.
    await expect(
      page
        .getByRole("link", { name: /Fluentra/i })
        .filter({ visible: true })
        .first(),
    ).toBeVisible({ timeout: 15_000 });

    // Exactly one h1, which is the thing the old assertion could not have
    // caught: the catalogue had none at all, and its unit headings sat at h2
    // under a course title rendered as h3.
    await expect(page.locator("h1")).toHaveCount(1);
  });
});
