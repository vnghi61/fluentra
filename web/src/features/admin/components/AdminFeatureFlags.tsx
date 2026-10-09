import React, { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Clock,
  Flag,
  Percent,
  Plus,
  Power,
  Search,
  Trash2,
  User,
} from "lucide-react";
import { adminApi, type FeatureFlag } from "../api/adminApi";
import { CreateFeatureFlagModal } from "./CreateFeatureFlagModal";
import { StatusBadge } from "./StatusBadge";
import {
  AdminActiveFilters,
  AdminDataPanel,
  AdminFilterCard,
  AdminFilterChip,
  AdminListSkeleton,
  AdminNotice,
  AdminRefreshButton,
  AdminRowIcon,
  AdminSectionHeader,
  AdminStatGrid,
  adminStatTones,
  adminTable,
  type AdminStat,
} from "./AdminSurface";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/utils";
import { NativeSelect } from "@/components/ui/native-select";

export const AdminFeatureFlags: React.FC = () => {
  const { t, i18n } = useTranslation();
  const [search, setSearch] = useState("");
  const [stateFilter, setStateFilter] = useState<"" | "enabled" | "disabled">(
    "",
  );
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [updatingKey, setUpdatingKey] = useState<string | null>(null);
  const [deletingKey, setDeletingKey] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  // Expiry is judged against when the page was opened, not re-read per render.
  const [now] = useState(() => Date.now());

  // Bumped by the refresh button; the click handler sets the loading flag so
  // the effect only sets state from the request's callbacks.
  const [reloadKey, setReloadKey] = useState(0);
  const reloadFlags = () => {
    setIsLoading(true);
    setReloadKey((key) => key + 1);
  };

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listFlags()
      .then((res) => {
        if (cancelled) return;
        setFlags(res.items);
        setError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(
          err instanceof Error
            ? err.message
            : t("admin.failedToLoadFeatureFlags"),
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [t, reloadKey]);

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
            : t("admin.failedToUpdateFeatureFlag"),
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
            : t("admin.failedToUpdateRolloutPercentage"),
        );
      } finally {
        setUpdatingKey(null);
      }
    },
    [t],
  );

  const handleDelete = useCallback(
    async (key: string) => {
      setDeleteTarget(null);
      setDeletingKey(key);
      try {
        await adminApi.deleteFlag(key);
        setFlags((prev) => prev.filter((f) => f.key !== key));
      } catch (err: unknown) {
        setError(
          err instanceof Error
            ? err.message
            : t("admin.failedToDeleteFeatureFlag"),
        );
      } finally {
        setDeletingKey(null);
      }
    },
    [t],
  );

  const term = search.trim().toLocaleLowerCase();
  const visibleFlags = flags.filter(
    (flag) =>
      (!stateFilter ||
        (stateFilter === "enabled" ? flag.enabled : !flag.enabled)) &&
      (!term || flag.key.toLocaleLowerCase().includes(term)),
  );
  const filtered = Boolean(term || stateFilter);
  const isExpired = (flag: FeatureFlag) =>
    new Date(flag.expires_on).getTime() < now;
  const resetFilters = () => {
    setSearch("");
    setStateFilter("");
  };
  const stats: AdminStat[] = [
    {
      key: "total",
      label: t("admin.flagsTotal"),
      value: flags.length.toLocaleString(i18n.resolvedLanguage),
      Icon: Flag,
      tone: adminStatTones.primary,
    },
    {
      key: "enabled",
      label: t("admin.enabled"),
      value: flags
        .filter((flag) => flag.enabled)
        .length.toLocaleString(i18n.resolvedLanguage),
      Icon: Power,
      tone: adminStatTones.success,
    },
    {
      key: "partial",
      label: t("admin.flagsPartialRollout"),
      value: flags
        .filter((flag) => flag.enabled && flag.rollout_percent < 100)
        .length.toLocaleString(i18n.resolvedLanguage),
      Icon: Percent,
      tone: adminStatTones.warning,
    },
    {
      key: "expired",
      label: t("admin.flagsExpired"),
      value: flags
        .filter(isExpired)
        .length.toLocaleString(i18n.resolvedLanguage),
      Icon: Clock,
      tone: adminStatTones.danger,
    },
  ];
  const stateSwitch = (flag: FeatureFlag) => (
    <div className="flex items-center gap-2">
      <Switch
        aria-label={`${t("admin.state")}: ${flag.key}`}
        checked={flag.enabled}
        disabled={updatingKey === flag.key}
        onCheckedChange={(checked) => void handleToggleEnabled(flag, checked)}
      />
      <StatusBadge tone={flag.enabled ? "success" : "neutral"}>
        <span
          aria-hidden="true"
          className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
        />
        {flag.enabled ? t("admin.enabled") : t("admin.disabled")}
      </StatusBadge>
    </div>
  );
  const rollout = (flag: FeatureFlag, labelled = false) => (
    <div className="flex items-center gap-2">
      <input
        type="range"
        min={0}
        max={100}
        step={5}
        value={flag.rollout_percent}
        aria-label={`${t("admin.rollout")}: ${flag.key}`}
        disabled={updatingKey === flag.key || !flag.enabled}
        onChange={(e) => {
          void handleUpdateRollout(flag, parseInt(e.target.value, 10));
        }}
        className="h-11 w-24 cursor-pointer accent-primary disabled:opacity-40"
      />
      <span className="text-sm font-medium tabular-nums text-text">
        {labelled
          ? `${t("admin.rollout")}: ${flag.rollout_percent}%`
          : `${flag.rollout_percent}%`}
      </span>
    </div>
  );
  const expiry = (flag: FeatureFlag) => (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-sm tabular-nums",
        isExpired(flag)
          ? "font-semibold text-danger-accent"
          : "text-text-muted",
      )}
    >
      <Clock aria-hidden="true" className="h-4 w-4 shrink-0" />
      {flag.expires_on}
      {isExpired(flag) && ` (${t("admin.flagExpired")})`}
    </span>
  );
  const deleteButton = (flag: FeatureFlag) => (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      onClick={() => setDeleteTarget(flag.key)}
      disabled={deletingKey === flag.key}
      aria-label={`${t("admin.deleteFlag")}: ${flag.key}`}
      className="text-text-muted hover:bg-danger/10 hover:text-danger-accent"
    >
      <Trash2 aria-hidden="true" className="h-4 w-4" />
    </Button>
  );

  return (
    <div className="space-y-4">
      <AdminSectionHeader
        title={t("admin.featureFlagsRollouts")}
        description={t(
          "admin.featureFlagsSubtitle",
          "Control dynamic system feature switches, percentage rollouts, and owner assignments.",
        )}
        actions={
          <Button
            type="button"
            onClick={() => setIsCreateModalOpen(true)}
            className="whitespace-nowrap"
          >
            <Plus aria-hidden="true" className="h-4 w-4" />
            {t("admin.createFlag")}
          </Button>
        }
      />

      <AdminFilterCard>
        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
          <div className="relative w-full sm:min-w-0 sm:flex-1">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
            />
            <Input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label={t("admin.searchFlags")}
              placeholder={t("admin.searchFlags")}
              className="bg-surface-muted pl-10"
            />
          </div>
          <NativeSelect
            value={stateFilter}
            aria-label={t("admin.state")}
            onChange={(event) =>
              setStateFilter(event.target.value as "" | "enabled" | "disabled")
            }
            wrapperClassName="flex-1 sm:w-auto sm:flex-none"
          >
            <option value="">{t("admin.allStatusesFilter")}</option>
            <option value="enabled">{t("admin.enabled")}</option>
            <option value="disabled">{t("admin.disabled")}</option>
          </NativeSelect>
          <AdminRefreshButton onClick={() => reloadFlags()} busy={isLoading} />
        </div>
        {filtered && (
          <AdminActiveFilters onReset={resetFilters}>
            {term && <AdminFilterChip>{search.trim()}</AdminFilterChip>}
            {stateFilter && (
              <AdminFilterChip>
                {stateFilter === "enabled"
                  ? t("admin.enabled")
                  : t("admin.disabled")}
              </AdminFilterChip>
            )}
          </AdminActiveFilters>
        )}
      </AdminFilterCard>

      <AdminStatGrid
        stats={stats}
        loading={isLoading && flags.length === 0}
        label={t("admin.featureFlagsRollouts")}
      />

      {error && (
        <AdminNotice tone="danger">
          <span className="flex items-start gap-2">
            <AlertCircle aria-hidden="true" className="h-5 w-5 shrink-0" />
            {error}
          </span>
        </AdminNotice>
      )}

      <AdminDataPanel>
        {isLoading && flags.length === 0 ? (
          <AdminListSkeleton label={t("admin.loadingFlags")} />
        ) : flags.length === 0 ? (
          <EmptyState
            icon={Flag}
            title={t("admin.noFeatureFlagsConfigured")}
            description={t("admin.createAFeatureFlagToControlRollouts")}
            className="py-10"
          />
        ) : visibleFlags.length === 0 ? (
          <EmptyState
            icon={Flag}
            title={t("states.emptyTitle", "No items found")}
            description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
            action={
              <Button variant="outline" onClick={resetFilters}>
                {t("admin.resetFilters")}
              </Button>
            }
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[960px]")}>
                <caption className="sr-only">
                  {t("page.featureFlags", "Feature Flags")}
                </caption>
                <thead className={adminTable.head}>
                  <tr>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-14 text-right")}
                    >
                      {t("common.rowNumber")}
                    </th>
                    <th scope="col" className={adminTable.th}>
                      {t("admin.flagKeyDescription")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-48")}>
                      {t("admin.state")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("admin.rollout")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-40")}>
                      {t("admin.owner")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-40")}>
                      {t("admin.expiresOn")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-20 text-right")}
                    >
                      {t("admin.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {visibleFlags.map((flag, index) => (
                    <tr key={flag.key} className={cn(adminTable.row, "h-16")}>
                      <td
                        className={cn(
                          adminTable.td,
                          "text-right tabular-nums text-text-muted",
                        )}
                      >
                        {index + 1}
                      </td>
                      <td className={adminTable.td}>
                        <div className="flex min-w-0 items-center gap-3">
                          <AdminRowIcon icon={Flag} />
                          <div className="min-w-0">
                            <p
                              title={flag.key}
                              className="truncate font-mono text-sm font-semibold text-text"
                            >
                              {flag.key}
                            </p>
                            <p
                              title={flag.description}
                              className="truncate text-sm text-text-muted"
                            >
                              {flag.description}
                            </p>
                          </div>
                        </div>
                      </td>
                      <td className={adminTable.td}>{stateSwitch(flag)}</td>
                      <td className={adminTable.td}>{rollout(flag)}</td>
                      <td className={adminTable.td}>
                        <span
                          title={flag.owner}
                          className="inline-flex max-w-full items-center gap-1 truncate text-sm text-text-muted"
                        >
                          <User
                            aria-hidden="true"
                            className="h-4 w-4 shrink-0"
                          />
                          {flag.owner}
                        </span>
                      </td>
                      <td className={adminTable.td}>{expiry(flag)}</td>
                      <td className={cn(adminTable.td, "text-right")}>
                        {deleteButton(flag)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul className="divide-y divide-border-subtle md:hidden">
              {visibleFlags.map((flag) => (
                <li key={flag.key} className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    <AdminRowIcon icon={Flag} className="h-10 w-10" />
                    <div className="min-w-0 flex-1">
                      <p className="break-all font-mono text-sm font-semibold text-text">
                        {flag.key}
                      </p>
                      <p className="break-words text-sm text-text-muted">
                        {flag.description}
                      </p>
                    </div>
                    {deleteButton(flag)}
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    {stateSwitch(flag)}
                    {rollout(flag, true)}
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-text-muted">
                    <span className="inline-flex min-w-0 items-center gap-1 break-all">
                      <User aria-hidden="true" className="h-4 w-4 shrink-0" />
                      {flag.owner}
                    </span>
                    {expiry(flag)}
                  </div>
                </li>
              ))}
            </ul>
          </>
        )}
      </AdminDataPanel>

      <CreateFeatureFlagModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onSuccess={(newFlag) => {
          setFlags((prev) => [...prev, newFlag]);
        }}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title={t("admin.deleteFlag")}
        description={
          deleteTarget
            ? t("admin.confirmDeleteFlag", {
                key: deleteTarget,
                defaultValue: `Delete feature flag "${deleteTarget}"?`,
              })
            : ""
        }
        cancelLabel={t("common.cancel", "Cancel")}
        confirmLabel={t("admin.deleteFlag")}
        destructive
        isPending={deleteTarget !== null && deletingKey === deleteTarget}
        onConfirm={() => {
          if (deleteTarget) void handleDelete(deleteTarget);
        }}
      />
    </div>
  );
};
