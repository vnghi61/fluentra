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
import {
  useAvatarUrl,
  useDisplayName,
} from "@/features/account/hooks/useDisplayName";

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
  if (pathname.startsWith("/exams/") && !pathname.endsWith("/report")) {
    return true;
  }
  if (pathname === "/placement" || pathname.startsWith("/placement/")) {
    return true;
  }
  if (pathname === "/welcome" || pathname.startsWith("/welcome/")) {
    return true;
  }
  return bareRoutes.some(
    (prefix) => pathname === prefix || pathname.startsWith(prefix),
  );
}

/**
 * The immersive runners, which keep the shell header but not the bottom bar.
 *
 * The lesson runner is full-screen and distraction-free (P10.3) and carries its
 * own exit and progress. The fixed bottom bar sat on top of its primary action:
 * on a 390 px phone the Check button of a four-option exercise rendered behind
 * it. These routes stay in the shell — unlike the exam sitting, a lesson is open
 * to a visitor with no account, and the header is where their Sign in and Create
 * account links live.
 */
const runnerRoutes = ["/learn/lesson/", "/practice/daily"];

function isRunnerRoute(pathname: string): boolean {
  return runnerRoutes.some((prefix) => pathname.startsWith(prefix));
}
import { authApi } from "@/features/auth";
import { clearAllWritingDrafts } from "@/features/writing/utils/draftStorage";
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

const MyWordsPage = lazyRouteComponent(
  () => import("@/routes/MyWordsPage"),
  "MyWordsPage",
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

const MyWritingPage = lazyRouteComponent(
  () => import("@/routes/MyWritingPage"),
  "MyWritingPage",
);

const MyResourcesPage = lazyRouteComponent(
  () => import("@/routes/MyResourcesPage"),
  "MyResourcesPage",
);

const ResourceDetailPage = lazyRouteComponent(
  () => import("@/routes/ResourceDetailPage"),
  "ResourceDetailPage",
);

const MySpeakingPage = lazyRouteComponent(
  () => import("@/routes/MySpeakingPage"),
  "MySpeakingPage",
);

const ExamsPage = lazyRouteComponent(
  () => import("@/routes/ExamsPage"),
  "ExamsPage",
);

const ExamSittingPage = lazyRouteComponent(
  () => import("@/routes/ExamSittingPage"),
  "ExamSittingPage",
);

const ExamReportPage = lazyRouteComponent(
  () => import("@/routes/ExamReportPage"),
  "ExamReportPage",
);

const WelcomePage = lazyRouteComponent(
  () => import("@/pages/WelcomePage"),
  "WelcomePage",
);

const PlacementPage = lazyRouteComponent(
  () => import("@/routes/PlacementPage"),
  "PlacementPage",
);

const StudioPage = lazyRouteComponent(
  () => import("@/routes/StudioPage"),
  "StudioPage",
);

const StudioEditorPage = lazyRouteComponent(
  () => import("@/routes/StudioEditorPage"),
  "StudioEditorPage",
);

const FoundationTopicPage = lazyRouteComponent(
  () => import("@/routes/FoundationTopicPage"),
  "FoundationTopicPage",
);

/** Lazy: it reads /me/permissions, which nobody but an administrator needs. */

const AdminSidebarNav = React.lazy(() =>
  import("@/features/admin/components/AdminSidebarNav").then((m) => ({
    default: m.AdminSidebarNav,
  })),
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
  const signedIn = status === "authenticated";
  const displayName = useDisplayName(signedIn);
  const avatarUrl = useAvatarUrl(signedIn);

  const handleLogout = async () => {
    clearAllWritingDrafts();
    await authApi.logout();
    void navigate({ to: "/login" });
  };

  return (
    <AppShell
      user={user}
      status={status}
      adminNav={
        user?.role === "admin" || user?.role === "moderator" ? (
          <React.Suspense fallback={null}>
            <AdminSidebarNav />
          </React.Suspense>
        ) : undefined
      }
      onLogout={() => void handleLogout()}
      chrome={!isBareRoute(pathname)}
      bottomNav={!isRunnerRoute(pathname)}
      displayName={displayName}
      avatarUrl={avatarUrl}
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

export const dailyPracticeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice/daily",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: LessonPage,
});

// A learner's own vocabulary. Signed-in only, like the review session: an
// upload belongs to a person, and a guest has none.
export const myWordsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice/my-words",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: MyWordsPage,
});

export const myWritingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/my-writing",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: MyWritingPage,
});

export const mySpeakingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/my-speaking",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: MySpeakingPage,
});

// The learner's own uploaded material. Signed-in only: a resource belongs to a
// person, and another learner's id answers 404.
export const myResourcesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/my-resources",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: MyResourcesPage,
});

export const resourceDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/my-resources/$resourceId",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ResourceDetailPage,
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
    if (user?.role !== "admin" && user?.role !== "moderator") {
      throw redirect({ to: "/" });
    }
  },
  component: AdminPage,
});

/**
 * One route per administrative section, so the sidebar has something to link to
 * and a reload lands where the reader was. The same page renders them: which
 * section is on screen is read from the path, not held in component state.
 */
export const adminSectionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/$section",
  beforeLoad: () => {
    const { status, user } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
    if (user?.role !== "admin" && user?.role !== "moderator") {
      throw redirect({ to: "/" });
    }
  },
  component: AdminPage,
});

export const studioRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/studio",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: StudioPage,
});

export const studioNewCourseRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/studio/courses/new",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: StudioEditorPage,
});

export const studioEditCourseRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/studio/courses/$draftId",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: StudioEditorPage,
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

export const examsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/exams",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ExamsPage,
});

export const examSittingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/exams/$attemptId",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ExamSittingPage,
});

export const examReportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/exams/$attemptId/report",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: ExamReportPage,
});

export const welcomeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/welcome",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: WelcomePage,
});

export const placementRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/placement",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: PlacementPage,
});

export const foundationTopicRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/foundation/topics/$code",
  // Open to a visitor with no account (ADR-0025). Shows personalized mastery if signed in.
  component: FoundationTopicPage,
});

export const routeTree = rootRoute.addChildren([
  homeRoute,
  welcomeRoute,
  placementRoute,
  foundationTopicRoute,
  learnRoute,
  lessonRoute,

  practiceRoute,
  dailyPracticeRoute,
  reviewRoute,
  myWordsRoute,
  myWritingRoute,
  mySpeakingRoute,
  myResourcesRoute,
  resourceDetailRoute,
  examsRoute,
  examSittingRoute,
  examReportRoute,
  progressRoute,
  settingsRoute,
  studioRoute,
  studioNewCourseRoute,
  studioEditCourseRoute,
  adminRoute,
  adminSectionRoute,
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
