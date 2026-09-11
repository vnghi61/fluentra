import React from "react";
import { Link } from "@tanstack/react-router";
import { LogOut, PenTool, Settings, ShieldCheck, UserRound } from "lucide-react";
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
  /** The learner's own name, when the profile has loaded. */
  displayName?: string | undefined;
  /** The learner's uploaded avatar, when there is one. */
  avatarUrl?: string | undefined;
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
  displayName,
  avatarUrl,
  onLogout,
}: AccountMenuProps): React.JSX.Element {
  const { t } = useTranslation();

  // An uploaded avatar, else the learner's initial, else the generic icon.
  // The initial is the middle rung on purpose: it is theirs, it is legible at
  // 44 px, and it distinguishes two accounts on the same browser — which the
  // person icon never did.
  const initial = displayName?.trim()?.charAt(0)?.toUpperCase();

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
        {avatarUrl !== undefined && avatarUrl !== "" ? (
          <img
            src={avatarUrl}
            alt=""
            // alt="" and aria-hidden: the trigger already carries the
            // accessible name above, and a second one here would read the
            // learner their own name twice on the way into their own menu.
            aria-hidden="true"
            className="h-full w-full rounded-full object-cover"
          />
        ) : initial !== undefined && initial !== "" ? (
          <span className="text-sm font-bold" aria-hidden="true">
            {initial}
          </span>
        ) : (
          <UserRound className="h-5 w-5" aria-hidden="true" />
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {/*
          A greeting, not a role dump.

          This read `Role: USER`, which tells a learner something true and
          useless: every learner is a user, so the line was noise for everyone
          except the handful of administrators. Their own name is what belongs
          at the top of their own menu.

          The role still shows for an administrator, because for them it is a
          real distinction and a reminder of which account they are in.
        */}
        <DropdownMenuLabel>
          <span className="text-text-muted font-normal">
            {t("nav.greeting", "Hello")}
          </span>{" "}
          <span className="font-semibold text-text">
            {displayName?.trim() || t("nav.greetingFallback", "there")}
          </span>
          {role === "admin" && (
            <span className="ml-2 rounded-md bg-primary/10 px-1.5 py-0.5 text-[10px] font-bold uppercase text-primary-accent">
              {role}
            </span>
          )}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/settings">
            <Settings className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("nav.settings", "Settings")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/my-writing">
            <PenTool className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("nav.myWriting", "My Writing")}
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
