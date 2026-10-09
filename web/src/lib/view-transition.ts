/**
 * Page-wide transitions for changes that repaint or re-lay-out a lot at once,
 * on top of the View Transitions API. The browser snapshots the page before and
 * after `update`, then animates the snapshots on the compositor, so the DOM
 * changes in one step instead of being re-laid-out on every frame.
 *
 * `kind` is written to <html data-vt> for the duration; index.css picks the
 * animation from it, and route changes (no `data-vt`) keep their own
 * shell-stays-put animation:
 *
 * - `theme`: a circle of the new palette grows from `origin` (where the learner
 *   clicked) to the farthest corner — `reveal` in CSS. Without an origin, a
 *   plain cross-fade.
 * - `locale`: the old words blur and drift up while the new ones settle in.
 * - `sidebar`: the sidebar, header and page morph between the collapsed and
 *   expanded layouts.
 *
 * Where the API is missing, or the learner asked the OS for less motion, the
 * change is applied directly.
 */
export type PageTransitionKind = "theme" | "locale" | "sidebar";

/** A viewport point, in CSS pixels, a transition grows out of. */
export interface RevealOrigin {
  x: number;
  y: number;
}

type ViewTransitionDocument = Document & {
  startViewTransition?: (cb: () => Promise<void> | void) => {
    finished: Promise<void>;
  };
};

/** Whether `withPageTransition` will animate rather than apply directly. */
export function canAnimatePage(): boolean {
  if (typeof document === "undefined") return false;
  const doc = document as ViewTransitionDocument;
  const reduced = window.matchMedia?.(
    "(prefers-reduced-motion: reduce)",
  ).matches;
  return typeof doc.startViewTransition === "function" && !reduced;
}

/**
 * Where a theme reveal should start, from the event that asked for it.
 *
 * A pointer click carries its own coordinates. A keyboard activation (a click
 * with `detail === 0`, a Radix menu `select`, a radio changed with the arrow
 * keys) has none, so the reveal starts from the centre of the control instead —
 * the place the learner is looking either way.
 */
export function revealOriginFrom(
  event: Event | { nativeEvent: Event; currentTarget: EventTarget | null },
): RevealOrigin | undefined {
  const native = "nativeEvent" in event ? event.nativeEvent : event;
  if (
    typeof MouseEvent !== "undefined" &&
    native instanceof MouseEvent &&
    native.detail > 0 &&
    (native.clientX !== 0 || native.clientY !== 0)
  ) {
    return { x: native.clientX, y: native.clientY };
  }
  const target =
    ("nativeEvent" in event ? event.currentTarget : null) ?? native.target;
  return revealOriginOf(target);
}

/** The centre of a control, for a change that came from it without a pointer. */
export function revealOriginOf(
  target: EventTarget | null | undefined,
): RevealOrigin | undefined {
  if (typeof Element === "undefined" || !(target instanceof Element)) {
    return undefined;
  }
  // A visually hidden radio is a 1px box; its label is what was seen.
  const box = (target.closest("label") ?? target).getBoundingClientRect();
  if (box.width === 0 && box.height === 0) return undefined;
  return { x: box.left + box.width / 2, y: box.top + box.height / 2 };
}

/** The latest transition; an earlier one finishing must not undo its setup. */
let current = 0;

export async function withPageTransition(
  kind: PageTransitionKind,
  update: () => void | Promise<void>,
  options: {
    /** `theme` only: where the reveal starts. */
    origin?: RevealOrigin | undefined;
    /** Written to <html data-vt-dir>, for a kind that animates either way. */
    dir?: string | undefined;
  } = {},
): Promise<void> {
  if (!canAnimatePage()) {
    await update();
    return;
  }
  const doc = document as ViewTransitionDocument;
  const root = document.documentElement;
  const token = ++current;
  const origin = kind === "theme" ? options.origin : undefined;
  if (origin) {
    // The circle has to cover the whole viewport by the end, so its radius is
    // the distance to the farthest corner.
    const x = Math.min(Math.max(origin.x, 0), window.innerWidth);
    const y = Math.min(Math.max(origin.y, 0), window.innerHeight);
    const r = Math.hypot(
      Math.max(x, window.innerWidth - x),
      Math.max(y, window.innerHeight - y),
    );
    root.style.setProperty("--vt-x", `${x}px`);
    root.style.setProperty("--vt-y", `${y}px`);
    root.style.setProperty("--vt-r", `${Math.ceil(r)}px`);
  }
  root.dataset["vt"] = origin ? "reveal" : kind;
  if (options.dir) root.dataset["vtDir"] = options.dir;
  else delete root.dataset["vtDir"];
  try {
    const transition = doc.startViewTransition(async () => {
      await update();
      if (kind === "locale") {
        // A language switch re-renders through a React state update scheduled
        // by i18next; give the scheduler a task to commit it before the new
        // snapshot is taken. Rendering is paused inside this callback, so this
        // must be a task, not an animation frame.
        await new Promise((resolve) => setTimeout(resolve, 0));
      }
    });
    await transition.finished;
  } catch {
    // An aborted transition still applied the update; nothing to undo.
  } finally {
    // Starting a transition skips the running one, whose `finished` then
    // settles while the new one is on screen: leave the new one's state alone.
    if (token === current) clearState(root);
  }
}

function clearState(root: HTMLElement): void {
  delete root.dataset["vt"];
  delete root.dataset["vtDir"];
  root.style.removeProperty("--vt-x");
  root.style.removeProperty("--vt-y");
  root.style.removeProperty("--vt-r");
}
