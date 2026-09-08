import React, { useEffect, useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  ChevronLeft,
  ChevronRight,
  Eye,
  Loader2,
  Lock,
  Search,
  Trash2,
  Unlock,
  Users,
  AlertCircle,
} from "lucide-react";
import { adminApi, type AdminUserSummary } from "../api/adminApi";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { AdminActionReasonModal } from "./AdminActionReasonModal";
import { AdminUserDetailModal } from "./AdminUserDetailModal";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import type { components } from "@/types/api";

type UserStatus = components["schemas"]["UserStatus"];

/** How many accounts a page holds. The row number is derived from it. */
const PAGE_SIZE = 15;

const columnHelper = createColumnHelper<AdminUserSummary>();

/**
 * What the per-row select can do, beyond opening the account.
 *
 * `suspend` locks login and is undone by `reinstate`. `soft_delete` opens the
 * 30-day erasure grace period and is undone by nobody here — only the account's
 * owner can cancel it — which is why the two are separate entries rather than
 * one "disable" with a severity flag, and why they need separate permissions.
 */
type RowAction = "inspect" | "suspend" | "reinstate" | "soft_delete";

/**
 * The two batch operations.
 *
 * Reinstatement is deliberately absent. Locking and deleting are things an
 * administrator does *to* a set of accounts they have judged together;
 * un-locking is a per-account decision about that account's own case, and a
 * batch reinstate would mostly be used to undo a batch suspend that should not
 * have happened — which is a reason to be careful before, not fast after.
 */
type BulkAction = "suspend" | "soft_delete";

/**
 * Which actions an account in this state can actually take.
 *
 * Offering "suspend" on an already-suspended account, or either write on an
 * account already pending deletion, is the "buttons that answer 403" the
 * PERMISSIONS comment warns about — except worse, because the server would
 * accept some of them and produce a state nobody asked for.
 */
function actionsForStatus(status: AdminUserSummary["status"]): RowAction[] {
  switch (status) {
    case "active":
      return ["inspect", "suspend", "soft_delete"];
    case "suspended":
      return ["inspect", "reinstate", "soft_delete"];
    default:
      // pending_deletion and deleted: nothing left to do but look.
      return ["inspect"];
  }
}

