import React, { useEffect, useState, useMemo } from "react";
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
  Search,
  Users,
  AlertCircle,
} from "lucide-react";
import { adminApi, type AdminUserSummary } from "../api/adminApi";
import { AdminUserDetailModal } from "./AdminUserDetailModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { components } from "@/types/api";

type UserStatus = components["schemas"]["UserStatus"];

const columnHelper = createColumnHelper<AdminUserSummary>();

export const AdminUserList: React.FC = () => {
  const [users, setUsers] = useState<AdminUserSummary[]>([]);
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
        limit: 15,
      });

      setUsers(res.items);
      setCurrentCursor(cursor);
      setNextCursor(res.next_cursor);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to load learners list.",
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
      columnHelper.accessor("display_name", {
        header: "Learner",
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
        header: "Status",
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
        header: "Joined",
        cell: (info) => (
          <span className="text-xs text-text-muted">
            {new Date(info.getValue()).toLocaleDateString()}
          </span>
        ),
      }),
      columnHelper.display({
        id: "actions",
        header: "Actions",
        cell: (info) => (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setSelectedUserId(info.row.original.id)}
            className="text-text-muted hover:text-primary-accent hover:bg-primary/10"
          >
            <Eye className="mr-1.5 h-3.5 w-3.5" />
            Inspect
          </Button>
        ),
      }),
    ],
    [],
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
              placeholder="Search by name or email..."
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
            <option value="">All Statuses</option>
            <option value="active">Active</option>
            <option value="suspended">Suspended</option>
            <option value="pending_deletion">Pending Deletion</option>
            <option value="deleted">Deleted</option>
          </select>

          <Button type="submit" size="md">
            <Search className="mr-1.5 h-4 w-4" />
            Search
          </Button>
        </form>
      </div>

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
              No learners found
            </p>
            <p className="text-xs text-text-muted">
              Try adjusting your search query or filters.
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
            Showing <strong className="text-text">{users.length}</strong>{" "}
            learner(s)
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
              Previous
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
