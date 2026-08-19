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
import {
  adminApi,
  type AdminUserSummary,
} from "../api/adminApi";
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
  const [currentCursor, setCurrentCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [cursorHistory, setCursorHistory] = useState<(string | undefined)[]>([]);

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
              <div className="h-9 w-9 overflow-hidden rounded-full border border-indigo-500/30 bg-slate-800 flex items-center justify-center text-xs font-bold text-indigo-300 shrink-0">
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
                <p className="text-sm font-medium text-slate-100 truncate">
                  {user.display_name}
                </p>
                <p className="text-xs text-slate-400 font-mono truncate">
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
          <span className="text-xs font-mono text-slate-300">
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
                  ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                  : status === "suspended"
                  ? "bg-rose-500/10 text-rose-400 border-rose-500/20"
                  : "bg-amber-500/10 text-amber-400 border-amber-500/20"
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
          <span className="text-xs text-slate-400">
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
            className="text-slate-400 hover:text-indigo-300 hover:bg-indigo-500/10"
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
      <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-4 space-y-4">
        <form onSubmit={handleSearchSubmit} className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-slate-400" />
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by name or email..."
              className="pl-10"
            />
          </div>

          <select
            value={selectedStatus}
            onChange={(e) => setSelectedStatus(e.target.value as UserStatus | "")}
            className="h-11 min-h-[44px] rounded-lg border border-slate-800 bg-slate-900 px-3 text-base md:text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
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
        <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3.5 text-xs text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {/* Table Container */}
      <div className="rounded-xl border border-slate-800 bg-slate-900/60 overflow-hidden">
        {isLoading ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
          </div>
        ) : users.length === 0 ? (
          <div className="flex min-h-[250px] flex-col items-center justify-center p-8 text-center space-y-2">
            <Users className="h-8 w-8 text-slate-500" />
            <p className="text-sm font-medium text-slate-300">No learners found</p>
            <p className="text-xs text-slate-500">
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
                    className="border-b border-slate-800 bg-slate-950/60"
                  >
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        className="px-4 py-3.5 text-xs font-semibold uppercase tracking-wider text-slate-400"
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
              <tbody className="divide-y divide-slate-800/60">
                {table.getRowModel().rows.map((row) => (
                  <tr
                    key={row.id}
                    className="hover:bg-slate-800/40 transition-colors"
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
        <div className="flex items-center justify-between border-t border-slate-800 px-4 py-3 bg-slate-950/40 text-xs text-slate-400">
          <div>
            Showing <strong className="text-slate-200">{users.length}</strong> learner(s)
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
