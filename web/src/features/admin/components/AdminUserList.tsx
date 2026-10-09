import React, { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Eye,
  Lock,
  MoreHorizontal,
  Search,
  Trash2,
  Unlock,
  Users,
  AlertCircle,
  Filter,
  X,
} from "lucide-react";
import { adminApi, type AdminUserSummary } from "../api/adminApi";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { AdminActionReasonModal } from "./AdminActionReasonModal";
import { AdminUserDetailModal } from "./AdminUserDetailModal";
import { CopyableId } from "./CopyableId";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "./StatusBadge";
import { cn } from "@/lib/utils";
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { components } from "@/types/api";
import { NativeSelect } from "@/components/ui/native-select";

type UserStatus = components["schemas"]["UserStatus"];

/** How many accounts a page holds. The row number is derived from it. */
const PAGE_SIZE = 15;

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
  const { t, i18n } = useTranslation();
  const { can } = usePermissions();
  const [users, setUsers] = useState<AdminUserSummary[]>([]);
  // How many accounts match, across every page. `users.length` is the size of
  // one page, and the footer was showing it as though it were the whole set.
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Search & Filter state
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearch, setAppliedSearch] = useState("");
  const [selectedStatus, setSelectedStatus] = useState<UserStatus | "">("");
  const [mobileStatus, setMobileStatus] = useState<UserStatus | "">("");
  const [mobileFiltersOpen, setMobileFiltersOpen] = useState(false);

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

  // Selection is page-scoped and cleared on refetch, so a batch never silently
  // acts on accounts the administrator can no longer see.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkAction, setBulkAction] = useState<BulkAction | null>(null);
  const [bulkFailures, setBulkFailures] = useState<string[] | null>(null);
  // The size of the batch as it was submitted. Read after the fact by the
  // result banner, which cannot use `selected.size` because the selection is
  // cleared by the refetch that follows a batch.
  const [selectedCountAtSubmit, setSelectedCountAtSubmit] = useState(0);
  const requestId = useRef(0);

  const fetchUsers = useCallback(
    async (cursor?: string) => {
      const id = ++requestId.current;
      setIsLoading(true);
      setError(null);
      setSelected(new Set());

      try {
        // `email_prefix` and `display_name` are separate parameters and the
        // server ANDs them, so one search box has to choose. An "@" means the
        // administrator is typing an address; anything else is a name. Sending
        // both would match only accounts whose name and address both start with
        // the same text, which is nobody.
        const term = appliedSearch;
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

        if (id !== requestId.current) return;
        setUsers(res.items);
        setTotal(res.total);
        setCurrentCursor(cursor);
        setNextCursor(res.next_cursor);
      } catch (err: unknown) {
        if (id !== requestId.current) return;
        setError(
          err instanceof Error
            ? err.message
            : t("admin.failedToLoadLearnersList"),
        );
      } finally {
        if (id === requestId.current) setIsLoading(false);
      }
    },
    [appliedSearch, selectedStatus, t],
  );

  useEffect(() => {
    // Reset pagination when filters change
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setCursorHistory([]);
    void fetchUsers(undefined);
  }, [fetchUsers]);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCursorHistory([]);
    const term = searchQuery.trim();
    setAppliedSearch(term);
    if (term === appliedSearch) void fetchUsers(undefined);
  };

  const resetFilters = () => {
    setSearchQuery("");
    setAppliedSearch("");
    setSelectedStatus("");
    setMobileFiltersOpen(false);
  };

  const canSelect = (user: AdminUserSummary) =>
    actionsForStatus(user.status).some((action) =>
      action === "suspend"
        ? can(PERMISSIONS.userSuspend)
        : action === "soft_delete" && can(PERMISSIONS.userDelete),
    );
  const selectableUsers = users.filter(canSelect);
  const canBulkAction =
    can(PERMISSIONS.userSuspend) || can(PERMISSIONS.userDelete);
  const allSelected =
    selectableUsers.length > 0 &&
    selectableUsers.every((user) => selected.has(user.id));
  const renderSelectAll = () => (
    <Checkbox
      checked={allSelected}
      aria-label={t("admin.selectPage")}
      className={
        selected.size > 0 && !allSelected
          ? "border-primary bg-primary/10 after:h-0.5 after:w-2.5 after:bg-primary"
          : undefined
      }
      disabled={isLoading || Boolean(error) || selectableUsers.length === 0}
      ref={(input) => {
        if (input) input.indeterminate = selected.size > 0 && !allSelected;
      }}
      onCheckedChange={(checked) =>
        setSelected(
          new Set(checked ? selectableUsers.map((user) => user.id) : []),
        )
      }
    />
  );
  const formatJoined = (date: string) =>
    new Date(date).toLocaleDateString(i18n.resolvedLanguage, {
      day: "2-digit",
      month: "short",
      year: "numeric",
    });

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

  const actionLabel = (action: RowAction): string =>
    action === "inspect"
      ? t("admin.inspectAccount")
      : action === "suspend"
        ? t("admin.lockLogin")
        : action === "reinstate"
          ? t("admin.unlockLogin")
          : t("admin.softDeleteAccount");

  const renderSelect = (user: AdminUserSummary) => {
    // Only accounts a batch can actually act on are selectable. A ticked
    // row that every bulk action would skip is a row that makes the count
    // in the toolbar a lie.
    if (!canSelect(user)) return null;
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
  };

  const renderAvatar = (user: AdminUserSummary) => (
    <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-primary/10 text-sm font-semibold text-primary-accent ring-1 ring-primary/10">
      {user.avatar_url ? (
        <img
          src={user.avatar_url}
          alt=""
          className="h-full w-full object-cover"
        />
      ) : (
        <span aria-hidden="true">
          {user.display_name
            .trim()
            .split(/\s+/)
            .slice(0, 2)
            .map((part) => part.charAt(0))
            .join("")
            .toUpperCase() || "?"}
        </span>
      )}
    </div>
  );

  const renderStatus = (status: AdminUserSummary["status"]) => (
    <StatusBadge
      tone={
        status === "active"
          ? "success"
          : status === "suspended"
            ? "danger"
            : status === "deleted"
              ? "neutral"
              : "warning"
      }
    >
      <span
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
        aria-hidden="true"
      />
      {t(`admin.userStatus.${status}`, status)}
    </StatusBadge>
  );

  // One menu per row. The items keep the accessible names the icon buttons
  // had, and none of them calls the API: every write lands on the reason
  // modal first.
  const renderActions = (user: AdminUserSummary) => {
    const allowed = actionsForStatus(user.status).filter((action) => {
      if (action === "suspend") return can(PERMISSIONS.userSuspend);
      if (action === "reinstate") return can(PERMISSIONS.userReinstate);
      if (action === "soft_delete") return can(PERMISSIONS.userDelete);
      return can(PERMISSIONS.userRead);
    });
    if (allowed.length === 0) return null;
    return (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={t("admin.actionsFor", { name: user.display_name })}
          className="flex h-11 w-11 min-h-[44px] min-w-[44px] cursor-pointer items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        >
          <MoreHorizontal className="h-5 w-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {allowed.map((action) => {
            const Icon =
              action === "inspect"
                ? Eye
                : action === "suspend"
                  ? Lock
                  : action === "reinstate"
                    ? Unlock
                    : Trash2;
            return (
              <DropdownMenuItem
                key={action}
                destructive={action === "soft_delete"}
                onSelect={() => {
                  if (action === "inspect") {
                    setSelectedUserId(user.id);
                    return;
                  }
                  setPendingAction({ action, user });
                }}
              >
                <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                {actionLabel(action)}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  const rowOffset = cursorHistory.length * PAGE_SIZE;

  return (
    <div className="space-y-4">
      <div className="overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3 px-4 pt-5 sm:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
              <Users className="h-5 w-5" aria-hidden="true" />
            </span>
            <h2 className="text-lg font-semibold leading-snug text-text">
              {t("admin.accountDirectory")}
            </h2>
            {!isLoading && !error && (
              <span
                data-testid="learner-total"
                className="rounded-full border border-border-subtle bg-surface-muted px-2.5 py-0.5 text-xs font-semibold tabular-nums text-text-muted"
              >
                {total.toLocaleString(i18n.resolvedLanguage)}
              </span>
            )}
          </div>
          {canBulkAction && (
            <p className="hidden text-sm text-text-muted lg:block">
              {t("admin.selectAccountsHint")}
            </p>
          )}
        </div>
        <div className="space-y-3 p-4 sm:px-6 sm:pb-5">
          <form
            onSubmit={handleSearchSubmit}
            className="flex flex-wrap items-center gap-3"
          >
            <div className="relative w-full sm:w-auto sm:min-w-0 sm:flex-1">
              <Search
                className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
                aria-hidden="true"
              />
              <Input
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={t("admin.searchByNameOrEmail")}
                aria-label={t("admin.searchByNameOrEmail")}
                className="bg-surface-muted pl-10"
                type="search"
              />
            </div>

            <NativeSelect
              value={selectedStatus}
              aria-label={t("admin.filterByStatus")}
              onChange={(e) =>
                setSelectedStatus(e.target.value as UserStatus | "")
              }
              wrapperClassName="hidden min-w-44 md:block md:w-auto"
            >
              <option value="">{t("admin.allStatuses")}</option>
              <option value="active">{t("admin.active")}</option>
              <option value="suspended">{t("admin.suspended")}</option>
              <option value="pending_deletion">
                {t("admin.pendingDeletion")}
              </option>
              <option value="deleted">{t("admin.deleted")}</option>
            </NativeSelect>

            <Button
              type="button"
              variant="outline"
              className="flex-1 whitespace-nowrap px-3 md:hidden"
              onClick={() => {
                setMobileStatus(selectedStatus);
                setMobileFiltersOpen(true);
              }}
            >
              <Filter className="h-4 w-4" aria-hidden="true" />
              {t("admin.filters", "Filters")}
              {selectedStatus && (
                <span
                  className="h-1.5 w-1.5 rounded-full bg-primary"
                  aria-hidden="true"
                />
              )}
            </Button>

            <Button
              type="submit"
              size="md"
              className="flex-1 whitespace-nowrap px-3 sm:flex-none sm:px-4"
            >
              <Search className="h-4 w-4 shrink-0" aria-hidden="true" />
              {t("admin.search")}
            </Button>
          </form>
          {(appliedSearch || selectedStatus) && (
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className="text-text-muted">
                {t("admin.activeFilters")}
              </span>
              {appliedSearch && (
                <span className="max-w-full break-all rounded-lg bg-surface-muted px-2.5 py-1.5 text-text">
                  {appliedSearch}
                </span>
              )}
              {selectedStatus && renderStatus(selectedStatus)}
              <Button
                variant="ghost"
                size="sm"
                type="button"
                onClick={resetFilters}
              >
                <X className="h-4 w-4" aria-hidden="true" />
                {t("admin.resetFilters")}
              </Button>
            </div>
          )}
        </div>

        <Sheet open={mobileFiltersOpen} onOpenChange={setMobileFiltersOpen}>
          <SheetContent>
            <SheetHeader>
              <SheetTitle className="text-lg font-semibold leading-tight text-text">
                {t("admin.filters", "Filters")}
              </SheetTitle>
              <SheetDescription className="mt-1 text-sm text-text-muted">
                {t("admin.filterAccountsHint")}
              </SheetDescription>
            </SheetHeader>
            <SheetBody className="space-y-5">
              <label className="block space-y-1.5 text-sm font-medium text-text">
                {t("admin.filterByStatus")}
                <NativeSelect
                  value={mobileStatus}
                  onChange={(event) =>
                    setMobileStatus(event.target.value as UserStatus | "")
                  }
                >
                  <option value="">{t("admin.allStatuses")}</option>
                  <option value="active">{t("admin.active")}</option>
                  <option value="suspended">{t("admin.suspended")}</option>
                  <option value="pending_deletion">
                    {t("admin.pendingDeletion")}
                  </option>
                  <option value="deleted">{t("admin.deleted")}</option>
                </NativeSelect>
              </label>
              <div className="flex justify-between gap-2">
                <Button type="button" variant="outline" onClick={resetFilters}>
                  {t("admin.resetFilters")}
                </Button>
                <Button
                  type="button"
                  onClick={() => {
                    setSelectedStatus(mobileStatus);
                    setMobileFiltersOpen(false);
                  }}
                >
                  {t("common.apply", "Apply")}
                </Button>
              </div>
            </SheetBody>
          </SheetContent>
        </Sheet>

        {selected.size > 0 && (
          <div className="mx-4 mb-4 flex flex-col gap-3 rounded-xl border border-primary/20 bg-primary/5 p-3 sm:mx-6 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-sm font-medium text-text">
              {t("admin.bulkAction", { count: selected.size })}
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <NativeSelect
                value=""
                aria-label={t("admin.bulkAction", { count: selected.size })}
                onChange={(e) => {
                  const chosen = e.target.value as BulkAction | "";
                  e.target.value = "";
                  if (chosen !== "") setBulkAction(chosen);
                }}
                wrapperClassName="w-auto max-w-full"
              >
                <option value="">{t("admin.rowAction")}</option>
                {can(PERMISSIONS.userSuspend) && (
                  <option
                    value="suspend"
                    disabled={users.some(
                      (user) =>
                        selected.has(user.id) && user.status !== "active",
                    )}
                  >
                    {t("admin.bulkLock")}
                  </option>
                )}
                {can(PERMISSIONS.userDelete) && (
                  <option value="soft_delete">
                    {t("admin.bulkSoftDelete")}
                  </option>
                )}
              </NativeSelect>
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
          <div className="mx-4 mb-4 flex items-start gap-2.5 rounded-lg border border-warning/30 bg-warning/10 p-3.5 text-sm text-warning-accent sm:mx-6">
            <AlertCircle aria-hidden="true" className="h-5 w-5 shrink-0" />
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

        <div className="border-t border-border-subtle">
          {isLoading ? (
            <div
              role="status"
              aria-busy="true"
              aria-label={t("admin.loadingLearners")}
              data-testid="learner-skeleton"
              className="divide-y divide-border-subtle"
            >
              {Array.from({ length: 5 }, (_, i) => (
                <div key={i} className="flex h-20 items-center gap-3 px-6">
                  <Skeleton className="h-9 w-9 shrink-0 rounded-full" />
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-3.5 w-1/3" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                  <Skeleton className="hidden h-5 w-16 rounded-full md:block" />
                </div>
              ))}
            </div>
          ) : error ? (
            <ErrorState
              className="py-10"
              title={t("admin.failedToLoadLearnersList")}
              description={error}
              onRetry={() => void fetchUsers(currentCursor)}
            />
          ) : users.length === 0 ? (
            <EmptyState
              className="py-10"
              icon={Users}
              title={t("admin.noLearnersFound")}
              description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
            />
          ) : (
            <>
              {/* Desktop: a compact table. jsdom has no viewport, so both
                layouts are rendered and CSS picks one. */}
              <div
                data-testid="learner-table"
                className="hidden overflow-x-auto md:block"
              >
                {/* min-w keeps names readable at 768px beside the expanded
                  sidebar; below it the table scrolls instead of squeezing. */}
                <table className="w-full min-w-[820px] table-fixed border-collapse text-left">
                  <caption className="sr-only">
                    {t("admin.accountDirectory")}
                  </caption>
                  <thead>
                    <tr className="border-b border-border-subtle bg-surface-muted/60">
                      <th scope="col" className="w-16 px-2 py-1">
                        {canBulkAction ? (
                          renderSelectAll()
                        ) : (
                          <span className="sr-only">
                            {t("admin.selectRow")}
                          </span>
                        )}
                      </th>
                      <th
                        scope="col"
                        className="px-3 py-3 text-xs font-semibold text-text-muted"
                      >
                        {t("admin.learner")}
                      </th>
                      <th
                        scope="col"
                        className="px-3 py-3 text-xs font-semibold text-text-muted"
                      >
                        {t("admin.email")}
                      </th>
                      <th
                        scope="col"
                        className="w-36 px-3 py-3 text-xs font-semibold text-text-muted"
                      >
                        {t("admin.status")}
                      </th>
                      <th
                        scope="col"
                        className="w-36 px-3 py-3 text-xs font-semibold text-text-muted"
                      >
                        {t("admin.joined")}
                      </th>
                      <th scope="col" className="w-16 px-2 py-3">
                        <span className="sr-only">{t("common.actions")}</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-subtle">
                    {users.map((user, index) => (
                      <tr
                        key={user.id}
                        data-row-number={rowOffset + index + 1}
                        className={cn(
                          "h-20 transition-colors hover:bg-surface-muted/40",
                          selected.has(user.id) && "bg-primary/5",
                        )}
                      >
                        <td className="px-2">{renderSelect(user)}</td>
                        <td className="px-3">
                          <div className="flex min-w-0 items-center gap-3">
                            {renderAvatar(user)}
                            <div className="min-w-0">
                              <p
                                title={user.display_name}
                                className="truncate text-sm font-semibold text-text"
                              >
                                {user.display_name}
                              </p>
                              <CopyableId id={user.id} />
                            </div>
                          </div>
                        </td>
                        <td className="px-3">
                          <span
                            title={user.email}
                            className="block truncate text-sm text-text-muted"
                          >
                            {user.email}
                          </span>
                        </td>
                        <td className="px-3">{renderStatus(user.status)}</td>
                        <td className="px-3 text-sm tabular-nums text-text-muted">
                          <time dateTime={user.created_at}>
                            {formatJoined(user.created_at)}
                          </time>
                        </td>
                        <td className="px-2 text-right">
                          {renderActions(user)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Mobile: one card per learner, no horizontal scrolling. */}
              {canBulkAction && (
                <div className="flex items-center gap-2 border-b border-border-subtle bg-surface-muted/60 px-3 py-1 md:hidden">
                  {renderSelectAll()}
                  <span className="text-sm font-medium text-text-muted">
                    {t("admin.selectPage")}
                  </span>
                </div>
              )}
              <ul
                data-testid="learner-cards"
                className="divide-y divide-border-subtle md:hidden"
              >
                {users.map((user) => (
                  <li
                    key={user.id}
                    className={cn(
                      "space-y-2 p-4",
                      selected.has(user.id) && "bg-primary/5",
                    )}
                  >
                    <div className="flex items-center gap-3">
                      {renderAvatar(user)}
                      <div className="min-w-0 flex-1">
                        <p
                          title={user.display_name}
                          className="truncate text-sm font-semibold text-text"
                        >
                          {user.display_name}
                        </p>
                      </div>
                      <div className="shrink-0">{renderActions(user)}</div>
                    </div>
                    <p className="break-all text-sm text-text-muted">
                      {user.email}
                    </p>
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                        {renderStatus(user.status)}
                        <span className="text-sm text-text-muted">
                          <time dateTime={user.created_at}>
                            {formatJoined(user.created_at)}
                          </time>
                        </span>
                      </div>
                      <div className="shrink-0">{renderSelect(user)}</div>
                    </div>
                    <CopyableId id={user.id} />
                  </li>
                ))}
              </ul>
            </>
          )}

          {/* No `onPageSelect`: /admin/users is cursor-paged, so page 6 cannot be
            asked for directly. The position readout is still worth having — the
            complaint was never that the numbers were unclickable, it was that
            there were none. */}
          {!error && (
            <Pagination
              page={cursorHistory.length + 1}
              pageCount={Math.max(1, Math.ceil(total / PAGE_SIZE))}
              total={total}
              rangeFrom={users.length > 0 ? rowOffset + 1 : 0}
              rangeTo={rowOffset + users.length}
              isBusy={isLoading}
              canPrevious={cursorHistory.length > 0}
              canNext={Boolean(nextCursor)}
              onPrevious={handlePreviousPage}
              onNext={handleNextPage}
              className="bg-surface-card px-4 sm:px-6"
            />
          )}
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
          justification the server records, so this is where the row menu
          lands rather than firing the request straight off the selection. */}
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
