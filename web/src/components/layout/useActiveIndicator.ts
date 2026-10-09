import * as React from "react";

export interface IndicatorRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Marks the element the main indicator should sit on. */
export const INDICATOR_ATTR = "data-indicator";

/** Marks the active item of a nested list, such as the Admin sections. */
export const SUB_INDICATOR_ATTR = "data-subindicator";

/**
 * Where the active item of a navigation list is, so one indicator can slide to
 * it rather than each item drawing its own and the highlight jumping.
 *
 * Items opt in with `<attr>="true"`. When several match, the last visible one
 * wins: it is the most specific. Positions are offsets inside `container`,
 * which must be the indicator's positioned parent, so the indicator scrolls
 * with the list.
 *
 * `animate` says whether the latest move should slide. A change of `deps` (a
 * navigation) slides; a resize or a re-layout (the sidebar collapsing, the
 * Admin group opening) follows instantly, because a slide chasing a moving
 * target lags behind it and reads as the two animations disagreeing.
 *
 * `layoutKey` names a re-layout the caller makes itself, such as collapsing
 * the sidebar. A change of it is measured synchronously, in the same commit,
 * so the indicator is already in place when the sidebar's view transition
 * takes its "after" snapshot; the observers below would catch it a frame
 * late.
 */
export function useActiveIndicator(
  container: React.RefObject<HTMLElement | null>,
  deps: React.DependencyList,
  attr: string = INDICATOR_ATTR,
  layoutKey?: unknown,
): { rect: IndicatorRect | null; animate: boolean } {
  const [state, setState] = React.useState<{
    rect: IndicatorRect | null;
    animate: boolean;
  }>({ rect: null, animate: false });

  const measure = React.useCallback(
    (slide: boolean) => {
      const root = container.current;
      if (!root) return;
      const candidates = Array.from(
        root.querySelectorAll<HTMLElement>(`[${attr}="true"]`),
      ).filter((el) => el.offsetWidth > 0 && el.offsetHeight > 0);
      const el = candidates[candidates.length - 1];
      if (!el) {
        setState((prev) =>
          prev.rect === null ? prev : { rect: null, animate: false },
        );
        return;
      }
      // offsetLeft/Top are relative to the nearest positioned ancestor; walk
      // up to `root` in case an item sits inside its own positioned wrapper.
      let x = 0;
      let y = 0;
      let node: HTMLElement | null = el;
      while (node && node !== root) {
        x += node.offsetLeft;
        y += node.offsetTop;
        node = node.offsetParent as HTMLElement | null;
      }
      const next = { x, y, width: el.offsetWidth, height: el.offsetHeight };
      setState((prev) => {
        const same =
          prev.rect &&
          prev.rect.x === next.x &&
          prev.rect.y === next.y &&
          prev.rect.width === next.width &&
          prev.rect.height === next.height;
        if (same) return prev;
        // The first placement never slides in from the corner.
        return { rect: next, animate: slide && prev.rect !== null };
      });
    },
    [container, attr],
  );

  // Measured on the next frame, after the new active item has its attributes
  // and layout.
  React.useLayoutEffect(() => {
    const frame = requestAnimationFrame(() => measure(true));
    return () => cancelAnimationFrame(frame);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-measure when the caller's route state changes
  }, deps);

  const laidOut = React.useRef(false);
  React.useLayoutEffect(() => {
    // The first placement belongs to the effect above.
    if (!laidOut.current) {
      laidOut.current = true;
      return;
    }
    measure(false);
  }, [layoutKey, measure]);

  React.useEffect(() => {
    const root = container.current;
    if (!root || typeof ResizeObserver === "undefined") return;
    let frame = 0;
    const follow = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => measure(false));
    };
    const resize = new ResizeObserver(follow);
    resize.observe(root);
    const mutation = new MutationObserver(follow);
    mutation.observe(root, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ["hidden"],
    });
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      mutation.disconnect();
    };
  }, [container, measure]);

  return state;
}
