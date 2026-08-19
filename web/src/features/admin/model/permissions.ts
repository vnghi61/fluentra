import { useEffect, useState } from "react";

import { apiFetch } from "@/api/client";
import type { components } from "@/types/api";

export type MyPermissions = components["schemas"]["MyPermissions"];

/**
 * The named permissions the admin screens gate on.
 *
 * They are the constants from `internal/modules/rbac/contract/permissions.go`.
 * Hiding a control the caller cannot use is a courtesy — the server re-checks
 * every call regardless, and the operation's `x-permission` in the spec is the
 * authority. Both, always: hiding alone is not a control, and enforcing alone
 * offers buttons that answer 403.
 */
export const PERMISSIONS = {
  userList: "user.list",
  userRead: "user.read",
  userSuspend: "user.suspend",
  userReinstate: "user.reinstate",
  userManageSessions: "user.manage_sessions",
  systemFlags: "system.flags",
} as const;

export interface PermissionState {
  /** Whether the caller holds a named permission. */
  can: (permission: string) => boolean;
  isLoading: boolean;
  /** Non-null when the roles could not be read; controls stay hidden. */
  error: string | null;
}

/**
 * Reads `/me/permissions` once and answers questions about it.
 *
 * On failure it answers "no" to everything. A read that did not happen is not
 * evidence of a permission, and showing an action because a request failed is
 * the wrong direction to fail in.
 */
export function usePermissions(): PermissionState {
  const [granted, setGranted] = useState<Set<string> | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const mine = await apiFetch<MyPermissions>("/api/v1/me/permissions");
        if (!cancelled) setGranted(new Set(mine.permissions));
      } catch (err: unknown) {
        if (!cancelled) {
          setError(
            err instanceof Error
              ? err.message
              : "Could not read your permissions.",
          );
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  return {
    can: (permission: string) => granted?.has(permission) ?? false,
    isLoading,
    error,
  };
}
