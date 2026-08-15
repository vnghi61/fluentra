import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  type AnyRoute,
} from "@tanstack/react-router";

import { AppShell } from "@/components/layout/AppShell";
import { HomePage } from "@/routes/HomePage";
import { PracticePage } from "@/routes/PracticePage";

/**
 * The route tree is declared in code rather than generated from the filesystem.
 * File-based routing needs a Vite plugin and a generated route file, and a
 * generated file that nothing yet regenerates in CI is a staleness gate waiting
 * to be discovered — which this repository has already been bitten by once.
 * Two routes do not justify it.
 */

const rootRoute = createRootRoute({
  component: () => (
    <AppShell>
      <Outlet />
    </AppShell>
  ),
});

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

const practiceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice",
  component: PracticePage,
});

const routeTree = rootRoute.addChildren([
  homeRoute,
  practiceRoute,
] as AnyRoute[]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
