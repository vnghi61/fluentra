import React from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  BookOpen,
  Flame,
  GraduationCap,
  LayoutDashboard,
  LineChart,
  LogIn,
  MoreHorizontal,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  ShieldCheck,
  Sparkles,
  Target,
  UserPlus,
} from "lucide-react";
import { LazyMotion, m, useReducedMotion } from "motion/react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { NavTooltip, SidebarCollapsedContext } from "@/components/ui/tooltip";
import { BrandMark } from "./BrandMark";

/** Lazy: see AccountMenu's own comment for why it is not in the entry chunk. */
const AccountMenu = React.lazy(() => import("./AccountMenu"));

/**
 * Lazy: secondary destinations sheet for mobile navigation.
 * Keeping it lazy ensures the full bottom-sheet and motion physics are not loaded
 * into the entry chunk.
 */
const MoreSheet = React.lazy(() => import("./MoreSheet"));

const loadFeatures = () => import("./motion-features").then((r) => r.default);

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
  /** The account's email, shown under the name in the account menu. */
  email?: string | undefined;
  /** The learner's uploaded avatar, when the profile has loaded one. */
  avatarUrl?: string | undefined;
  /**
   * The administrative group for the sidebar, passed in for the same reason
   * `controls` is: it is permission filtered, and a component may not reach into
   * a feature. The router supplies it only to an administrator, so no one else
   * pays for the `/me/permissions` read it makes.
   */
  adminNav?: React.ReactNode;
  /**
   * Whether to draw the mobile bottom navigation.
   *
   * The lesson runner is full-screen and distraction-free (P10.3): it carries
   * its own header, exit and progress, and the fixed bar otherwise sits on top
   * of its primary action — the Check button of a four-option exercise lands
   * behind it on a 390 px phone. The router decides; the shell only obeys.
   */
  bottomNav?: boolean;
  /**
   * Pinned slot at the foot of the desktop sidebar, typically used for
   * motivation or learner streak reminders.
   */
  sidebarFooter?: React.ReactNode;
  /**
   * Search slot rendered in the desktop header, preceding controls and account.
   */
  search?: React.ReactNode;
  /**
   * Notifications slot rendered in the header between controls and account.
   */
  notifications?: React.ReactNode;
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
    to: "/exams",
    labelKey: "nav.exams",
    fallback: "Exams",
    Icon: GraduationCap,
    exact: false,
  },
  {
    to: "/progress",
    labelKey: "nav.progress",
    fallback: "Progress",
    Icon: LineChart,
    exact: false,
  },
  {
    to: "/studio",
    labelKey: "nav.studio",
    fallback: "Studio",
    Icon: Sparkles,
    exact: false,
  },
] as const;

// Exported because the administrative group is rendered by the admin feature —
// a feature may import a component, the reverse is what the boundary forbids —
// and a second copy of these strings is how the sidebar ends up with two
// different ideas of what "active" looks like.
export const navBase =
  "flex items-center gap-2.5 h-10 px-3 rounded-xl min-h-[44px] transition-colors text-sm font-medium";
export const navIdle = "text-text-muted hover:bg-surface-muted hover:text-text";
export const navActive =
  "bg-primary/10 text-primary-accent font-semibold rounded-xl";

/**
 * Motivational card slot rendered at the bottom of the desktop sidebar.
 */
export const MotivationCard: React.FC<{ className?: string }> = ({
  className,
}) => {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        "rounded-xl border border-border-subtle bg-surface-card p-2.5 text-text",
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary-accent">
          <Flame className="h-3.5 w-3.5" aria-hidden="true" />
        </div>
        <span className="text-xs font-semibold text-text">
          {t("nav.motivationTitle", "Keep the momentum!")}
        </span>
      </div>
      <p className="mt-1.5 text-xs leading-relaxed text-text-muted">
        {t(
          "nav.motivationBody",
          "Consistent daily practice builds fluency that lasts.",
        )}
      </p>
    </div>
  );
};

