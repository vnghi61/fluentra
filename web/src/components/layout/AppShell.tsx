import React from "react";
import { flushSync } from "react-dom";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  BookOpen,
  Flame,
  GraduationCap,
  LayoutDashboard,
  LineChart,
  LogIn,
  MoreHorizontal,
  ChevronLeft,
  ChevronsLeft,
  ChevronsRight,
  Settings,
  ShieldCheck,
  Sparkles,
  Target,
  UserPlus,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Locale } from "@/i18n";
import type { ThemeChoice } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { withPageTransition, type RevealOrigin } from "@/lib/view-transition";
import { NavTooltip, SidebarCollapsedContext } from "@/components/ui/tooltip";
import { BrandMark } from "./BrandMark";
import { INDICATOR_ATTR, useActiveIndicator } from "./useActiveIndicator";

/** Lazy: see AccountMenu's own comment for why it is not in the entry chunk. */
const AccountMenu = React.lazy(() => import("./AccountMenu"));

/**
 * Lazy: secondary destinations sheet for mobile navigation.
 * Keeping it lazy keeps the bottom sheet out of the entry chunk.
 */
const mobileBackClass =
  "-ml-2 flex min-h-[44px] min-w-0 max-w-[60vw] items-center gap-1 rounded-lg px-2 text-base font-semibold text-text transition-colors hover:bg-surface-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary md:hidden";

const MoreSheet = React.lazy(() => import("./MoreSheet"));

const bottomTabClass = (active: boolean) =>
  cn(
    "group/tab relative z-10 flex flex-1 flex-col items-center justify-center gap-0.5 min-w-[44px] min-h-[44px] h-full text-center text-xs select-none transition-colors duration-200 [-webkit-tap-highlight-color:transparent] motion-reduce:transition-none",
    active
      ? "text-primary-accent font-semibold"
      : "text-text-muted hover:text-text font-medium",
  );

/** The icon lifts a little on the active tab and dips on press. */
const bottomIconClass = (active: boolean) =>
  cn(
    "h-5 w-5 shrink-0 transition-transform duration-200 ease-out group-active/tab:scale-90 motion-reduce:transition-none",
    active && "-translate-y-0.5",
  );

/**
 * One bar under the tabs that slides to the active one. Measured rather than
 * computed from an index, so it also follows the More tab and survives a tab
 * being added. CSS transform only: no animation library.
 */
function SlidingTabIndicator({
  container,
  depsKey,
}: {
  container: React.RefObject<HTMLElement | null>;
  depsKey: string;
}) {
  const { rect, animate } = useActiveIndicator(container, [depsKey]);
  return (
    <span
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute bottom-1.5 left-0 h-1 w-8 rounded-full bg-primary",
        animate &&
          "transition-[transform,opacity] duration-[var(--motion-nav)] ease-[var(--ease-nav)] motion-reduce:transition-none",
        rect ? "opacity-100" : "opacity-0",
      )}
      style={{
        transform: rect
          ? `translateX(${rect.x + rect.width / 2 - 16}px)`
          : undefined,
      }}
    />
  );
}

/**
 * The highlight behind the active sidebar item, sliding between items. Items
 * keep their own colour change; the pill is the only thing that moves.
 */
function SlidingSidebarIndicator({
  container,
  depsKey,
  layoutKey,
}: {
  container: React.RefObject<HTMLElement | null>;
  depsKey: string;
  /** A re-layout to follow at once rather than slide through (collapse). */
  layoutKey: unknown;
}) {
  const { rect, animate } = useActiveIndicator(
    container,
    [depsKey],
    INDICATOR_ATTR,
    layoutKey,
  );
  if (!rect) return null;
  return (
    <span
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute left-0 top-0 rounded-xl bg-primary/10",
        "before:absolute before:left-0 before:top-1/2 before:h-5 before:w-1 before:-translate-y-1/2 before:rounded-r-full before:bg-primary",
        animate &&
          "transition-[transform,width,height] duration-[var(--motion-nav)] ease-[var(--ease-nav)] motion-reduce:transition-none",
      )}
      style={{
        transform: `translate(${rect.x}px, ${rect.y}px)`,
        width: rect.width,
        height: rect.height,
      }}
    />
  );
}

