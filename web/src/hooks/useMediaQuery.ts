import { useSyncExternalStore } from "react";

/**
 * Whether a media query matches, kept in sync with `change` events.
 *
 * `fallback` answers when there is no `matchMedia` (jsdom, SSR). Use it for a
 * layout choice that really needs different markup; when CSS alone can switch
 * (a breakpoint class, a container query) prefer that, since it needs no JS.
 */
export function useMediaQuery(query: string, fallback = true): boolean {
  return useSyncExternalStore(
    (notify) => {
      if (
        typeof window === "undefined" ||
        typeof window.matchMedia !== "function"
      ) {
        return () => {};
      }
      const list = window.matchMedia(query);
      list.addEventListener("change", notify);
      return () => list.removeEventListener("change", notify);
    },
    () =>
      typeof window.matchMedia === "function"
        ? window.matchMedia(query).matches
        : fallback,
    () => fallback,
  );
}

/** `md` and up (768px), the point where the sidebar replaces bottom navigation. */
export const MD_UP = "(min-width: 768px)";
