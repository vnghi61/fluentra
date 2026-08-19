import React, { useEffect, useState, useMemo } from "react";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  AlertCircle,
  Clock,
  Flag,
  Loader2,
  Plus,
  Trash2,
  User,
} from "lucide-react";
import {
  adminApi,
  type FeatureFlag,
} from "../api/adminApi";
import { CreateFeatureFlagModal } from "./CreateFeatureFlagModal";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";

const columnHelper = createColumnHelper<FeatureFlag>();

export const AdminFeatureFlags: React.FC = () => {
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [updatingKey, setUpdatingKey] = useState<string | null>(null);
  const [deletingKey, setDeletingKey] = useState<string | null>(null);

  const fetchFlags = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await adminApi.listFlags();
      setFlags(res.items);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to load feature flags.",
      );
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void fetchFlags();
  }, []);

  const handleToggleEnabled = async (flag: FeatureFlag, newEnabled: boolean) => {
    setUpdatingKey(flag.key);
    try {
      const updated = await adminApi.updateFlag(flag.key, {
        enabled: newEnabled,
        expires_on: flag.expires_on,
        rollout_percent: flag.rollout_percent,
        description: flag.description,
      });
      setFlags((prev) => prev.map((f) => (f.key === flag.key ? updated : f)));
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to update feature flag.",
      );
    } finally {
      setUpdatingKey(null);
    }
  };

  const handleUpdateRollout = async (flag: FeatureFlag, percent: number) => {
    setUpdatingKey(flag.key);
    try {
      const updated = await adminApi.updateFlag(flag.key, {
        rollout_percent: percent,
        expires_on: flag.expires_on,
        enabled: flag.enabled,
        description: flag.description,
      });
      setFlags((prev) => prev.map((f) => (f.key === flag.key ? updated : f)));
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to update rollout percentage.",
      );
    } finally {
      setUpdatingKey(null);
    }
  };

  const handleDelete = async (key: string) => {
    if (!confirm(`Are you sure you want to permanently delete flag "${key}"?`)) {
      return;
    }

    setDeletingKey(key);
    try {
      await adminApi.deleteFlag(key);
      setFlags((prev) => prev.filter((f) => f.key !== key));
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to delete feature flag.",
      );
    } finally {
      setDeletingKey(null);
    }
  };

  const columns = useMemo(
    () => [
      columnHelper.accessor("key", {
        header: "Flag Key & Description",
        cell: (info) => {
          const flag = info.row.original;
          return (
            <div className="space-y-0.5">
              <p className="font-mono text-sm font-semibold text-slate-100">
                {flag.key}
              </p>
              <p className="text-xs text-slate-400">{flag.description}</p>
            </div>
          );
        },
      }),
      columnHelper.accessor("enabled", {
        header: "State",
        cell: (info) => {
          const flag = info.row.original;
          const isUpdating = updatingKey === flag.key;
          return (
            <div className="flex items-center gap-2">
              <Checkbox
                checked={flag.enabled}
                disabled={isUpdating}
                onCheckedChange={(checked) => {
                  void handleToggleEnabled(flag, !!checked);
                }}
              />
              <span
                className={`text-xs font-medium ${
                  flag.enabled ? "text-emerald-400" : "text-slate-400"
                }`}
              >
                {flag.enabled ? "Enabled" : "Disabled"}
              </span>
            </div>
          );
        },
      }),
      columnHelper.accessor("rollout_percent", {
        header: "Rollout",
        cell: (info) => {
          const flag = info.row.original;
          return (
            <div className="flex items-center gap-2 min-w-[120px]">
              <input
                type="range"
                min={0}
                max={100}
                step={5}
                value={flag.rollout_percent}
                disabled={updatingKey === flag.key || !flag.enabled}
                onChange={(e) => {
                  void handleUpdateRollout(flag, parseInt(e.target.value, 10));
                }}
                className="w-20 accent-indigo-500 cursor-pointer disabled:opacity-40"
              />
              <span className="text-xs font-mono text-slate-200">
                {flag.rollout_percent}%
              </span>
            </div>
          );
        },
      }),
      columnHelper.accessor("owner", {
        header: "Owner",
        cell: (info) => (
          <span className="text-xs text-slate-300 flex items-center gap-1 font-mono">
            <User className="h-3.5 w-3.5 text-slate-400" />
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor("expires_on", {
        header: "Expires On",
        cell: (info) => {
          const expiresDate = new Date(info.getValue());
          const isPast = expiresDate.getTime() < Date.now();
          return (
            <span
              className={`text-xs flex items-center gap-1 ${
                isPast ? "text-rose-400 font-semibold" : "text-slate-400"
              }`}
            >
              <Clock className="h-3.5 w-3.5" />
              {info.getValue()}
              {isPast && " (Expired)"}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: "actions",
        header: "Actions",
        cell: (info) => {
          const flag = info.row.original;
          return (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                void handleDelete(flag.key);
              }}
              disabled={deletingKey === flag.key}
              className="text-slate-400 hover:text-rose-400 hover:bg-rose-500/10"
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span className="sr-only">Delete flag</span>
            </Button>
          );
        },
      }),
    ],
    [updatingKey, deletingKey],
  );

  const table = useReactTable({
    data: flags,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div className="space-y-1">
          <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
            <Flag className="h-5 w-5 text-indigo-400" />
            Feature Flags & Rollouts
          </h3>
          <p className="text-xs text-slate-400">
            Control dynamic system feature switches, percentage rollouts, and owner assignments.
          </p>
        </div>

        <Button
          type="button"
          onClick={() => setIsCreateModalOpen(true)}
          size="sm"
        >
          <Plus className="mr-1.5 h-4 w-4" />
          Create Flag
        </Button>
      </div>

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3.5 text-xs text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {/* Table */}
      <div className="rounded-xl border border-slate-800 bg-slate-900/60 overflow-hidden">
        {isLoading ? (
          <div className="flex min-h-[250px] items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
          </div>
        ) : flags.length === 0 ? (
          <div className="flex min-h-[200px] flex-col items-center justify-center p-8 text-center space-y-2">
            <Flag className="h-8 w-8 text-slate-500" />
            <p className="text-sm font-medium text-slate-300">No feature flags configured</p>
            <p className="text-xs text-slate-500">
              Create a feature flag to control rollouts.
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
                      <td key={cell.id} className="px-4 py-3.5 text-sm">
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
      </div>

      <CreateFeatureFlagModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onSuccess={(newFlag) => {
          setFlags((prev) => [...prev, newFlag]);
        }}
      />
    </div>
  );
};
