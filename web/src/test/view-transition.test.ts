import { afterEach, describe, expect, it, vi } from "vitest";

import {
  revealOriginFrom,
  revealOriginOf,
  withPageTransition,
} from "@/lib/view-transition";

function boxed(el: Element, rect: Partial<DOMRect>): void {
  vi.spyOn(el, "getBoundingClientRect").mockReturnValue({
    x: 0,
    y: 0,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    width: 0,
    height: 0,
    toJSON: () => ({}),
    ...rect,
  });
}

describe("revealOriginFrom", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses the pointer position of a real click", () => {
    const button = document.createElement("button");
    const click = new MouseEvent("click", {
      detail: 1,
      clientX: 120,
      clientY: 48,
    });
    expect(
      revealOriginFrom({ nativeEvent: click, currentTarget: button }),
    ).toEqual({ x: 120, y: 48 });
  });

  it("falls back to the centre of the control for a keyboard activation", () => {
    const button = document.createElement("button");
    boxed(button, { left: 100, top: 20, width: 40, height: 20 });
    // Enter or Space on a button dispatches a click with detail 0 and no point.
    const click = new MouseEvent("click", { detail: 0 });
    expect(
      revealOriginFrom({ nativeEvent: click, currentTarget: button }),
    ).toEqual({ x: 120, y: 30 });
  });

  it("uses a Radix select event's target, and a hidden radio's label", () => {
    const label = document.createElement("label");
    const radio = document.createElement("input");
    radio.type = "radio";
    label.append(radio);
    boxed(label, { left: 0, top: 600, width: 100, height: 64 });
    let origin: unknown;
    radio.addEventListener("menu.itemSelect", (event) => {
      origin = revealOriginFrom(event);
    });
    radio.dispatchEvent(new CustomEvent("menu.itemSelect"));
    expect(origin).toEqual({ x: 50, y: 632 });
    expect(revealOriginOf(radio)).toEqual({ x: 50, y: 632 });
  });

  it("reports no origin for an element without a box", () => {
    expect(revealOriginOf(document.createElement("div"))).toBeUndefined();
    expect(revealOriginOf(null)).toBeUndefined();
  });
});

describe("withPageTransition", () => {
  it("applies the change directly where view transitions are missing", async () => {
    const update = vi.fn();
    await withPageTransition("sidebar", update, { dir: "collapse" });
    expect(update).toHaveBeenCalledOnce();
    expect(document.documentElement.dataset["vt"]).toBeUndefined();
    expect(document.documentElement.dataset["vtDir"]).toBeUndefined();
  });

  it("marks the page for its kind while a transition runs, then clears it", async () => {
    const seen: Array<Record<string, string | undefined>> = [];
    const start = (cb: () => Promise<void>) => {
      const root = document.documentElement;
      seen.push({
        vt: root.dataset["vt"],
        x: root.style.getPropertyValue("--vt-x"),
        r: root.style.getPropertyValue("--vt-r"),
      });
      return { finished: cb() };
    };
    Object.defineProperty(document, "startViewTransition", {
      value: start,
      configurable: true,
    });
    try {
      await withPageTransition("theme", () => undefined, {
        origin: { x: 0, y: 0 },
      });
      await withPageTransition("theme", () => undefined);
    } finally {
      Reflect.deleteProperty(document, "startViewTransition");
    }
    // The circle from a corner has to reach the opposite one.
    const diagonal = Math.ceil(
      Math.hypot(window.innerWidth, window.innerHeight),
    );
    expect(seen).toEqual([
      { vt: "reveal", x: "0px", r: `${diagonal}px` },
      { vt: "theme", x: "", r: "" },
    ]);
    expect(document.documentElement.dataset["vt"]).toBeUndefined();
    expect(document.documentElement.style.getPropertyValue("--vt-r")).toBe("");
  });
});
