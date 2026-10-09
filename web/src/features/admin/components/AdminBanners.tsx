import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CalendarClock,
  Clock,
  Megaphone,
  Pencil,
  Plus,
  Power,
  Trash2,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Switch } from "@/components/ui/switch";
import {
  BannerThumbnail,
  bannerStatus,
  useAdminBanners,
  useDeleteBanner,
  useUpdateBanner,
  type Banner,
  type BannerRequest,
  type BannerStatus,
} from "@/features/banners";
import { cn } from "@/lib/utils";
import {
  AdminDataPanel,
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
import { BannerFormDialog } from "./BannerFormDialog";
import { StatusBadge, type StatusTone } from "./StatusBadge";

const STATUS_TONE: Record<BannerStatus, StatusTone> = {
  active: "success",
  scheduled: "info",
  expired: "warning",
  off: "neutral",
};

/** Banners, as the request that edits one expects them back. */
/**
 * The PUT body for a switch flip. PUT replaces the whole banner, so the image
 * is carried over: a body without it would clear the picture, and the server
 * deletes an image its banner no longer names.
 */
function toRequest(banner: Banner, isActive: boolean): BannerRequest {
  return {
    kind: banner.kind,
    title: banner.title,
    body: banner.body,
    cta_label: banner.cta_label,
    cta_url: banner.cta_url,
    tone: banner.tone,
    priority: banner.priority,
    is_active: isActive,
    dismissible: banner.dismissible,
    starts_at: banner.starts_at,
    ends_at: banner.ends_at,
    image_key: banner.image_key ?? null,
    image_alt: banner.image_alt ?? "",
  };
}

export function AdminBanners(): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const {
    data,
    dataUpdatedAt,
    isLoading,
    isFetching,
    isError,
    error,
    refetch,
  } = useAdminBanners();
  const updateBanner = useUpdateBanner();
  const deleteBanner = useDeleteBanner();

  const [editing, setEditing] = React.useState<Banner | null>(null);
  const [formOpen, setFormOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<Banner | null>(null);
  const [actionError, setActionError] = React.useState<string | null>(null);
  // Status is judged against when the list arrived, not re-read per render.
  const now = dataUpdatedAt;

  const banners = data?.items ?? [];
  const locale = i18n.resolvedLanguage;
  const formatDate = (iso: string) =>
    new Date(iso).toLocaleString(locale, {
      dateStyle: "medium",
      timeStyle: "short",
    });

  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };
  const openEdit = (banner: Banner) => {
    setEditing(banner);
    setFormOpen(true);
  };

  const toggleActive = (banner: Banner, next: boolean) => {
    setActionError(null);
    updateBanner.mutate(
      { id: banner.id, body: toRequest(banner, next) },
      {
        onError: (err) =>
          setActionError(err.message || t("adminBanners.saveFailed")),
      },
    );
  };

  const confirmDelete = () => {
    if (!deleteTarget) return;
    setActionError(null);
    deleteBanner.mutate(deleteTarget.id, {
      onSuccess: () => setDeleteTarget(null),
      onError: (err) => {
        setDeleteTarget(null);
        setActionError(err.message || t("adminBanners.deleteFailed"));
      },
    });
  };

  const counts = banners.reduce<Record<BannerStatus, number>>(
    (acc, banner) => {
      acc[bannerStatus(banner, now)] += 1;
      return acc;
    },
    { active: 0, scheduled: 0, expired: 0, off: 0 },
  );
  const stats: AdminStat[] = [
    {
      key: "active",
      label: t("adminBanners.status.active"),
      value: counts.active.toLocaleString(locale),
      Icon: Power,
      tone: adminStatTones.success,
    },
    {
      key: "scheduled",
      label: t("adminBanners.status.scheduled"),
      value: counts.scheduled.toLocaleString(locale),
      Icon: CalendarClock,
      tone: adminStatTones.primary,
    },
    {
      key: "expired",
      label: t("adminBanners.status.expired"),
      value: counts.expired.toLocaleString(locale),
      Icon: Clock,
      tone: adminStatTones.warning,
    },
    {
      key: "off",
      label: t("adminBanners.status.off"),
      value: counts.off.toLocaleString(locale),
      Icon: Megaphone,
      tone: adminStatTones.neutral,
    },
  ];

  const statusBadge = (banner: Banner) => {
    const status = bannerStatus(banner, now);
    return (
      <StatusBadge tone={STATUS_TONE[status]}>
        <span
          aria-hidden="true"
          className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
        />
        {t(`adminBanners.status.${status}`)}
      </StatusBadge>
    );
  };
  // A slide is a picture, so its tone means nothing; say what it is instead.
  const toneLabel = (banner: Banner) => (
    <span className="text-sm text-text">
      {banner.kind === "slide"
        ? t("adminBanners.kind.slide")
        : `${t("adminBanners.kind.announcement")} · ${t(`adminBanners.tone.${banner.tone}`)}`}
    </span>
  );
  const windowText = (banner: Banner) => {
    if (!banner.starts_at && !banner.ends_at) {
      return t("adminBanners.windowAlways");
    }
    const from = banner.starts_at
      ? formatDate(banner.starts_at)
      : t("adminBanners.windowNow");
    const to = banner.ends_at
      ? formatDate(banner.ends_at)
      : t("adminBanners.windowOpenEnded");
    return `${from} – ${to}`;
  };
  const activeSwitch = (banner: Banner) => (
    <Switch
      aria-label={t("adminBanners.toggleActive", { title: banner.title })}
      checked={banner.is_active}
      disabled={
        updateBanner.isPending && updateBanner.variables?.id === banner.id
      }
      onCheckedChange={(checked) => toggleActive(banner, checked)}
    />
  );
  const rowActions = (banner: Banner) => (
    <div className="flex items-center justify-end gap-2">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={() => openEdit(banner)}
        aria-label={t("adminBanners.editBanner", { title: banner.title })}
      >
        <Pencil aria-hidden="true" className="h-4 w-4" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={() => setDeleteTarget(banner)}
        aria-label={t("adminBanners.deleteBanner", { title: banner.title })}
        className="text-text-muted hover:bg-danger/10 hover:text-danger-accent"
      >
        <Trash2 aria-hidden="true" className="h-4 w-4" />
      </Button>
    </div>
  );

  return (
    <div className="space-y-4">
      <AdminSectionHeader
        title={t("adminBanners.title")}
        description={t("adminBanners.subtitle")}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <AdminRefreshButton
              onClick={() => void refetch()}
              busy={isFetching}
            />
            <Button
              type="button"
              onClick={openCreate}
              className="whitespace-nowrap"
            >
              <Plus aria-hidden="true" className="h-4 w-4" />
              {t("adminBanners.create")}
            </Button>
          </div>
        }
      />

      <AdminStatGrid
        stats={stats}
        loading={isLoading}
        label={t("adminBanners.title")}
      />

      {(isError || actionError) && (
        <AdminNotice tone="danger">
          <span className="flex items-start gap-2">
            <AlertCircle aria-hidden="true" className="h-5 w-5 shrink-0" />
            {actionError ??
              (error instanceof Error && error.message
                ? error.message
                : t("adminBanners.loadFailed"))}
          </span>
        </AdminNotice>
      )}

      <AdminDataPanel>
        {isLoading ? (
          <AdminListSkeleton label={t("adminBanners.loading")} />
        ) : banners.length === 0 ? (
          <EmptyState
            icon={Megaphone}
            title={t("adminBanners.emptyTitle")}
            description={t("adminBanners.emptyDescription")}
            action={
              <Button type="button" onClick={openCreate}>
                <Plus aria-hidden="true" className="h-4 w-4" />
                {t("adminBanners.create")}
              </Button>
            }
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[960px]")}>
                <caption className="sr-only">{t("adminBanners.title")}</caption>
                <thead className={adminTable.head}>
                  <tr>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-14 text-right")}
                    >
                      {t("common.rowNumber")}
                    </th>
                    <th scope="col" className={adminTable.th}>
                      {t("adminBanners.colBanner")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-32")}>
                      {t("adminBanners.colStatus")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-28")}>
                      {t("adminBanners.colKind")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-24 text-right")}
                    >
                      {t("adminBanners.fieldPriority")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-64")}>
                      {t("adminBanners.colWindow")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-20")}>
                      {t("adminBanners.fieldActive")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-28 text-right")}
                    >
                      {t("common.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {banners.map((banner, index) => (
                    <tr key={banner.id} className={cn(adminTable.row, "h-16")}>
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
                          {banner.image_url ? (
                            <BannerThumbnail url={banner.image_url} />
                          ) : (
                            <AdminRowIcon icon={Megaphone} />
                          )}
                          <div className="min-w-0">
                            <p
                              title={banner.title}
                              className="truncate text-sm font-semibold text-text"
                            >
                              {banner.title}
                            </p>
                            {banner.body && (
                              <p
                                title={banner.body}
                                className="truncate text-sm text-text-muted"
                              >
                                {banner.body}
                              </p>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className={adminTable.td}>{statusBadge(banner)}</td>
                      <td className={adminTable.td}>{toneLabel(banner)}</td>
                      <td
                        className={cn(
                          adminTable.td,
                          "text-right tabular-nums text-text",
                        )}
                      >
                        {banner.priority}
                      </td>
                      <td
                        className={cn(adminTable.td, "text-sm text-text-muted")}
                      >
                        {windowText(banner)}
                      </td>
                      <td className={adminTable.td}>{activeSwitch(banner)}</td>
                      <td className={adminTable.td}>{rowActions(banner)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul className="divide-y divide-border-subtle md:hidden">
              {banners.map((banner) => (
                <li key={banner.id} className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    {banner.image_url ? (
                      <BannerThumbnail
                        url={banner.image_url}
                        className="h-10 w-10"
                      />
                    ) : (
                      <AdminRowIcon icon={Megaphone} className="h-10 w-10" />
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="break-words text-sm font-semibold text-text">
                        {banner.title}
                      </p>
                      {banner.body && (
                        <p className="line-clamp-2 break-words text-sm text-text-muted">
                          {banner.body}
                        </p>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {statusBadge(banner)}
                    {toneLabel(banner)}
                    <span className="text-sm tabular-nums text-text-muted">
                      {t("adminBanners.priorityValue", {
                        value: banner.priority,
                      })}
                    </span>
                  </div>
                  <p className="text-sm text-text-muted">
                    {windowText(banner)}
                  </p>
                  <div className="flex items-center justify-between gap-2">
                    {activeSwitch(banner)}
                    {rowActions(banner)}
                  </div>
                </li>
              ))}
            </ul>
          </>
        )}
      </AdminDataPanel>

      <BannerFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        banner={editing}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open && !deleteBanner.isPending) setDeleteTarget(null);
        }}
        title={t("adminBanners.deleteTitle")}
        description={
          deleteTarget
            ? t("adminBanners.deleteConfirm", { title: deleteTarget.title })
            : ""
        }
        cancelLabel={t("common.cancel")}
        confirmLabel={t("adminBanners.delete")}
        destructive
        keepOpenOnConfirm
        isPending={deleteBanner.isPending}
        onConfirm={confirmDelete}
      />
    </div>
  );
}