export const AppShell: React.FC<AppShellProps> = ({
  children,
  user,
  status = "idle",
  onLogout,
  chrome = true,
  controls,
  banner,
  adminNav,
  displayName,
  email,
  avatarUrl,
  bottomNav = true,
  sidebarFooter,
  search,
  notifications,
}) => {
  const { t } = useTranslation();
  const signedIn = status === "authenticated";
  const reduceMotion = Boolean(useReducedMotion());

  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });

  const [moreOpen, setMoreOpen] = React.useState(false);
  const [hasOpenedMore, setHasOpenedMore] = React.useState(false);

  const [collapsed, setCollapsed] = React.useState<boolean>(() => {
    try {
      return localStorage.getItem("fluentra.sidebarCollapsed") === "true";
    } catch {
      // Private mode denies localStorage; fall back to default expanded.
      return false;
    }
  });

  const toggleSidebar = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem("fluentra.sidebarCollapsed", String(next));
      } catch {
        // Private mode denies localStorage; not being able to remember is not a reason to refuse it.
      }
      return next;
    });
  };

  const settingsLabel = t("nav.settings", "Settings");
  const adminLabel = t("nav.admin", "Admin");

  const handleOpenMore = () => {
    setHasOpenedMore(true);
    setMoreOpen(true);
  };

  const isLearnActive = pathname === "/learn" || pathname.startsWith("/learn/");
  const isPracticeActive =
    pathname === "/practice" || pathname.startsWith("/practice/");
  const isProgressActive =
    pathname === "/progress" || pathname.startsWith("/progress/");
  const isMoreActive =
    pathname.startsWith("/exams") ||
    pathname.startsWith("/studio") ||
    pathname.startsWith("/settings") ||
    pathname.startsWith("/admin");

  // The account controls, in the top-right corner rather than the foot of the
  // sidebar, and behind an avatar once there is an account to show — which is
  // what keeps Sign out out of primary navigation.
  const account =
    signedIn && user ? (
      <React.Suspense
        fallback={<div className="h-11 w-11" aria-hidden="true" />}
      >
        <AccountMenu
          role={user.role}
          displayName={displayName}
          email={email}
          avatarUrl={avatarUrl}
          onLogout={onLogout}
          // Studio endpoints are x-permission: self; its route guard checks authentication only.
          canAccessStudio={signedIn}
        />
      </React.Suspense>
    ) : (
      <div className="flex items-center gap-2">
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
  // nothing competing with the form.
  if (!chrome) {
    return (
      <div className="min-h-screen bg-surface-muted text-text flex flex-col">
        {banner}
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
    <div className="theme-surface min-h-screen bg-surface-muted text-text flex flex-col">
      {banner}
      <div className="flex-1 flex flex-col md:flex-row">
        {/* Desktop Sidebar Navigation */}
        <aside
          data-collapsed={collapsed ? "true" : "false"}
          className={cn(
            "group theme-surface hidden md:flex flex-col shrink-0 bg-surface-card border-r border-border-subtle p-3 md:sticky md:top-0 md:h-screen md:self-start transition-[width] duration-[200ms] ease-in-out motion-reduce:transition-none",
            collapsed ? "w-[72px]" : "w-[240px]",
          )}
        >
          <SidebarCollapsedContext.Provider value={collapsed}>
            <>
              <div
                className={cn(
                  "flex shrink-0 items-center justify-between gap-2 pb-3",
                )}
              >
                <NavTooltip label="Fluentra">
                  <Link
                    to="/"
                    className={cn(
                      "flex items-center gap-2 px-1 min-h-[44px]",
                      collapsed && "justify-center px-0",
                    )}
                  >
                    <BrandMark className="h-6 w-6 shrink-0 text-brand" />
                    <span
                      className={
                        collapsed
                          ? "sr-only"
                          : "text-base font-bold tracking-tight text-brand"
                      }
                    >
                      Fluentra
                    </span>
                  </Link>
                </NavTooltip>
                <NavTooltip
                  label={
                    collapsed
                      ? t("nav.expandSidebar", "Expand sidebar")
                      : t("nav.collapseSidebar", "Collapse sidebar")
                  }
                >
                  <button
                    type="button"
                    onClick={toggleSidebar}
                    aria-label={
                      collapsed
                        ? t("nav.expandSidebar", "Expand sidebar")
                        : t("nav.collapseSidebar", "Collapse sidebar")
                    }
                    aria-expanded={!collapsed}
                    className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-text-muted hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary transition-colors cursor-pointer"
                  >
                    {collapsed ? (
                      <PanelLeftOpen
                        className="h-5 w-5 shrink-0"
                        aria-hidden="true"
                      />
                    ) : (
                      <PanelLeftClose
                        className="h-5 w-5 shrink-0"
                        aria-hidden="true"
                      />
                    )}
                  </button>
                </NavTooltip>
              </div>

              <nav
                aria-label={t("nav.mainNavigation", "Main navigation")}
                className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto [scrollbar-width:thin]"
              >
                {destinations.map(({ to, labelKey, fallback, Icon, exact }) => {
                  const label = t(labelKey, fallback);
                  return (
                    <NavTooltip key={to} label={label}>
                      <Link
                        to={to}
                        activeOptions={{ exact }}
                        className={cn(
                          navBase,
                          navIdle,
                          collapsed && "justify-center px-0",
                        )}
                        activeProps={{
                          className: cn(
                            navBase,
                            navActive,
                            collapsed && "justify-center px-0",
                          ),
                        }}
                      >
                        <Icon
                          className="h-[18px] w-[18px] shrink-0"
                          aria-hidden="true"
                        />
                        <span className={collapsed ? "sr-only" : undefined}>
                          {label}
                        </span>
                      </Link>
                    </NavTooltip>
                  );
                })}

                {signedIn && (
                  <>
                    <hr className="my-2 border-t border-border-subtle" />

                    <NavTooltip label={settingsLabel}>
                      <Link
                        to="/settings"
                        className={cn(
                          navBase,
                          navIdle,
                          collapsed && "justify-center px-0",
                        )}
                        activeProps={{
                          className: cn(
                            navBase,
                            navActive,
                            collapsed && "justify-center px-0",
                          ),
                        }}
                      >
                        <Settings
                          className="h-[18px] w-[18px] shrink-0"
                          aria-hidden="true"
                        />
                        <span className={collapsed ? "sr-only" : undefined}>
                          {settingsLabel}
                        </span>
                      </Link>
                    </NavTooltip>

                    {(user?.role === "admin" || user?.role === "moderator") &&
                      (adminNav ?? (
                        <NavTooltip label={adminLabel}>
                          <Link
                            to="/admin"
                            className={cn(
                              navBase,
                              navIdle,
                              collapsed && "justify-center px-0",
                            )}
                            activeProps={{
                              className: cn(
                                navBase,
                                navActive,
                                collapsed && "justify-center px-0",
                              ),
                            }}
                          >
                            <ShieldCheck
                              className="h-[18px] w-[18px] shrink-0"
                              aria-hidden="true"
                            />
                            <span className={collapsed ? "sr-only" : undefined}>
                              {adminLabel}
                            </span>
                          </Link>
                        </NavTooltip>
                      ))}
                  </>
                )}
              </nav>

              {!collapsed && (sidebarFooter || signedIn) && (
                <div className="shrink-0 pt-3">
                  {sidebarFooter ?? <MotivationCard />}
                </div>
              )}
            </>
          </SidebarCollapsedContext.Provider>
        </aside>

        {/* Header + content column */}
        <div className="flex-1 flex flex-col min-w-0">
          <header className="theme-surface sticky top-0 z-40 flex items-center justify-between gap-3 h-14 md:h-16 px-4 md:px-6 bg-surface-card border-b border-border-subtle">
            {/* The brand belongs here only where the sidebar is not drawing it. */}
            <Link
              to="/"
              className="flex items-center gap-2 min-h-[44px] md:hidden"
            >
              <BrandMark className="h-6 w-6 shrink-0 text-brand" />
              <span className="text-base font-bold tracking-tight text-brand">
                Fluentra
              </span>
            </Link>

            {/* Desktop search slot */}
            {search ? (
              <div className="hidden md:flex flex-1 items-center max-w-md">
                {search}
              </div>
            ) : (
              <div className="hidden md:block" />
            )}

            <div className="flex items-center gap-1 sm:gap-2">
              <div className={signedIn ? "hidden md:flex" : "flex"}>
                {controls}
              </div>
              {notifications}
              {account}
            </div>
          </header>

          <main
            className={cn(
              "flex-1 min-w-0 w-[min(100%,1440px)] p-4 md:p-6 xl:px-8 min-[1600px]:max-w-[1520px] mx-auto",
              bottomNav
                ? "pb-[calc(4rem+env(safe-area-inset-bottom)+16px)] md:pb-6"
                : "pb-4 md:pb-6",
            )}
          >
            {children}
          </main>
        </div>
      </div>

      {/*
        Mobile Bottom Navigation Bar — exactly 5 items:
        4 links (Dashboard, Learn, Practice, Progress) and 1 More button.
      */}
      {bottomNav && (
        <LazyMotion features={loadFeatures}>
          <nav
            aria-label={t("nav.mobileNavigation", "Mobile navigation")}
            className="md:hidden fixed bottom-0 left-0 right-0 h-[calc(4rem+env(safe-area-inset-bottom))] bg-surface-card border-t border-border-subtle flex items-center justify-around z-50 px-2 pb-[env(safe-area-inset-bottom)]"
          >
            {/* 1. Dashboard */}
            <Link
              to="/"
              className={cn(
                "relative flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-[11px] transition-colors",
                pathname === "/"
                  ? "text-primary-accent font-semibold"
                  : "text-text-muted hover:text-text font-medium",
              )}
            >
              {pathname === "/" && (
                <m.span
                  {...(reduceMotion
                    ? {}
                    : { layoutId: "bottom-nav-indicator" })}
                  className="absolute top-1 h-1 w-6 rounded-full bg-primary"
                  aria-hidden="true"
                  transition={{ type: "spring", damping: 30, stiffness: 350 }}
                />
              )}
              <LayoutDashboard
                className="h-5 w-5 shrink-0"
                aria-hidden="true"
              />
              <span className="max-w-[64px] truncate">
                {t("nav.dashboard", "Dashboard")}
              </span>
            </Link>

            {/* 2. Learn */}
            <Link
              to="/learn"
              className={cn(
                "relative flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-[11px] transition-colors",
                isLearnActive
                  ? "text-primary-accent font-semibold"
                  : "text-text-muted hover:text-text font-medium",
              )}
            >
              {isLearnActive && (
                <m.span
                  {...(reduceMotion
                    ? {}
                    : { layoutId: "bottom-nav-indicator" })}
                  className="absolute top-1 h-1 w-6 rounded-full bg-primary"
                  aria-hidden="true"
                  transition={{ type: "spring", damping: 30, stiffness: 350 }}
                />
              )}
              <BookOpen className="h-5 w-5 shrink-0" aria-hidden="true" />
              <span className="max-w-[64px] truncate">
                {t("nav.learn", "Learn")}
              </span>
            </Link>

            {/* 3. Practice */}
            <Link
              to="/practice"
              className={cn(
                "relative flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-[11px] transition-colors",
                isPracticeActive
                  ? "text-primary-accent font-semibold"
                  : "text-text-muted hover:text-text font-medium",
              )}
            >
              {isPracticeActive && (
                <m.span
                  {...(reduceMotion
                    ? {}
                    : { layoutId: "bottom-nav-indicator" })}
                  className="absolute top-1 h-1 w-6 rounded-full bg-primary"
                  aria-hidden="true"
                  transition={{ type: "spring", damping: 30, stiffness: 350 }}
                />
              )}
              <Target className="h-5 w-5 shrink-0" aria-hidden="true" />
              <span className="max-w-[64px] truncate">
                {t("nav.practice", "Practice")}
              </span>
            </Link>

            {/* 4. Progress */}
            <Link
              to="/progress"
              className={cn(
                "relative flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-[11px] transition-colors",
                isProgressActive
                  ? "text-primary-accent font-semibold"
                  : "text-text-muted hover:text-text font-medium",
              )}
            >
              {isProgressActive && (
                <m.span
                  {...(reduceMotion
                    ? {}
                    : { layoutId: "bottom-nav-indicator" })}
                  className="absolute top-1 h-1 w-6 rounded-full bg-primary"
                  aria-hidden="true"
                  transition={{ type: "spring", damping: 30, stiffness: 350 }}
                />
              )}
              <LineChart className="h-5 w-5 shrink-0" aria-hidden="true" />
              <span className="max-w-[64px] truncate">
                {t("nav.progress", "Progress")}
              </span>
            </Link>

            {/* 5. More */}
            <button
              type="button"
              aria-haspopup="dialog"
              aria-expanded={moreOpen}
              onClick={handleOpenMore}
              className={cn(
                "relative flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-[11px] transition-colors cursor-pointer",
                isMoreActive
                  ? "text-primary-accent font-semibold"
                  : "text-text-muted hover:text-text font-medium",
              )}
            >
              {isMoreActive && (
                <m.span
                  {...(reduceMotion
                    ? {}
                    : { layoutId: "bottom-nav-indicator" })}
                  className="absolute top-1 h-1 w-6 rounded-full bg-primary"
                  aria-hidden="true"
                  transition={{ type: "spring", damping: 30, stiffness: 350 }}
                />
              )}
              <MoreHorizontal className="h-5 w-5 shrink-0" aria-hidden="true" />
              <span className="max-w-[64px] truncate">
                {t("nav.more", "More")}
              </span>
            </button>
          </nav>
        </LazyMotion>
      )}

      {hasOpenedMore && (
        <React.Suspense fallback={null}>
          <MoreSheet
            open={moreOpen}
            onOpenChange={setMoreOpen}
            signedIn={signedIn}
            isAdmin={
              signedIn && (user?.role === "admin" || user?.role === "moderator")
            }
            controls={controls}
          />
        </React.Suspense>
      )}
    </div>
  );
};

AppShell.displayName = "AppShell";
