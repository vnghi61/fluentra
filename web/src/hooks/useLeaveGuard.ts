import { useCallback, useEffect, useRef } from "react";
import { useBlocker, useRouterState } from "@tanstack/react-router";

export interface LeaveGuard {
  /** A navigation is waiting for the learner's answer: show the dialog. */
  blocked: boolean;
  /** Leave: let the waiting navigation through. */
  proceed: () => void;
  /** Stay: cancel the waiting navigation. */
  stay: () => void;
  /** Lets the next navigation through unasked (finishing, or the page's own exit). */
  allowNext: () => void;
}

/**
 * Asks before leaving a session that is under way: a lesson or practice set
 * half done, an exam sitting with its clock running.
 *
 * Covers both ways out. A link, the sidebar or Back goes through the router and
 * gets the page's own dialog; a reload, a closed tab or a typed address goes
 * through `beforeunload`, where the browser shows its own prompt (its text is
 * not ours to set). Moves that stay on the same page — a search parameter, the
 * next question — never ask; `allowed(pathname)` lets a page name destinations
 * that are part of finishing, such as an exam's report.
 */
export function useLeaveGuard(
  active: boolean,
  allowed: (nextPathname: string) => boolean = () => false,
): LeaveGuard {
  const bypass = useRef(false);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  // A bypass covers one departure. Landing somewhere new (the same page
  // component on the next lesson, say) arms the guard again.
  useEffect(() => {
    bypass.current = false;
  }, [pathname]);

  const blocker = useBlocker({
    shouldBlockFn: ({ current, next }) => {
      if (!active || bypass.current) return false;
      if (current.pathname === next.pathname) return false;
      return !allowed(next.pathname);
    },
    enableBeforeUnload: () => active && !bypass.current,
    withResolver: true,
  });

  const allowNext = useCallback(() => {
    bypass.current = true;
  }, []);

  return {
    blocked: blocker.status === "blocked",
    proceed: () => {
      bypass.current = true;
      blocker.proceed?.();
    },
    stay: () => blocker.reset?.(),
    allowNext,
  };
}
