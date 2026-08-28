import React, { useCallback, useEffect, useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
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
import { adminApi, type FeatureFlag } from "../api/adminApi";
import { CreateFeatureFlagModal } from "./CreateFeatureFlagModal";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";

const columnHelper = createColumnHelper<FeatureFlag>();

export const AdminFeatureFlags: React.FC = () => {
  const { t } = useTranslation();
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [updatingKey, setUpdatingKey] = useState<string | null>(null);
  const [deletingKey, setDeletingKey] = useState<string | null>(null);

  const fetchFlags = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await adminApi.listFlags();
      setFlags(res.items);
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : t(
              "admin.failedToLoadFeatureFlags",
              "Failed to load feature flags.",
            ),
      );
    } finally {
      setIsLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void fetchFlags();
  }, [fetchFlags]);

  const handleToggleEnabled = useCallback(
    async (flag: FeatureFlag, newEnabled: boolean) => {
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
          err instanceof Error
            ? err.message
            : t(
                "admin.failedToUpdateFeatureFlag",
                "Failed to update feature flag.",
              ),
        );
      } finally {
        setUpdatingKey(null);
      }
    },
    [t],
  );

  const handleUpdateRollout = useCallback(
    async (flag: FeatureFlag, percent: number) => {
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
          err instanceof Error
            ? err.message
            : t(
                "admin.failedToUpdateRolloutPercentage",
                "Failed to update rollout percentage.",
              ),
        );
      } finally {
        setUpdatingKey(null);
      }
    },
    [t],
  );

  const handleDelete = useCallback(
    async (key: string) => {
      if (
        !confirm(`Are you sure you want to permanently delete flag "${key}"?`)
      ) {
        return;
      }

      setDeletingKey(key);
      try {
        await adminApi.deleteFlag(key);
        setFlags((prev) => prev.filter((f) => f.key !== key));
      } catch (err: unknown) {
        setError(
          err instanceof Error
            ? err.message
            : t(
                "admin.failedToDeleteFeatureFlag",
                "Failed to delete feature flag.",
              ),
        );
      } finally {
        setDeletingKey(null);
      }
    },
    [t],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor("key", {
        header: t("admin.flagKeyDescription", "Flag Key & Description"),
        cell: (info) => {
          const flag = info.row.original;
          return (
            <div className="space-y-0.5">
              <p className="font-mono text-sm font-semibold text-text">
                {flag.key}
              </p>
              <p className="text-xs text-text-muted">{flag.description}</p>
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
                  flag.enabled ? "text-success-accent" : "text-text-muted"
                }`}
              >
                {flag.enabled
                  ? t("admin.enabled", "Enabled")
                  : t("admin.disabled", "Disabled")}
              </span>
            </div>
          );
        },
      }),
      columnHelper.accessor("rollout_percent", {
        header: t("admin.rollout", "Rollout"),
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
                className="w-20 accent-primary cursor-pointer disabled:opacity-40"
              />
              <span className="text-xs font-mono text-text">
                {flag.rollout_percent}%
              </span>
            </div>
          );
        },
      }),
      columnHelper.accessor("owner", {
        header: "Owner",
        cell: (info) => (
          <span className="text-xs text-text-muted flex items-center gap-1 font-mono">
            <User className="h-3.5 w-3.5 text-text-muted" />
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor("expires_on", {
        header: t("admin.expiresOn", "Expires On"),
        cell: (info) => {
          const expiresDate = new Date(info.getValue());
          const isPast = expiresDate.getTime() < Date.now();
          return (
            <span
              className={`text-xs flex items-center gap-1 ${
                isPast ? "text-danger-accent font-semibold" : "text-text-muted"
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
        header: t("admin.actions", "Actions"),
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
              className="text-text-muted hover:text-danger-accent hover:bg-danger/10"
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span className="sr-only">
                {t("admin.deleteFlag", "Delete flag")}
              </span>
            </Button>
          );
        },
      }),
    ],
    [
      updatingKey,
      deletingKey,
      t,
      handleToggleEnabled,
      handleUpdateRollout,
      handleDelete,
    ],
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
          <h3 className="text-base font-semibold text-text flex items-center gap-2">
            <Flag className="h-5 w-5 text-primary-accent" />
            {t("admin.featureFlagsRollouts", "Feature Flags & Rollouts")}
          </h3>
          <p className="text-xs text-text-muted">
            Control dynamic system feature switches, percentage rollouts, and
            owner assignments.
          </p>
        </div>

        <Button
          type="button"
          onClick={() => setIsCreateModalOpen(true)}
          size="sm"
        >
          <Plus className="mr-1.5 h-4 w-4" />
          {t("admin.createFlag", "Create Flag")}
        </Button>
      </div>

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3.5 text-xs text-danger-accent">
          <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {/* Table */}
      <div className="rounded-xl border border-border-subtle bg-surface-card/60 overflow-hidden">
        {isLoading ? (
          <div className="flex min-h-[250px] items-center justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
          </div>
        ) : flags.length === 0 ? (
          <div className="flex min-h-[200px] flex-col items-center justify-center p-8 text-center space-y-2">
            <Flag className="h-8 w-8 text-text-muted" />
            <p className="text-sm font-medium text-text-muted">
              {t(
                "admin.noFeatureFlagsConfigured",
                "No feature flags configured",
              )}
            </p>
            <p className="text-xs text-text-muted">
              {t(
                "admin.createAFeatureFlagToControlRollouts",
                "Create a feature flag to control rollouts.",
              )}
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
