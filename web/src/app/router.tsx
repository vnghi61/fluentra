import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
  useNavigate,
  type AnyRoute,
} from "@tanstack/react-router";

import { AppShell } from "@/components/layout/AppShell";
import { authApi } from "@/features/auth";
import { ForgotPasswordPage } from "@/pages/ForgotPasswordPage";
import { LoginPage } from "@/pages/LoginPage";
import { OAuthCallbackPage } from "@/pages/OAuthCallbackPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { HomePage } from "@/routes/HomePage";
import { PracticePage } from "@/routes/PracticePage";
import { useAuthStore } from "@/stores/authStore";

/**
 * The route tree is declared in code rather than generated from the filesystem.
 * File-based routing needs a Vite plugin and a generated route file, and a
 * generated file that nothing yet regenerates in CI is a staleness gate waiting
 * to be discovered — which this repository has already been bitten by once.
 */

function RootApp(): React.JSX.Element {
  const user = useAuthStore((s) => s.user);
  const status = useAuthStore((s) => s.status);
  const navigate = useNavigate();

  const handleLogout = async () => {
    await authApi.logout();
    void navigate({ to: "/login" });
  };

  return (
    <AppShell
      user={user}
      status={status}
      onLogout={() => void handleLogout()}
    >
      <Outlet />
    </AppShell>
  );
}

export const rootRoute = createRootRoute({
  component: RootApp,
});

export const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: HomePage,
});

export const practiceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/practice",
  beforeLoad: () => {
    const { status } = useAuthStore.getState();
    if (status === "unauthenticated") {
      throw redirect({ to: "/login" });
    }
  },
  component: PracticePage,
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
  path: "/auth/callback/google",
  component: OAuthCallbackPage,
});

export const routeTree = rootRoute.addChildren([
  homeRoute,
  practiceRoute,
  loginRoute,
  registerRoute,
  forgotPasswordRoute,
  oauthCallbackRoute,
] as AnyRoute[]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
