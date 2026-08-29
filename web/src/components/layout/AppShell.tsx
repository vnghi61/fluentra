import React from "react";
import { Link } from "@tanstack/react-router";
import {
  BookOpen,
  LayoutDashboard,
  LineChart,
  LogIn,
  Settings,
  ShieldCheck,
  Target,
  UserPlus,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { BrandMark } from "./BrandMark";

/** Lazy: see AccountMenu's own comment for why it is not in the entry chunk. */
const AccountMenu = React.lazy(() => import("./AccountMenu"));

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
  /**
   * The theme and language switchers, passed in rather than built here: they
   * read and write the learner's preferences, and a component may not reach
   * into a store or a feature.
   */
  controls?: React.ReactNode;
  /**
   * Status chrome that belongs above everything — the cold-start notice. Passed
   * in for the same reason `controls` is: it reads the API host's state, and a
   * component may not reach into `api`.
   */
  banner?: React.ReactNode;
  /** The learner's own name, for the account menu's greeting. */
  displayName?: string | undefined;
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
  controls,
  banner,
  displayName,
}) => {
  const { t } = useTranslation();
  const signedIn = status === "authenticated";

  // The account controls, in the top-right corner rather than the foot of the
  // sidebar, and behind an avatar once there is an account to show — which is
  // what keeps Sign out out of primary navigation. The menu waited on the
  // dropdown primitive; that now exists, so this is no longer a bare button.
  const account =
    signedIn && user ? (
      <React.Suspense
        fallback={<div className="h-11 w-11" aria-hidden="true" />}
      >
        <AccountMenu
          role={user.role}
          displayName={displayName}
          onLogout={onLogout}
        />
      </React.Suspense>
    ) : (
      <div className="flex items-center gap-2">
        {/*
          min-w-[44px]: below `sm` the label is hidden and only the icon is
          left, which collapsed this to 42 px wide — a floor on the height
          alone does not make a 44x44 target. It went unmeasured until the
          curriculum opened, because signed-out chrome had never appeared on a
          framed screen before: the auth pages draw no header at all.
        */}
        <Link
          to="/register"
          aria-label={t("nav.createAccount", "Create account")}
          className="flex items-center justify-center gap-2 h-10 px-3 rounded-lg border border-border hover:bg-surface-muted text-text min-h-[44px] min-w-[44px] transition-colors text-sm font-medium"
        >
          <UserPlus className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="hidden sm:inline">
            {t("nav.createAccount", "Create account")}
          </span>
        </Link>
        {/*
          The label hides below `sm`, the way Create account's already did, and
          the aria-label is what keeps the control named when it does. Both
          together are 320 px of header the signed-out cluster did not have:
          brand, theme, language and two auth controls pushed the document to
          357 px, and the narrow-320 suite caught it the moment this chrome
          first appeared on a framed screen.

          The aria-label is not optional garnish. Hiding the only text a control
          carries is how a button ends up with no accessible name at all, which
          is a mistake this file has made before.
        */}
        <Link
          to="/login"
          aria-label={t("nav.signIn", "Sign in")}
          className="flex items-center justify-center gap-2 h-10 px-3 sm:px-4 rounded-full bg-primary hover:bg-primary-hover text-primary-fg min-h-[44px] min-w-[44px] transition-colors text-sm font-semibold"
        >
          <LogIn className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="hidden sm:inline">{t("nav.signIn", "Sign in")}</span>
        </Link>
      </div>
    );

  // Authentication stands on its own page: no navigation to gated routes, and
  // nothing competing with the form. LoginForm and RegisterForm already link to
  // each other, so no header is needed to get between them.
  if (!chrome) {
    return (
      <div className="min-h-screen bg-surface-muted text-text flex flex-col">
        {banner}
        {/*
          Theme and language, and nothing else. Removing the shell from the auth
          pages also removed the only way a Vietnamese learner could switch the
          interface to Vietnamese before signing in. These two controls are not
          navigation, so they can come back without bringing the sidebar with
          them.
        */}
        {controls ? (
          <div className="flex justify-end p-3">{controls}</div>
        ) : null}
        <main className="flex-1 flex flex-col justify-center p-4 w-full">
          {children}
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface-muted text-text flex flex-col">
      {banner}
      <div className="flex-1 flex flex-col md:flex-row">
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
            <div className="flex items-center gap-1">
              {controls}
              {account}
            </div>
          </header>

          <main className="flex-1 p-4 pb-20 md:pb-4 max-w-7xl mx-auto w-full">
            {children}
          </main>
        </div>
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