function BottomNavLink({
  to,
  active,
  icon: Icon,
  label,
}: {
  to: string;
  active: boolean;
  icon: React.ComponentType<{
    className?: string;
    "aria-hidden"?: boolean | "true";
  }>;
  label: string;
}) {
  return (
    <Link
      to={to}
      preload="intent"
      viewTransition
      aria-current={active ? "page" : undefined}
      {...{ [INDICATOR_ATTR]: active ? "true" : "false" }}
      // Tapping the tab you are on does nothing, rather than replaying the
      // page transition.
      onClick={(e) => {
        if (active) e.preventDefault();
      }}
      className={bottomTabClass(active)}
    >
      <Icon className={bottomIconClass(active)} aria-hidden="true" />
      <span className="max-w-[64px] truncate">{label}</span>
    </Link>
  );
}

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
  /** Theme and language state for the mobile More screen's own pickers. */
  themeChoice?: ThemeChoice | undefined;
  onThemeChoice?:
    ((choice: ThemeChoice, origin?: RevealOrigin) => void) | undefined;
  locale?: Locale | undefined;
  onLocale?: ((locale: Locale) => void) | undefined;
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
/**
 * Sidebar items. The active highlight is `SlidingSidebarIndicator`, drawn once
 * behind the list, so an item itself only changes colour; hover gives idle
 * items a tint, a nudged icon and a press dip.
 */
export const navBase =
  "relative z-10 flex items-center gap-2.5 h-10 px-[15px] overflow-hidden whitespace-nowrap rounded-xl min-h-[44px] text-sm font-medium transition-[background-color,color,transform] duration-200 ease-out active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary [&>svg]:transition-transform [&>svg]:duration-200 [&>svg]:ease-out motion-reduce:transition-none motion-reduce:active:scale-100 motion-reduce:[&>svg]:transition-none";
export const navIdle =
  "text-text-muted hover:bg-surface-muted hover:text-text hover:[&>svg]:scale-110 hover:[&>svg]:text-primary-accent";
export const navActive = "text-primary-accent font-semibold";

/**
 * A sidebar label. Faded rather than switched to `sr-only`, and the item keeps
 * the same padding in both states, so collapsing animates one width and
 * nothing jumps: the icon never moves, the text fades out over it.
 */
