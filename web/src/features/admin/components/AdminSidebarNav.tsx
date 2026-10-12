import React from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronDown, ShieldCheck } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  navActive,
  navBase,
  navIdle,
  navLabel,
} from "@/components/layout/AppShell";
import {
  SUB_INDICATOR_ATTR,
  useActiveIndicator,
} from "@/components/layout/useActiveIndicator";
import { NavTooltip, useSidebarCollapsed } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

import { useVisibleAdminSections } from "../model/sections";

/**
 * The administrative group in the app's left sidebar.
 *
 * It lives in the admin feature rather than in AppShell for two reasons. The
 * frame is a `components` element and may not import a feature — the same rule
 * go-arch-lint enforces on the Go side — and the section list is permission
 * filtered, which means reading `/me/permissions`. Mounting it inside AppShell
 * would ask for that on every page load for every visitor, signed out ones
 * included. AppShell takes it as a node, the way it already takes its banner and
 * its controls, and the router only supplies it to an administrator.
 */
export function AdminSidebarNav(): React.JSX.Element | null {
  const { t } = useTranslation();
  const { sections } = useVisibleAdminSections();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });

  const inAdmin = pathname === "/admin" || pathname.startsWith("/admin/");
  const submenuId = React.useId();
  const collapsed = useSidebarCollapsed();
  const railRef = React.useRef<HTMLUListElement>(null);

  // Ten sections can outgrow a laptop-height sidebar. Keep the current one in
  // view rather than leaving it below the fold of a scrolled list.
  React.useEffect(() => {
    const frame = requestAnimationFrame(() => {
      railRef.current
        ?.querySelector<HTMLElement>('[aria-current="page"]')
        ?.scrollIntoView?.({ block: "nearest" });
    });
    return () => cancelAnimationFrame(frame);
  }, [pathname]);
  // null until the user toggles: until then, where the reader is decides.
  const [override, setOverride] = React.useState<boolean | null>(null);
  const open = override ?? inAdmin;

  // An administrator whose permissions grant no section is shown no group at
  // all, rather than a heading over an empty list.
  if (sections.length === 0) return null;

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1 group-data-[collapsed=true]:flex-col">
        <NavTooltip label={t("nav.admin", "Admin")}>
          <Link
            to="/admin"
            preload="intent"
            viewTransition
            data-indicator={inAdmin ? "true" : "false"}
            className={cn(
              navBase,
              inAdmin ? navActive : navIdle,
              "min-w-0 flex-1 group-data-[collapsed=true]:w-full group-data-[collapsed=true]:flex-none",
            )}
          >
            <ShieldCheck
              className="h-[18px] w-[18px] shrink-0"
              aria-hidden="true"
            />
            <span className={navLabel(collapsed)}>
              {t("nav.admin", "Admin")}
            </span>
          </Link>
        </NavTooltip>
        <button
          type="button"
          aria-expanded={open}
          aria-controls={submenuId}
          aria-label={t("nav.adminMenuToggle", "Toggle admin sections")}
          onClick={() => setOverride(!open)}
          className="flex h-11 min-h-[44px] w-11 min-w-[44px] shrink-0 cursor-pointer group-data-[collapsed=true]:hidden items-center justify-center rounded-xl text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          <ChevronDown
            className={`h-4 w-4 shrink-0 transition-transform motion-reduce:transition-none ${open ? "" : "-rotate-90"}`}
            aria-hidden="true"
          />
        </button>
      </div>

      <ul
        ref={railRef}
        id={submenuId}
        hidden={!open}
        className="relative space-y-0.5 border-l border-border-subtle pl-3 ml-[23px] group-data-[collapsed=true]:border-l-0 group-data-[collapsed=true]:pl-0 group-data-[collapsed=true]:ml-0"
      >
        {!collapsed && <RailDot container={railRef} depsKey={pathname} />}
        {sections.map(({ key, path, labelKey, labelFallback, Icon }, index) => {
          const label = t(labelKey, labelFallback);
          // Bare /admin shows the first section the admin may see, so that
          // section is the current one too.
          const active =
            pathname === path ||
            pathname.startsWith(`${path}/`) ||
            (pathname === "/admin" && index === 0);
          return (
            <li key={key} className="group/rail relative">
              {/* Hover marker on the rail; the active dot slides over it. */}
              <span
                aria-hidden="true"
                className="pointer-events-none absolute -left-[16px] top-1/2 h-[7px] w-[7px] -translate-y-1/2 scale-50 rounded-full bg-text-muted opacity-0 transition-[opacity,transform] duration-150 group-hover/rail:scale-100 group-hover/rail:opacity-100 group-data-[collapsed=true]:hidden motion-reduce:transition-none"
              />
              <NavTooltip label={label}>
                <Link
                  to={path}
                  preload="intent"
                  viewTransition
                  aria-current={active ? "page" : undefined}
                  {...{
                    [SUB_INDICATOR_ATTR]: active ? "true" : "false",
                    // Collapsed, the rail is gone and the section icon
                    // carries the main highlight instead.
                    "data-indicator": active && collapsed ? "true" : "false",
                  }}
                  className={cn(
                    navBase,
                    active ? navActive : navIdle,
                    "text-sm",
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                  <span className={navLabel(collapsed)}>{label}</span>
                </Link>
              </NavTooltip>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export default AdminSidebarNav;

/** The active section's dot, sliding along the rail between sections. */
function RailDot({
  container,
  depsKey,
}: {
  container: React.RefObject<HTMLElement | null>;
  depsKey: string;
}): React.JSX.Element | null {
  const { rect, animate } = useActiveIndicator(
    container,
    [depsKey],
    SUB_INDICATOR_ATTR,
  );
  if (!rect) return null;
  return (
    <span
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute left-0 top-0 z-20 h-[9px] w-[9px] rounded-full bg-primary ring-4 ring-primary/15",
        animate &&
          "transition-transform duration-[var(--motion-nav)] ease-[var(--ease-nav)] motion-reduce:transition-none",
      )}
      style={{
        // The rail is the list's left border; centre the dot on it.
        transform: `translate(-5px, ${rect.y + rect.height / 2 - 4.5}px)`,
      }}
    />
  );
}
