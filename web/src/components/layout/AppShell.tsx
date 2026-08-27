import React from "react";
import { Link } from "@tanstack/react-router";
import {
  BookOpen,
  LayoutDashboard,
  LineChart,
  LogIn,
  LogOut,
  Settings,
  ShieldCheck,
  Target,
  UserPlus,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { BrandMark } from "./BrandMark";

export interface AppShellProps {
  children: React.ReactNode;
  user?: { role: string } | null | undefined;
  status?: "idle" | "authenticated" | "unauthenticated" | undefined;
  onLogout?: (() => void) | undefined;
  /**
   * Whether to draw the app frame at all.
   *
   * The shell wraps the root route, so /login and /register inherited a sidebar
   * and a bottom bar advertising four destinations a signed-out visitor cannot
   * reach. The composition root decides; this component only obeys.
   */
  chrome?: boolean;
}

/**
 * Every navigation destination, declared once.
 *
 * The sidebar and the mobile bar used to spell the same five links out twice,
 * in two different sets of hardcoded colours, which is how `Settings` ended up
 * translated in neither. One list, two renderers.
 *
 * `exact` matters only for "/": without it the dashboard link stays active on
 * every route, because every path starts with a slash.
 */
const destinations = [
  {
    to: "/",
    labelKey: "nav.dashboard",
    fallback: "Dashboard",
    Icon: LayoutDashboard,
    exact: true,
  },
  {
    to: "/learn",
    labelKey: "nav.learn",
    fallback: "Learn",
    Icon: BookOpen,
    exact: false,
  },
  {
    to: "/practice",
    labelKey: "nav.practice",
    fallback: "Practice",
    Icon: Target,
    exact: false,
  },
  {
    to: "/progress",
    labelKey: "nav.progress",
    fallback: "Progress",
    Icon: LineChart,
    exact: false,
  },
] as const;

const navBase =
  "flex items-center gap-3 h-11 px-3 rounded-lg min-h-[44px] transition-colors text-sm";
const navIdle = "text-text-muted hover:bg-surface-muted hover:text-text";
const navActive = "bg-primary/10 text-primary-accent font-semibold";

export const AppShell: React.FC<AppShellProps> = ({
  children,
  user,
  status = "idle",
  onLogout,
  chrome = true,
}) => {
  const { t } = useTranslation();
  const signedIn = status === "authenticated";

  // The account controls, in the top-right corner rather than the foot of the
  // sidebar. Sign out stays a visible button rather than living behind an
  // avatar menu: there is no dropdown primitive in this design system yet, and
  // eight E2E specs assert on a visible control. Build the primitive first.
  const account =
    signedIn && user ? (
      <div className="flex items-center gap-2">
        <span className="hidden sm:inline text-xs text-text-muted">
          Role:{" "}
          <span className="font-semibold text-text uppercase">{user.role}</span>
        </span>
        {/*
          aria-label, because the text label is hidden below `sm`. Without it
          the control is an icon and nothing else on a phone: unreadable to a
          screen reader, and invisible to `getByRole("button", { name: ... })`,
          which is how ten journeys assert that a learner is signed in.
        */}
        <button
          type="button"
          onClick={onLogout}
          aria-label={t("nav.signOut", "Sign out")}
          className="flex items-center gap-2 h-10 px-3 rounded-lg text-danger-accent hover:bg-danger/10 min-h-[44px] min-w-[44px] transition-colors text-sm font-medium cursor-pointer"
        >
          <LogOut className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="hidden sm:inline">
            {t("nav.signOut", "Sign out")}
          </span>
        </button>
      </div>
    ) : (
      <div className="flex items-center gap-2">
        <Link
          to="/register"
          aria-label={t("nav.createAccount", "Create account")}
          className="flex items-center gap-2 h-10 px-3 rounded-lg border border-border hover:bg-surface-muted text-text min-h-[44px] transition-colors text-sm font-medium"
        >
          <UserPlus className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="hidden sm:inline">
            {t("nav.createAccount", "Create account")}
          </span>
        </Link>
        <Link
          to="/login"
          className="flex items-center gap-2 h-10 px-4 rounded-full bg-primary hover:bg-primary-hover text-primary-fg min-h-[44px] transition-colors text-sm font-semibold"
        >
          <LogIn className="h-4 w-4 shrink-0" aria-hidden="true" />
          {t("nav.signIn", "Sign in")}
        </Link>
      </div>
    );

  // Authentication stands on its own page: no navigation to gated routes, and
  // nothing competing with the form. LoginForm and RegisterForm already link to
  // each other, so no header is needed to get between them.
  if (!chrome) {
    return (
      <div className="min-h-screen bg-surface-muted text-text flex flex-col">
        <main className="flex-1 flex flex-col justify-center p-4 w-full">
          {children}
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface-muted text-text flex flex-col md:flex-row">
      {/* Desktop Sidebar Navigation */}
      <aside className="hidden md:flex flex-col w-64 shrink-0 bg-surface-card border-r border-border-subtle p-4 gap-6 justify-between">
        <div className="flex flex-col gap-6">
          <Link to="/" className="flex items-center gap-2.5">
            {/* Inline, so the mark takes --color-brand and needs no request. */}
            <BrandMark className="h-7 w-7 shrink-0 text-brand" />
            <span className="text-lg font-bold tracking-tight text-text">
              Fluentra
            </span>
          </Link>

          <nav className="flex flex-col gap-1">
            {destinations.map(({ to, labelKey, fallback, Icon, exact }) => (
              <Link
                key={to}
                to={to}
                activeOptions={{ exact }}
                className={`${navBase} ${navIdle}`}
                activeProps={{ className: `${navBase} ${navActive}` }}
              >
                <Icon
                  className="h-[18px] w-[18px] shrink-0"
                  aria-hidden="true"
                />
                {t(labelKey, fallback)}
              </Link>
            ))}

            {signedIn && (
              <Link
                to="/settings"
                className={`${navBase} ${navIdle}`}
                activeProps={{ className: `${navBase} ${navActive}` }}
              >
                <Settings
                  className="h-[18px] w-[18px] shrink-0"
                  aria-hidden="true"
                />
                {t("nav.settings", "Settings")}
              </Link>
            )}

            {signedIn && user?.role === "admin" && (
              <Link
                to="/admin"
                className={`${navBase} ${navIdle}`}
                activeProps={{ className: `${navBase} ${navActive}` }}
              >
                <ShieldCheck
                  className="h-[18px] w-[18px] shrink-0"
                  aria-hidden="true"
                />
                {t("nav.admin", "Admin")}
              </Link>
            )}
          </nav>
        </div>
      </aside>

      {/* Header + content column */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="sticky top-0 z-40 flex items-center justify-between gap-3 h-14 px-4 bg-surface-card border-b border-border-subtle">
          {/* The brand belongs here only where the sidebar is not drawing it. */}
          {/*
            min-h-[44px]: the mark is 26px and the word sits beside it, so
            without a floor this link was 94x26 — a tap target under the 44x44
            minimum ADR-0024 sets, and the narrow-320 suite caught it at 320px
            in both locales.
          */}
          <Link
            to="/"
            className="flex items-center gap-2 min-h-[44px] md:hidden"
          >
            <BrandMark className="h-[26px] w-[26px] shrink-0 text-brand" />
            <span className="text-base font-bold tracking-tight text-text">
              Fluentra
            </span>
          </Link>
          <div className="hidden md:block" />
          {account}
        </header>

        <main className="flex-1 p-4 pb-20 md:pb-4 max-w-7xl mx-auto w-full">
          {children}
        </main>
      </div>

      {/*
        Mobile Bottom Navigation Bar — the four learner destinations and nothing
        else. Settings, Admin and Sign out moved to the header, which is where
        the IA puts them and which keeps the bar at four thumb-sized targets
        instead of seven.
      */}
      <nav className="md:hidden fixed bottom-0 left-0 right-0 h-16 bg-surface-card border-t border-border-subtle flex items-center justify-around z-50 px-2 pb-[env(safe-area-inset-bottom)]">
        {destinations.map(({ to, labelKey, fallback, Icon, exact }) => (
          <Link
            key={to}
            to={to}
            activeOptions={{ exact }}
            className="flex flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] text-text-muted text-[11px] font-medium"
            activeProps={{
              className:
                "flex flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] text-primary-accent text-[11px] font-semibold",
            }}
          >
            <Icon className="h-5 w-5" aria-hidden="true" />
            {t(labelKey, fallback)}
          </Link>
        ))}
      </nav>
    </div>
  );
};
