import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";

import { expect, type Page } from "@playwright/test";

import { extractOtpCode, waitForEmail } from "./mailpit";

/**
 * A learner these journeys can create against the real API.
 *
 * The address is unique per test run because the journeys share one Postgres:
 * a fixed address turns a re-run into a "that email is taken" failure that has
 * nothing to do with what the journey is testing.
 */
export interface Learner {
  email: string;
  password: string;
  displayName: string;
}

export function newLearner(tag: string): Learner {
  const unique = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  return {
    email: `learner-${tag}-${unique}@example.com`,
    password: newPassword(),
    // The tag stays out of the display name. BR-USER-02 rejects any name
    // containing "admin", "support", "fluentra" and friends as impersonation,
    // so a journey tagged "j9-admin" was refused with DISPLAY_NAME_NOT_ALLOWED
    // before it ever reached the flow it was testing.
    displayName: `Learner ${unique.slice(-6)}`,
  };
}

/**
 * A password the breach corpus cannot know, because it did not exist until now.
 *
 * BREACHED_PASSWORD_CHECK is on, and a memorable literal like newPassword()
 * is in every corpus there is — registration answers 422 PASSWORD_TOO_WEAK. It
 * has to be random rather than merely long: the check fails *open*, so a
 * breached literal is refused on a machine that can reach the corpus and
 * accepted in CI, which cannot. Randomness is what makes the two agree.
 */
export function newPassword(): string {
  const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  const bytes = randomBytes(20);
  return Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join("");
}

/**
 * Fills the six OTP boxes, which submits on its own.
 *
 * `OtpInput` fires `onComplete` the moment the last digit lands, and the screen
 * verifies immediately. Clicking "Verify & continue" afterwards races that: the
 * button is detached mid-click and Playwright reports an unstable element, not
 * a wrong code. So there is no click here on purpose.
 */
export async function enterOtp(page: Page, code: string): Promise<void> {
  const boxes = page.locator('input[inputmode="numeric"]');

  // Typed, not filled box by box. `OtpInput` moves focus to the next box inside
  // its own onChange, so a per-box `fill()` races that move: Playwright resolves
  // box N+1, the component focuses it, and the fill lands on an element that has
  // just been re-rendered. It stalls mid-code under load, which reads as a
  // 30-second timeout on a digit. Typing into the first box lets the
  // component's own focus management do the walking, which is also what a
  // learner does.
  // Cleared first. The screen does not reset the digits after a refusal, so
  // typing over a full row makes `onComplete` fire on the very first keystroke
  // — with five stale digits and one new one — and every keystroke after that
  // is swallowed by the `isVerifying` guard. Five wrong attempts then cost one.
  const count = await boxes.count();
  for (let index = count - 1; index >= 0; index -= 1) {
    await boxes.nth(index).fill("");
  }

  await boxes.first().click();
  await page.keyboard.type(code, { delay: 20 });
}

/**
 * Registers a learner through the UI, reads the code out of Mailpit, verifies,
 * and leaves the browser signed in on the dashboard.
 *
 * It deliberately does not clear the mailbox. Every learner gets a unique
 * address, so matching on the recipient is enough — and a global clear would
 * delete the code a parallel worker is waiting for.
 */
export async function registerAndVerify(
  page: Page,
  learner: Learner,
): Promise<void> {
  await page.goto("/register");
  await page.getByLabel(/Display name/i).fill(learner.displayName);
  await page.getByLabel(/Email address/i).fill(learner.email);
  await page.getByLabel(/^Password/i).fill(learner.password);

  // Captured so a refused registration reports the server's reason. Without it
  // every refusal — rate limit, weak password, address taken — looks the same:
  // "heading not found", which sends you looking at the wrong layer.
  let refusal = "";
  page.on("response", (response) => {
    if (response.url().includes("/auth/register") && !response.ok()) {
      refusal = `${response.status()}`;
      void response
        .text()
        .then((body) => {
          refusal = `${response.status()} ${body.slice(0, 200)}`;
        })
        .catch(() => undefined);
    }
  });

  await page.getByRole("button", { name: /Create account/i }).click();

  await expect(
    page.getByRole("heading", { name: /Enter verification code/i }),
    `registration did not reach the OTP screen. Server refusal: ${refusal || "none"}`,
  ).toBeVisible({ timeout: 15_000 });

  const message = await waitForEmail(learner.email);
  await enterOtp(page, extractOtpCode(message.Text || message.HTML || ""));

  await expect(page).toHaveURL("/", { timeout: 15_000 });
}

/** Signs the current browser out and waits for the login screen. */
export async function signOut(page: Page): Promise<void> {
  await page.getByRole("button", { name: /Sign out|Logout/i }).first().click();
  await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
}

/**
 * Signs in, optionally ticking "Stay signed in on this device".
 *
 * That checkbox is what creates a trusted device, so it is the only way a
 * journey about the device list gets a row to act on.
 */
export async function signIn(
  page: Page,
  learner: Learner,
  options: { rememberDevice?: boolean } = {},
): Promise<void> {
  await page.goto("/login");
  await page.getByLabel(/Email address/i).fill(learner.email);
  await page.getByLabel(/^Password/i).fill(learner.password);

  const remember = page.getByText(/Stay signed in on this device/i);
  if (options.rememberDevice === false) {
    // It defaults to checked, so an unticked run has to untick it.
    await remember.click();
  }

  // Captured so a refusal is reported as the server's reason rather than as a
  // bare "still on /login" timeout, which says nothing about why.
  let refusal = "";
  page.on("response", (response) => {
    if (response.url().includes("/auth/login") && !response.ok()) {
      refusal = `${response.status()} ${response.url()}`;
    }
  });

  await page.getByRole("button", { name: /^Sign in$/i }).click();

  await expect(page, `sign-in did not land on the dashboard. Server refusal: ${refusal || "none"}. Form said: ${await formError(page)}`).toHaveURL("/", { timeout: 10_000 });
}

/** Whatever the login form is currently showing as an error, or "nothing". */
async function formError(page: Page): Promise<string> {
  const alert = page.locator('[role="alert"], .text-rose-300, .text-rose-400');
  if ((await alert.count()) === 0) return "nothing";
  return (await alert.first().innerText().catch(() => "unreadable")).trim();
}

/**
 * Grants the admin role to an account that already exists.
 *
 * It shells out to `make promote-admin`, which runs db/seeds/rbac.sql — the
 * same path a developer or an operator uses. Reaching into the database from
 * the test with its own SQL would mean the journey proves a grant nobody else
 * performs; driving the real target means a change to the seed breaks this
 * journey, which is the point.
 */
export function promoteToAdmin(email: string): void {
  execFileSync("make", ["promote-admin", `EMAIL=${email}`], {
    cwd: new URL("../../../", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"),
    stdio: "pipe",
  });
}

/**
 * Asserts the browser is signed in.
 *
 * The shell renders "Sign out" in the desktop sidebar and "Logout" in the
 * mobile bottom bar, so a matcher naming only one passes on half the device
 * matrix and fails on the other half — for a reason that has nothing to do with
 * the journey under test.
 */
export async function expectSignedIn(page: Page): Promise<void> {
  await expect(
    page.getByRole("button", { name: /Sign out|Logout/i }).first(),
  ).toBeVisible();
}
