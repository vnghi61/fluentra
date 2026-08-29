import React from "react";
import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Outlet,
  redirect,
  useNavigate,
  useRouterState,
  type AnyRoute,
} from "@tanstack/react-router";

import i18n from "@/i18n";
import { AppShell } from "@/components/layout/AppShell";
import { ServerWakingBanner } from "@/components/layout/ServerWakingBanner";
import { useWakeStatus } from "@/hooks/useWakeStatus";
import { usePreferencesSync } from "@/features/account/hooks/usePreferencesSync";
import { useDisplayName } from "@/features/account/hooks/useDisplayName";

/**
 * Lazy for the same reason AccountMenu is: both are built on Radix, so together
 * they keep its ~30 kB out of the entry chunk instead of each paying for it.
 */
const ThemeLanguageControls = React.lazy(async () => ({
  default: (await import("@/features/account/components/ThemeLanguageControls"))
    .ThemeLanguageControls,
}));

/**
 * The routes that stand on their own, with no app frame around them.
 *
 * AppShell wraps the root route, so without this list /login drew a sidebar and
 * a bottom bar offering four destinations a signed-out visitor is redirected
 * away from. Listed by path rather than by route id because the OAuth callback
 * carries a provider segment.
 */
const bareRoutes = [
  "/login",
  "/register",
  "/forgot-password",
  "/reset-password",
  "/auth/",
];

function isBareRoute(pathname: string): boolean {
  return bareRoutes.some(
    (prefix) => pathname === prefix || pathname.startsWith(prefix),
  );
}
import { authApi } from "@/features/auth";
import { ForgotPasswordPage } from "@/pages/ForgotPasswordPage";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { useAuthStore } from "@/stores/authStore";

/**
 * The route tree is declared in code rather than generated from the filesystem.
 * File-based routing needs a Vite plugin and a generated route file, and a
 * generated file that nothing yet regenerates in CI is a staleness gate waiting
 * to be discovered — which this repository has already been bitten by once.
 */

function RouteLoadingSpinner(): React.JSX.Element {
  return (
    <div
      role="status"
      aria-label={i18n.t("app.loading", "Loading")}
      className="flex items-center justify-center p-8 min-h-[200px]"
    >
      <div className="h-8 w-8 animate-spin rounded-full border-4 border-border-subtle border-t-primary" />
      <span className="sr-only">{i18n.t("app.loading", "Loading")}</span>
    </div>
  );
}

const DashboardPage = lazyRouteComponent(
  () => import("@/routes/DashboardPage"),
  "DashboardPage",
);

const LearnPage = lazyRouteComponent(
  () => import("@/routes/LearnPage"),
  "LearnPage",
);

const LessonPage = lazyRouteComponent(
  () => import("@/routes/LessonPage"),
  "LessonPage",
);

const PracticePage = lazyRouteComponent(
  () => import("@/routes/PracticePage"),
  "PracticePage",
);

const ReviewPage = lazyRouteComponent(
  () => import("@/routes/ReviewPage"),
  "ReviewPage",
);

const ProgressPage = lazyRouteComponent(
  () => import("@/routes/ProgressPage"),
  "ProgressPage",
);

const OAuthCallbackPage = lazyRouteComponent(
  () => import("@/pages/OAuthCallbackPage"),
  "OAuthCallbackPage",
);

const AccountSettingsPage = lazyRouteComponent(
  () => import("@/pages/AccountSettingsPage"),
  "AccountSettingsPage",
);

const AdminPage = lazyRouteComponent(
  () => import("@/pages/AdminPage"),
  "AdminPage",
);

function RootApp(): React.JSX.Element {
  const user = useAuthStore((s) => s.user);
  const status = useAuthStore((s) => s.status);
  const navigate = useNavigate();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const { themeChoice, locale, setThemeChoice, setLocaleChoice } =
    usePreferencesSync(status === "authenticated");
  const wake = useWakeStatus();
  const displayName = useDisplayName(status === "authenticated");

  const handleLogout = async () => {
    await authApi.logout();
    void navigate({ to: "/login" });
  };

  return (
    <AppShell
      user={user}
      status={status}
      onLogout={() => void handleLogout()}
      chrome={!isBareRoute(pathname)}
      displayName={displayName}
      banner={
        <ServerWakingBanner
          waking={wake === "waking"}
          unreachable={wake === "unreachable"}
        />
      }
      controls={
        <React.Suspense
          fallback={<div className="h-11 w-24" aria-hidden="true" />}
        >
          <ThemeLanguageControls
            themeChoice={themeChoice}
            locale={locale}
            onThemeChoice={setThemeChoice}
            onLocale={setLocaleChoice}
          />
        </React.Suspense>
      }
    >
      <Outlet />
    </AppShell>
  );
}

export const rootRoute = createRootRoute({
  component: RootApp,
});

/**
 * The routes a visitor with no account may reach.
 *
 * Everything here used to redirect to /login the moment `status` said
 * unauthenticated, which is how the product's whole value ended up behind a
 * registration form. ADR-0025 opens the curriculum; these are the screens that
 * serve it.
 *
 * `/` is not among them, and cannot be: the dashboard is "continue where you
 * left off", "reviews due" and "your skill mastery", every one of which is a
 * fact about a person. For a guest it has no content, so `/` sends them to the
 * catalogue, which is the thing they came to see.
 */
export const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/learn" });
    }
  },
  component: DashboardPage,
});

export const learnRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/learn",
  // Open to a visitor with no account (ADR-0025). The screen itself says what
  // is not being saved; it does not pretend to be signed in.
  component: LearnPage,
});

export const lessonRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/learn/lesson/$lessonId",
  // Open to a visitor with no account (ADR-0025). The screen itself says what
  // is not being saved; it does not pretend to be signed in.
  component: LessonPage,
});

export const practiceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice",
  // Open to a visitor with no account (ADR-0025). The screen itself says what
  // is not being saved; it does not pretend to be signed in.
  component: PracticePage,
});

export const reviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice/review",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ReviewPage,
});

export const progressRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/progress",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ProgressPage,
});

export const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: AccountSettingsPage,
});

export const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin",
  beforeLoad: () => {
    const { status, user } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
    if (user?.role !== "admin") {
      throw redirect({ to: "/" });
    }
  },
  component: AdminPage,
});

export const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "authenticated") {
      throw redirect({ to: "/" });
    }
  },
  component: LoginPage,
});

export const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/register",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "authenticated") {
      throw redirect({ to: "/" });
    }
  },
  component: RegisterPage,
});

export const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/forgot-password",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "authenticated") {
      throw redirect({ to: "/" });
    }
  },
  component: ForgotPasswordPage,
});

export const oauthCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  // Matches OAUTH_GOOGLE_REDIRECT_URL in .env.example / .env.prod. The path
  // Google returns the code to must be a route the SPA owns, or the callback
  // renders a 404 and the single-use state expires before it is ever spent.
  path: "/auth/google/callback",
  component: OAuthCallbackPage,
});

export const routeTree = rootRoute.addChildren([
  homeRoute,
  learnRoute,
  lessonRoute,
  practiceRoute,
  reviewRoute,
  progressRoute,
  settingsRoute,
  adminRoute,
  loginRoute,
  registerRoute,
  forgotPasswordRoute,
  oauthCallbackRoute,
] as AnyRoute[]);

export const router = createRouter({
  routeTree,
  defaultPendingComponent: RouteLoadingSpinner,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