export const AdminUserList: React.FC = () => {
  const { t } = useTranslation();
  const { can } = usePermissions();
  const [users, setUsers] = useState<AdminUserSummary[]>([]);
  // How many accounts match, across every page. `users.length` is the size of
  // one page, and the footer was showing it as though it were the whole set.
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Search & Filter state
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedStatus, setSelectedStatus] = useState<UserStatus | "">("");

  // Cursor Pagination state
  const [currentCursor, setCurrentCursor] = useState<string | undefined>(
    undefined,
  );
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [cursorHistory, setCursorHistory] = useState<(string | undefined)[]>(
    [],
  );

  // Selected user for details modal
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);

  // The row action awaiting its justification. Both writes require a reason of
  // at least ten characters server-side, so the modal is not optional chrome.
  const [pendingAction, setPendingAction] = useState<{
    action: Exclude<RowAction, "inspect">;
    user: AdminUserSummary;
  } | null>(null);

  // Accounts ticked for a batch, by id. Held as a Set rather than a flag on the
  // row so a selection survives the row being re-fetched, and cleared whenever
  // the page changes: a batch that silently spans pages is a batch nobody can
  // see the whole of before confirming it.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkAction, setBulkAction] = useState<BulkAction | null>(null);
  const [bulkFailures, setBulkFailures] = useState<string[] | null>(null);
  // The size of the batch as it was submitted. Read after the fact by the
  // result banner, which cannot use `selected.size` because the selection is
  // cleared by the refetch that follows a batch.
  const [selectedCountAtSubmit, setSelectedCountAtSubmit] = useState(0);

  const fetchUsers = async (cursor?: string) => {
    setIsLoading(true);
    setError(null);

    try {
      // `email_prefix` and `display_name` are separate parameters and the
      // server ANDs them, so one search box has to choose. An "@" means the
      // administrator is typing an address; anything else is a name. Sending
      // both would match only accounts whose name and address both start with
      // the same text, which is nobody.
      const term = searchQuery.trim();
      const res = await adminApi.searchUsers({
        ...(term
          ? term.includes("@")
            ? { email_prefix: term }
            : { display_name: term }
          : {}),
        status: selectedStatus || undefined,
        cursor,
        limit: PAGE_SIZE,
      });

      setUsers(res.items);
      setTotal(res.total);
      setCurrentCursor(cursor);
      setNextCursor(res.next_cursor);
      // A selection belongs to the page it was made on. Carrying it across a
      // page change would let a batch act on rows the administrator can no
      // longer see, and the toolbar count would stop matching the ticks.
      setSelected(new Set());
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : t("admin.failedToLoadLearnersList"),
      );
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    // Reset pagination when filters change
    setCursorHistory([]);
    void fetchUsers(undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedStatus]);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCursorHistory([]);
    void fetchUsers(undefined);
  };

  const handleNextPage = () => {
    if (!nextCursor) return;
    setCursorHistory((prev) => [...prev, currentCursor]);
    void fetchUsers(nextCursor);
  };

  const handlePreviousPage = () => {
    if (cursorHistory.length === 0) return;
    const prevCursor = cursorHistory[cursorHistory.length - 1];
    setCursorHistory((prev) => prev.slice(0, prev.length - 1));
    void fetchUsers(prevCursor);
  };

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: "select",
        header: () => null,
        cell: (info) => {
          const user = info.row.original;
          // Only accounts a batch can actually act on are selectable. A ticked
          // row that every bulk action would skip is a row that makes the count
          // in the toolbar a lie.
          const selectable = actionsForStatus(user.status).some(
            (a) => a === "suspend" || a === "soft_delete",
          );
          if (!selectable) return null;
          return (
            <Checkbox
              checked={selected.has(user.id)}
              aria-label={t("admin.selectRow")}
              onCheckedChange={(checked) =>
                setSelected((prev) => {
                  const next = new Set(prev);
                  if (checked) next.add(user.id);
                  else next.delete(user.id);
                  return next;
                })
              }
            />
          );
        },
      }),
      columnHelper.display({
        id: "rowNumber",
        header: t("common.rowNumber"),
        // Counted across pages, not within one. This list is cursor-paginated,
        // so there is no offset to add — the number of pages already stepped
        // through is what says where the page starts, and a per-page 1..15
        // would give two different learners the same number.
        cell: (info) => (
          <span className="tabular-nums text-text-muted">
            {cursorHistory.length * PAGE_SIZE + info.row.index + 1}
          </span>
        ),
      }),
      columnHelper.accessor("display_name", {
        header: t("admin.learner"),
        cell: (info) => {
          const user = info.row.original;
          return (
            <div className="flex items-center gap-3">
              <div className="h-9 w-9 overflow-hidden rounded-full border border-primary/30 bg-surface-muted flex items-center justify-center text-xs font-bold text-primary-accent shrink-0">
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={user.display_name}
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <span>{user.display_name.charAt(0).toUpperCase()}</span>
                )}
              </div>
              <div className="truncate">
                <p className="text-sm font-medium text-text truncate">
                  {user.display_name}
                </p>
                <p className="text-xs text-text-muted font-mono truncate">
                  {user.id.slice(0, 8)}...
                </p>
              </div>
            </div>
          );
        },
      }),
      columnHelper.accessor("email", {
        header: "Email",
        cell: (info) => (
          <span className="text-xs font-mono text-text-muted">
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor("status", {
        header: t("admin.status"),
        cell: (info) => {
          const status = info.getValue();
          return (
            <span
              className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium uppercase border ${
                status === "active"
                  ? "bg-success/10 text-success-accent border-success/20"
                  : status === "suspended"
                    ? "bg-danger/10 text-danger-accent border-danger/20"
                    : "bg-warning/10 text-warning-accent border-warning/20"
              }`}
            >
              {status}
            </span>
          );
        },
      }),
      columnHelper.accessor("created_at", {
        header: t("admin.joined"),
        cell: (info) => (
          <span className="text-xs text-text-muted">
            {new Date(info.getValue()).toLocaleDateString()}
          </span>
        ),
      }),
      columnHelper.display({
        id: "actions",
        header: t("common.actions"),
        cell: (info) => {
          const user = info.row.original;
          const allowed = actionsForStatus(user.status).filter((action) => {
            if (action === "suspend") return can(PERMISSIONS.userSuspend);
            if (action === "reinstate") return can(PERMISSIONS.userReinstate);
            if (action === "soft_delete") return can(PERMISSIONS.userDelete);
            return true;
          });

          // Icons, one button each. Every button carries a title and an
          // aria-label: an icon with neither is unreadable to a screen reader
          // and a guess to everyone else, and these three are not guessable —
          // "lock" and "delete" look alike at 16 px and do very different things.
          return (
            <div className="flex items-center justify-end gap-1">
              {allowed.map((action) => {
                const Icon =
                  action === "inspect"
                    ? Eye
                    : action === "suspend"
                      ? Lock
                      : action === "reinstate"
                        ? Unlock
                        : Trash2;
                const label =
                  action === "inspect"
                    ? t("admin.inspectAccount")
                    : action === "suspend"
                      ? t("admin.lockLogin")
                      : action === "reinstate"
                        ? t("admin.unlockLogin")
                        : t("admin.softDeleteAccount");
                const destructive = action === "soft_delete";
                return (
                  <Button
                    key={action}
                    type="button"
                    variant="ghost"
                    size="sm"
                    title={label}
                    aria-label={label}
                    onClick={() => {
                      if (action === "inspect") {
                        setSelectedUserId(user.id);
                        return;
                      }
                      setPendingAction({ action, user });
                    }}
                    className={
                      destructive
                        ? "h-11 w-11 min-h-[44px] min-w-[44px] p-0 text-danger-accent hover:bg-danger/10"
                        : "h-11 w-11 min-h-[44px] min-w-[44px] p-0 text-text-muted hover:text-primary-accent hover:bg-primary/10"
                    }
                  >
                    <Icon className="h-4 w-4" aria-hidden="true" />
                  </Button>
                );
              })}
            </div>
          );
        },
      }),
    ],
    [t, can, cursorHistory.length, selected],
  );

  const table = useReactTable({
    data: users,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="space-y-6">
      {/* Search & Filter Header */}
      <div className="rounded-xl border border-border-subtle bg-surface-card/60 p-4 space-y-4">
        <form
          onSubmit={handleSearchSubmit}
          className="flex flex-col sm:flex-row gap-3"
        >
          <div className="relative flex-1">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-text-muted" />
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t("admin.searchByNameOrEmail")}
              className="pl-10"
            />
          </div>

          <select
            value={selectedStatus}
            onChange={(e) =>
              setSelectedStatus(e.target.value as UserStatus | "")
            }
            className="h-11 min-h-[44px] rounded-lg border border-border-subtle bg-surface-card px-3 text-base md:text-xs text-text focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="">{t("admin.allStatuses")}</option>
            <option value="active">{t("admin.active")}</option>
            <option value="suspended">{t("admin.suspended")}</option>
            <option value="pending_deletion">
              {t("admin.pendingDeletion")}
            </option>
            <option value="deleted">{t("admin.deleted")}</option>
          </select>

          <Button type="submit" size="md">
            <Search className="mr-1.5 h-4 w-4" />
            {t("admin.search")}
          </Button>
        </form>
      </div>

      {selected.size > 0 && (
        <div className="flex flex-col gap-3 rounded-xl border border-primary/30 bg-primary/5 p-3 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-sm font-medium text-text">
            {t("admin.bulkAction", { count: selected.size })}
          </span>
          <div className="flex items-center gap-2">
            <select
              value=""
              aria-label={t("admin.bulkAction", { count: selected.size })}
              onChange={(e) => {
                const chosen = e.target.value as BulkAction | "";
                e.target.value = "";
                if (chosen !== "") setBulkAction(chosen);
              }}
              className="h-11 min-h-[44px] rounded-lg border border-border-subtle bg-surface-card px-3 text-base md:text-xs text-text focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="">{t("admin.rowAction")}</option>
              {can(PERMISSIONS.userSuspend) && (
                <option value="suspend">{t("admin.bulkLock")}</option>
              )}
              {can(PERMISSIONS.userDelete) && (
                <option value="soft_delete">{t("admin.bulkSoftDelete")}</option>
              )}
            </select>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setSelected(new Set())}
            >
              {t("admin.clearSelection")}
            </Button>
          </div>
        </div>
      )}

      {bulkFailures !== null && bulkFailures.length > 0 && (
        <div className="flex items-start gap-2.5 rounded-lg border border-warning/30 bg-warning/10 p-3.5 text-xs text-warning-accent">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            {/* Named, not counted. A batch that half-worked and says only
                "3 failed" leaves the administrator to find out which three by
                running the batch again. */}
            <p className="font-medium">
              {t("admin.bulkPartialFailure", {
                done: selectedCountAtSubmit - bulkFailures.length,
                total: selectedCountAtSubmit,
              })}
            </p>
            <ul className="mt-1 list-disc pl-4">
              {bulkFailures.map((name) => (
                <li key={name}>{name}</li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3.5 text-xs text-danger-accent">
          <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {/* Table Container */}
      <div className="rounded-xl border border-border-subtle bg-surface-card/60 overflow-hidden">
        {isLoading ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
          </div>
        ) : users.length === 0 ? (
          <div className="flex min-h-[250px] flex-col items-center justify-center p-8 text-center space-y-2">
            <Users className="h-8 w-8 text-text-muted" />
            <p className="text-sm font-medium text-text-muted">
              {t("admin.noLearnersFound")}
            </p>
            <p className="text-xs text-text-muted">
              {t("admin.tryAdjustingYourSearchQueryOrFilters")}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr
                    key={headerGroup.id}
                    className="border-b border-border-subtle bg-surface-muted"
                  >
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        className="px-4 py-3.5 text-xs font-semibold uppercase tracking-wider text-text-muted"
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(
                              header.column.columnDef.header,
                              header.getContext(),
                            )}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {table.getRowModel().rows.map((row) => (
                  <tr
                    key={row.id}
                    className="hover:bg-surface-muted/40 transition-colors"
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="px-4 py-3 text-sm">
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Cursor Pagination Footer */}
        <div className="flex items-center justify-between border-t border-border-subtle px-4 py-3 bg-surface-muted text-xs text-text-muted">
          <div>
            {/* The match count, not the page size. This read "15 learner(s)"
                whatever the search matched, because it counted the rows it had
                been handed. */}
            {t("admin.totalLearners", { count: total })}
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handlePreviousPage}
              disabled={cursorHistory.length === 0 || isLoading}
            >
              <ChevronLeft className="mr-1 h-3.5 w-3.5" />
              {t("admin.previous")}
            </Button>

            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={!nextCursor || isLoading}
            >
              Next
              <ChevronRight className="ml-1 h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </div>

      {/* Batch confirmation. One justification covers the whole batch, which
          is the honest shape: the server records the same sentence against each
          account, so it has to be a sentence that is true of all of them. */}
      {bulkAction !== null && (
        <AdminActionReasonModal
          isOpen
          title={
            bulkAction === "suspend"
              ? t("admin.bulkLockTitle", { count: selected.size })
              : t("admin.bulkSoftDeleteTitle", { count: selected.size })
          }
          description={`${
            bulkAction === "suspend"
              ? t("admin.suspendingWillImmediatelyBlockLogin")
              : t("admin.softDeletingOpensGrace")
          } ${t("admin.bulkOneReason")}`}
          actionButtonLabel={
            bulkAction === "suspend"
              ? t("admin.bulkLock")
              : t("admin.bulkSoftDelete")
          }
          variant="destructive"
          onClose={() => setBulkAction(null)}
          onConfirm={async (reason) => {
            const targets = users.filter((u) => selected.has(u.id));
            setSelectedCountAtSubmit(targets.length);

            // Sequential, and every result kept. Promise.all would reject on the
            // first refusal and leave the administrator with no idea which of
            // the rest went through — and a batch that half-applied is exactly
            // the case that has to be reportable.
            const failed: string[] = [];
            for (const target of targets) {
              try {
                if (bulkAction === "suspend") {
                  await adminApi.suspendUser(target.id, reason);
                } else {
                  await adminApi.softDeleteUser(target.id, reason);
                }
              } catch {
                failed.push(target.display_name || target.email);
              }
            }

            setBulkFailures(failed);
            setBulkAction(null);
            await fetchUsers(currentCursor);
          }}
        />
      )}

      {/* Row action confirmation. Suspension and soft deletion both require a
          justification the server records, so this is where the row select
          lands rather than firing the request straight off the change event. */}
      {pendingAction !== null && (
        <AdminActionReasonModal
          isOpen
          title={
            pendingAction.action === "suspend"
              ? t("admin.suspendAccount")
              : pendingAction.action === "reinstate"
                ? t("admin.reinstateAccount")
                : t("admin.softDeleteUser")
          }
          description={
            pendingAction.action === "suspend"
              ? t("admin.suspendingWillImmediatelyBlockLogin")
              : pendingAction.action === "reinstate"
                ? t("admin.reinstatingRestoresFullAccountAccess")
                : t("admin.softDeletingOpensGrace")
          }
          actionButtonLabel={
            pendingAction.action === "suspend"
              ? t("admin.suspendUser")
              : pendingAction.action === "reinstate"
                ? t("admin.reinstateUser")
                : t("admin.softDeleteAccount")
          }
          variant={
            pendingAction.action === "reinstate" ? "primary" : "destructive"
          }
          onClose={() => setPendingAction(null)}
          onConfirm={async (reason) => {
            const { action, user } = pendingAction;
            if (action === "suspend") {
              await adminApi.suspendUser(user.id, reason);
            } else if (action === "reinstate") {
              await adminApi.reinstateUser(user.id, reason);
            } else {
              await adminApi.softDeleteUser(user.id, reason);
            }
            setPendingAction(null);
            // Refetch rather than patch the row in place: suspension ends the
            // learner's sessions and soft deletion changes a status this screen
            // filters on, so the list this administrator is looking at is no
            // longer the list the server would return.
            await fetchUsers(currentCursor);
          }}
        />
      )}

      {/* User Details Modal */}
      <AdminUserDetailModal
        userId={selectedUserId}
        onClose={() => setSelectedUserId(null)}
        onUserStatusChanged={() => {
          void fetchUsers(currentCursor);
        }}
      />
    </div>
  );
};