export const navLabel = (collapsed: boolean) =>
  cn(
    "min-w-0 truncate transition-[opacity,transform] duration-[var(--motion-nav)] ease-[var(--ease-nav)] motion-reduce:transition-none",
    collapsed && "pointer-events-none -translate-x-1 opacity-0",
  );

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
          <Flame className="h-4 w-4" aria-hidden="true" />
        </div>
        <span className="text-sm font-semibold text-text">
          {t("nav.motivationTitle", "Keep the momentum!")}
        </span>
      </div>
      <p className="mt-1.5 text-sm leading-relaxed text-text-muted">
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
  themeChoice,
  onThemeChoice,
  locale,
  onLocale,
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
  // A known visitor has no dashboard: "/" sends them to /learn (ADR-0025). A
  // Dashboard tab for them lit up, redirected, and the highlight jumped to
  // Learn beside it, which read as tapping one tab and getting the next. While
  // the session is still unknown (`idle`, a cold host) the tab stays, so a
  // returning learner's bar does not change under them.
  const guest = status === "unauthenticated";
  const homePath = guest ? "/learn" : "/";
  const navItems = guest
    ? destinations.filter(({ to }) => to !== "/")
    : destinations;

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

  // One DOM change inside a view transition: the browser morphs the before
  // and after snapshots on the compositor (index.css, `data-vt="sidebar"`).
  // Transitioning the width instead re-laid-out the sidebar and the whole page
  // column on every frame, re-wrapped the motivation card as it grew, and made
  // the indicators chase the items through a ResizeObserver a frame behind.
  // Where view transitions are missing the width transition below remains.
  const toggleSidebar = () => {
    const next = !collapsed;
    try {
      localStorage.setItem("fluentra.sidebarCollapsed", String(next));
    } catch {
      // Private mode denies localStorage; not being able to remember is not a reason to refuse it.
    }
    void withPageTransition(
      "sidebar",
      () => {
        // Committed synchronously, so the "after" snapshot is the new layout.
        flushSync(() => setCollapsed(next));
      },
      { dir: next ? "collapse" : "expand" },
    );
  };

  const settingsLabel = t("nav.settings", "Settings");
  const adminLabel = t("nav.admin", "Admin");

  const moreButtonRef = React.useRef<HTMLButtonElement>(null);
  const sidebarNavRef = React.useRef<HTMLElement>(null);
  const bottomTabsRef = React.useRef<HTMLDivElement>(null);
  // The pages the More menu leads to. On one of them the More tab is already
  // the current tab.
  const onMoreRoute =
    pathname.startsWith("/exams") ||
    pathname.startsWith("/studio") ||
    pathname.startsWith("/settings") ||
    pathname.startsWith("/admin");
  const handleOpenMore = () => {
    // Tapping the current tab keeps what is on screen, as it does for every
    // other tab: on the More screen it stays (closing it uncovered the page
    // beneath, and the highlight jumped to that page's tab), and on a page the
    // More menu leads to it stays on that page rather than covering it.
    if (moreOpen || onMoreRoute) return;
    setHasOpenedMore(true);
    setMoreOpen(true);
  };

  const isLearnActive =
    !moreOpen && (pathname === "/learn" || pathname.startsWith("/learn/"));
  // While the More screen is open it is the current tab, whatever the route.
  const isDashboardActive = !moreOpen && pathname === "/";
  const isPracticeActive =
    !moreOpen &&
    (pathname === "/practice" || pathname.startsWith("/practice/"));
  const isProgressActive =
    !moreOpen &&
    (pathname === "/progress" || pathname.startsWith("/progress/"));
  const isMoreActive = moreOpen || onMoreRoute;

  // On a phone the pages the More menu leads to have no tab of their own to
  // go back through, so the header offers a way up: from a section's root
  // (Settings, Exams, Studio, Admin) back to the More menu, and from a page
  // inside one back to that section's root.
  const moreSegments = onMoreRoute ? pathname.split("/").filter(Boolean) : [];
  const moreSection = moreSegments[0];
  const sectionLabel: Record<string, string> = {
    exams: t("nav.exams"),
    studio: t("nav.studio"),
    settings: t("nav.settings"),
    admin: t("nav.admin"),
  };
  const mobileBack =
    moreSection === undefined ? null : moreSegments.length === 1 ? (
      <button
        type="button"
        onClick={() => {
          setHasOpenedMore(true);
          setMoreOpen(true);
        }}
        aria-label={t("nav.backTo", { place: t("nav.more") })}
        className={mobileBackClass}
      >
        <ChevronLeft className="h-5 w-5 shrink-0" aria-hidden="true" />
        <span className="truncate">{t("nav.more")}</span>
      </button>
    ) : (
      <Link
        to={`/${moreSection}`}
        aria-label={t("nav.backTo", {
          place: sectionLabel[moreSection] ?? moreSection,
        })}
        className={mobileBackClass}
      >
        <ChevronLeft className="h-5 w-5 shrink-0" aria-hidden="true" />
        <span className="truncate">
          {sectionLabel[moreSection] ?? moreSection}
        </span>
      </Link>
    );

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
      <div className="flex-1 flex flex-col md:flex-row">
        {/* Desktop Sidebar Navigation */}
        <aside
          data-collapsed={collapsed ? "true" : "false"}
          className={cn(
            "app-vt-sidebar group theme-surface hidden md:flex flex-col shrink-0 overflow-hidden bg-surface-card border-r border-border-subtle p-3 md:sticky md:top-0 md:h-screen md:self-start transition-[width] duration-[var(--motion-nav)] ease-[var(--ease-nav)] motion-reduce:transition-none",
            collapsed ? "w-[72px]" : "w-[240px]",
          )}
        >
          <SidebarCollapsedContext.Provider value={collapsed}>
            <>
              <div
                className={cn(
                  "flex shrink-0 items-center justify-between gap-2 pb-3",
                  // 72px holds one 44px target, not two side by side.
                  collapsed && "flex-col justify-start gap-1",
                )}
              >
                <NavTooltip label="Fluentra">
                  <Link
                    to={homePath}
                    className={cn(
                      "flex items-center gap-2 px-1 min-h-[44px]",
                      collapsed && "justify-center px-0",
                    )}
                  >
                    <BrandMark className="app-vt-brand h-6 w-6 shrink-0 text-brand" />
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
                    className="group/toggle flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  >
                    {/* A quiet 32px chip inside the 44px target. */}
                    <span className="app-vt-toggle flex h-8 w-8 items-center justify-center rounded-lg border border-border-subtle bg-surface-card text-text-muted shadow-xs transition-colors group-hover/toggle:border-primary/30 group-hover/toggle:bg-primary/5 group-hover/toggle:text-primary-accent motion-reduce:transition-none">
                      {collapsed ? (
                        <ChevronsRight
                          className="h-4 w-4 shrink-0"
                          aria-hidden="true"
                        />
                      ) : (
                        <ChevronsLeft
                          className="h-4 w-4 shrink-0"
                          aria-hidden="true"
                        />
                      )}
                    </span>
                  </button>
                </NavTooltip>
              </div>

              <nav
                ref={sidebarNavRef}
                aria-label={t("nav.mainNavigation", "Main navigation")}
                className="app-vt-nav relative flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto overscroll-contain [scrollbar-gutter:stable] [scrollbar-width:none] hover:[scrollbar-width:thin] focus-within:[scrollbar-width:thin] [&::-webkit-scrollbar]:w-0 hover:[&::-webkit-scrollbar]:w-1.5"
              >
                <SlidingSidebarIndicator
                  container={sidebarNavRef}
                  depsKey={pathname}
                  layoutKey={collapsed}
                />
                {navItems.map(({ to, labelKey, fallback, Icon, exact }) => {
                  const label = t(labelKey, fallback);
                  return (
                    <NavTooltip key={to} label={label}>
                      <Link
                        to={to}
                        preload="intent"
                        viewTransition
                        activeOptions={{ exact }}
                        className={cn(navBase, navIdle)}
                        activeProps={{
                          [INDICATOR_ATTR]: "true",
                          className: cn(navBase, navActive),
                        }}
                      >
                        <Icon
                          className="h-[18px] w-[18px] shrink-0"
                          aria-hidden="true"
                        />
                        <span className={navLabel(collapsed)}>{label}</span>
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
                        preload="intent"
                        viewTransition
                        className={cn(navBase, navIdle)}
                        activeProps={{
                          [INDICATOR_ATTR]: "true",
                          className: cn(navBase, navActive),
                        }}
                      >
                        <Settings
                          className="h-[18px] w-[18px] shrink-0"
                          aria-hidden="true"
                        />
                        <span className={navLabel(collapsed)}>
                          {settingsLabel}
                        </span>
                      </Link>
                    </NavTooltip>

                    {(user?.role === "admin" || user?.role === "moderator") &&
                      (adminNav ?? (
                        <NavTooltip label={adminLabel}>
                          <Link
                            to="/admin"
                            preload="intent"
                            viewTransition
                            className={cn(navBase, navIdle)}
                            activeProps={{
                              [INDICATOR_ATTR]: "true",
                              className: cn(navBase, navActive),
                            }}
                          >
                            <ShieldCheck
                              className="h-[18px] w-[18px] shrink-0"
                              aria-hidden="true"
                            />
                            <span className={navLabel(collapsed)}>
                              {adminLabel}
                            </span>
                          </Link>
                        </NavTooltip>
                      ))}
                  </>
                )}
              </nav>

              {!collapsed && (sidebarFooter || signedIn) && (
                // Laid out at the expanded width whatever the sidebar's
                // current width, so a width transition clips it instead of
                // re-wrapping its text on every frame.
                <div className="w-[216px] shrink-0 pt-3">
                  {sidebarFooter ?? <MotivationCard />}
                </div>
              )}
            </>
          </SidebarCollapsedContext.Provider>
        </aside>

        {/* Header + content column */}
        <div className="flex-1 flex flex-col min-w-0">
          <header className="app-vt-header theme-surface sticky top-0 z-40 flex items-center justify-between gap-3 h-14 md:h-16 px-4 md:px-6 bg-surface-card border-b border-border-subtle">
            {/* The brand belongs here only where the sidebar is not drawing it;
              on a page reached from More, a way back takes its place. */}
            {mobileBack ?? (
              <Link
                to={homePath}
                className="flex items-center gap-2 min-h-[44px] md:hidden"
              >
                <BrandMark className="h-6 w-6 shrink-0 text-brand" />
                <span className="text-base font-bold tracking-tight text-brand">
                  Fluentra
                </span>
              </Link>
            )}

            {/* Desktop search slot */}
            {search ? (
              <div className="hidden md:flex flex-1 items-center max-w-md">
                {search}
              </div>
            ) : (
              <div className="hidden md:block" />
            )}

            <div className="app-vt-header-actions flex items-center gap-1 sm:gap-2">
              <div className={signedIn ? "hidden md:flex" : "flex"}>
                {controls}
              </div>
              {notifications}
              {account}
            </div>
          </header>

          <main
            className={cn(
              "app-vt-main flex-1 min-w-0 w-full p-4 md:p-6 xl:px-8",
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
        <nav
          aria-label={t("nav.mobileNavigation", "Mobile navigation")}
          onClick={(event) => {
            // A tab tap leaves the More screen; the More tab only ever opens it.
            if (!(event.target as Element).closest("[data-more-trigger]")) {
              setMoreOpen(false);
            }
          }}
          className="app-vt-bottom-nav md:hidden fixed bottom-0 left-0 right-0 z-50 border-t border-border-subtle bg-surface-card px-2 pb-[env(safe-area-inset-bottom)] [touch-action:manipulation]"
        >
          <div
            ref={bottomTabsRef}
            className="relative flex h-16 items-stretch justify-around"
          >
            {!guest && (
              <BottomNavLink
                to="/"
                active={isDashboardActive}
                icon={LayoutDashboard}
                label={t("nav.dashboard", "Dashboard")}
              />
            )}
            <BottomNavLink
              to="/learn"
              active={isLearnActive}
              icon={BookOpen}
              label={t("nav.learn", "Learn")}
            />
            <BottomNavLink
              to="/practice"
              active={isPracticeActive}
              icon={Target}
              label={t("nav.practice", "Practice")}
            />
            <BottomNavLink
              to="/progress"
              active={isProgressActive}
              icon={LineChart}
              label={t("nav.progress", "Progress")}
            />
            <button
              type="button"
              ref={moreButtonRef}
              data-more-trigger
              {...{ [INDICATOR_ATTR]: isMoreActive ? "true" : "false" }}
              aria-haspopup="dialog"
              aria-expanded={moreOpen}
              onClick={handleOpenMore}
              className={cn(bottomTabClass(isMoreActive), "cursor-pointer")}
            >
              <MoreHorizontal
                className={bottomIconClass(isMoreActive)}
                aria-hidden="true"
              />
              <span className="max-w-[64px] truncate">
                {t("nav.more", "More")}
              </span>
            </button>
            <SlidingTabIndicator
              container={bottomTabsRef}
              depsKey={`${pathname}|${moreOpen}`}
            />
          </div>
        </nav>
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
            themeChoice={themeChoice}
            onThemeChoice={onThemeChoice}
            locale={locale}
            onLocale={onLocale}
            returnFocusRef={moreButtonRef}
          />
        </React.Suspense>
      )}
    </div>
  );
};

AppShell.displayName = "AppShell";
