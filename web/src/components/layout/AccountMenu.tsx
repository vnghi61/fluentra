import React from "react";
import { Link } from "@tanstack/react-router";
import { LogOut, Settings, ShieldCheck, UserRound } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export interface AccountMenuProps {
  role: string;
  onLogout?: (() => void) | undefined;
}

/**
 * The signed-in account menu.
 *
 * Its own module because AppShell is in the entry chunk and Radix's dropdown —
 * popper, focus scope, dismissable layer, portal, presence — costs about 30 kB
 * gzipped, which put the initial download 5.9 kB over the 200 kB budget. It is
 * needed by signed-in learners, after first paint, so AppShell loads it lazily.
 * It still fetches on mount; what it no longer does is sit on the critical path
 * for a visitor looking at the login form.
 */
export default function AccountMenu({
  role,
  onLogout,
}: AccountMenuProps): React.JSX.Element {
  const { t } = useTranslation();

  return (
    <DropdownMenu>
      {/*
          aria-label rather than visible text: the trigger is an avatar, so
          without a name it is an icon and nothing else — unreadable to a screen
          reader, and invisible to getByRole. Radix adds aria-expanded and
          aria-haspopup, and owns the focus trap, Escape and outside-click.
        */}
      <DropdownMenuTrigger
        aria-label={t("nav.account", "Account")}
        className="flex items-center justify-center h-11 w-11 min-h-[44px] min-w-[44px] rounded-full bg-primary/10 text-primary-accent hover:bg-primary/15 transition-colors cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface-card"
      >
        <UserRound className="h-5 w-5" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>
          Role:{" "}
          <span className="font-semibold text-text uppercase">{role}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/settings">
            <Settings className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("nav.settings", "Settings")}
          </Link>
        </DropdownMenuItem>
        {role === "admin" && (
          <DropdownMenuItem asChild>
            <Link to="/admin">
              <ShieldCheck className="h-4 w-4 shrink-0" aria-hidden="true" />
              {t("nav.admin", "Admin")}
            </Link>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        {/* onLogout is optional on the props, and exactOptionalPropertyTypes
              will not let `undefined` through to a required handler. */}
        <DropdownMenuItem destructive onSelect={() => onLogout?.()}>
          <LogOut className="h-4 w-4 shrink-0" aria-hidden="true" />
          {t("nav.signOut", "Sign out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
