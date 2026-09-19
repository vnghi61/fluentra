import React from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronDown, ShieldCheck } from "lucide-react";
import { useTranslation } from "react-i18next";

import { navActive, navBase, navIdle } from "@/components/layout/AppShell";

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

  // An administrator whose permissions grant no section is shown no group at
  // all, rather than a heading over an empty list.
  if (sections.length === 0) return null;

  return (
    <div className="space-y-1">
      <Link
        to="/admin"
        className={`${navBase} ${inAdmin ? navActive : navIdle} justify-between`}
      >
        <span className="flex items-center gap-3">
          <ShieldCheck
            className="h-[18px] w-[18px] shrink-0"
            aria-hidden="true"
          />
          {t("nav.admin", "Admin")}
        </span>
        <ChevronDown
          className={`h-4 w-4 shrink-0 transition-transform ${inAdmin ? "" : "-rotate-90"}`}
          aria-hidden="true"
        />
      </Link>

      {/* Expanded whenever the reader is anywhere under /admin, and collapsed
          otherwise. There is no toggle to get out of step with the address bar:
          where you are is what decides. */}
      {inAdmin && (
        <ul className="space-y-1 border-l border-border-subtle pl-3 ml-4">
          {sections.map(({ key, path, labelKey, labelFallback, Icon }) => (
            <li key={key}>
              <Link
                to={path}
                className={`${navBase} ${navIdle} text-[13px]`}
                activeProps={{
                  className: `${navBase} ${navActive} text-[13px]`,
                }}
              >
                <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                {t(labelKey, labelFallback)}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default AdminSidebarNav;
