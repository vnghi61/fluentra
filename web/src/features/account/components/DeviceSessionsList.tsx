import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  Laptop,
  LogOut,
  MonitorSmartphone,
  MoreVertical,
  ShieldCheck,
  ShieldOff,
  Smartphone,
  type LucideIcon,
} from "lucide-react";
import {
  accountApi,
  useSessions,
  useTrustedDevices,
  type SessionSummary,
  type TrustedDevice,
} from "../api/accountApi";
import { accountKeys } from "../api/keys";
import { groupDeviceSessions, type DeviceGroup } from "../model/deviceSessions";
import { ApiError } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { IconTile } from "./SettingsCard";
import {
  formatDate,
  formatDateTime,
  formatRelativeTime,
  isMobileLabel,
  settingsAlert,
  splitDeviceLabel,
} from "./settingsStyles";

interface DeviceSessionsListProps {
  onLoggedOut?: (() => void) | undefined;
}

type PendingConfirm =
  | { kind: "device"; group: DeviceGroup }
  | { kind: "untrust"; device: TrustedDevice }
  | { kind: "all" };

/** The pending marker for "sign out all other devices". */
const ALL = "\u0000all";

/** A 404 here means the row is already gone, which is the outcome asked for. */
function alreadyGone(err: unknown): boolean {
  return err instanceof ApiError && err.problem.status === 404;
}

const deviceIcon = (label: string | null): LucideIcon =>
  isMobileLabel(label) ? Smartphone : Laptop;

/**
 * "Devices & sign-ins": one card per device, its sessions grouped under it.
 *
 * Replaces the separate "Active sessions" and "Trusted devices" cards, which
 * listed the same laptop once per sign-in and once more for its trust record.
 * Location and IP rows are absent on purpose: the session API stores neither.
 */
export const DeviceSessionsList: React.FC<DeviceSessionsListProps> = ({
  onLoggedOut,
}) => {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const sessionsQuery = useSessions();
  const devicesQuery = useTrustedDevices();
  const [now] = useState(() => Date.now());
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<PendingConfirm | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const groups = useMemo(
    () =>
      groupDeviceSessions(
        sessionsQuery.data?.sessions ?? [],
        devicesQuery.data?.devices ?? [],
      ),
    [sessionsQuery.data, devicesQuery.data],
  );

  // Everything "sign out all other devices" would end: every sign-in but
  // this one, and every trust record but this browser's.
  const others = useMemo(
    () => ({
      sessions: groups.flatMap((g) => g.sessions.filter((s) => !s.current)),
      devices: groups.flatMap((g) => g.trusted.filter((d) => !d.current)),
    }),
    [groups],
  );
  const hasOthers = others.sessions.length > 0 || others.devices.length > 0;

  const isLoading = sessionsQuery.isPending || devicesQuery.isPending;
  const loadError = sessionsQuery.error ?? devicesQuery.error;

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: accountKeys.sessions() }),
      queryClient.invalidateQueries({ queryKey: accountKeys.devices() }),
    ]);
  };

  const labelOf = (label: string | null | undefined) => {
    const parts = splitDeviceLabel(label);
    if (parts) {
      return t("settings.devices.label", {
        ...parts,
        defaultValue: `${parts.browser} on ${parts.os}`,
      });
    }
    return label || t("account.unknownDevice", "Unknown Device");
  };

  const toggle = (key: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const failWith = (err: unknown, fallback: string) =>
    setActionError(
      err instanceof Error && err.message ? err.message : fallback,
    );

  const revokeSession = async (session: SessionSummary) => {
    if (session.current || pendingId) return;
    setPendingId(session.id);
    setActionError(null);
    try {
      await accountApi.revokeSession(session.id);
    } catch (err: unknown) {
      if (!alreadyGone(err)) {
        failWith(
          err,
          t("account.failedToRevokeSession", "Failed to revoke session."),
        );
      }
    } finally {
      await refresh();
      setPendingId(null);
    }
  };

  /**
   * Untrusts, then revokes, everything listed that is not the caller's own.
   * Untrusting first revokes a device's sessions server-side; the per-session
   * calls then mop up sign-ins that were not trusted (a 404 is "already gone").
   */
  const endAll = async (
    trusted: readonly TrustedDevice[],
    sessions: readonly SessionSummary[],
  ) => {
    for (const device of trusted) {
      if (device.current) continue;
      try {
        await accountApi.untrustDevice(device.id);
      } catch (err: unknown) {
        if (!alreadyGone(err)) throw err;
      }
    }
    for (const session of sessions) {
      if (session.current) continue;
      try {
        await accountApi.revokeSession(session.id);
      } catch (err: unknown) {
        if (!alreadyGone(err)) throw err;
      }
    }
  };

  /** Ends every sign-in on a device that is not this one, and its trust. */
  const signOutDevice = async (group: DeviceGroup) => {
    setPendingId(group.key);
    setActionError(null);
    try {
      await endAll(group.trusted, group.sessions);
    } catch (err: unknown) {
      failWith(
        err,
        t("account.failedToSignOutDevice", "Could not sign that device out."),
      );
    } finally {
      setConfirm(null);
      await refresh();
      setPendingId(null);
    }
  };

  /**
   * There is no "revoke all" operation for the caller's own account, so this
   * is the per-device and per-session calls the cards already make, in turn.
   */
  const signOutOthers = async () => {
    setPendingId(ALL);
    setActionError(null);
    try {
      await endAll(others.devices, others.sessions);
    } catch (err: unknown) {
      failWith(
        err,
        t(
          "settings.devices.signOutAllFailed",
          "Some devices could not be signed out. Try again.",
        ),
      );
    } finally {
      setConfirm(null);
      await refresh();
      setPendingId(null);
    }
  };

  const untrust = async (device: TrustedDevice) => {
    setPendingId(device.id);
    setActionError(null);
    try {
      await accountApi.untrustDevice(device.id);
      setConfirm(null);
    } catch (err: unknown) {
      failWith(
        err,
        t("account.failedToUntrustDevice", "Failed to untrust device."),
      );
      setConfirm(null);
      await refresh();
      setPendingId(null);
      return;
    }
    // Untrusting the device in hand signs the caller out, and the server
    // marks it `current` precisely so this does not have to guess. There is
    // nothing left to refetch with a revoked session.
    if (device.current) {
      setPendingId(null);
      onLoggedOut?.();
      return;
    }
    await refresh();
    setPendingId(null);
  };

  const otherSessionCount = (group: DeviceGroup) =>
    group.sessions.filter((s) => !s.current).length;

  const confirmCopy = (): {
    title: string;
    description: string | undefined;
    confirmLabel: string;
  } => {
    if (confirm?.kind === "all") {
      return {
        title: t(
          "settings.devices.signOutAllTitle",
          "Sign out all other devices?",
        ),
        description: t("settings.devices.signOutAllBody", {
          count: others.sessions.length,
          defaultValue: `Every other active sign-in (${others.sessions.length}) will end and other trusted devices will stop being trusted. You stay signed in on this device.`,
        }),
        confirmLabel: t(
          "settings.devices.signOutAll",
          "Sign out all other devices",
        ),
      };
    }
    if (confirm?.kind === "device") {
      const device = labelOf(confirm.group.label);
      return {
        title: t("account.signOutDeviceTitle", {
          device,
          defaultValue: `Sign out of ${device}?`,
        }),
        description: [
          t("account.signOutDeviceBody", {
            count: otherSessionCount(confirm.group),
            defaultValue: `Every active sign-in on this device (${otherSessionCount(confirm.group)}) will end. The device you are using now is unaffected.`,
          }),
          confirm.group.trusted.length > 0
            ? t(
                "account.signOutDeviceTrustedNote",
                "It will also stop being trusted.",
              )
            : "",
        ]
          .filter(Boolean)
          .join(" "),
        confirmLabel: t("account.signOutDevice", "Sign out"),
      };
    }
    return {
      title: t("account.stopTrustingThisDevice", "Stop trusting this device?"),
      description:
        confirm?.kind === "untrust"
          ? confirm.device.current
            ? t(
                "account.untrustCurrentWarning",
                "This is the device you are using right now. Untrusting it will sign you out of this browser immediately.",
              )
            : t(
                "account.untrustOtherWarning",
                "This device will stop being signed in. The device you are using now is unaffected.",
              )
          : undefined,
      confirmLabel: t("account.untrustSignOut", "Untrust & Sign Out"),
    };
  };
  const copy = confirmCopy();

  return (
    <section
      id="devices"
      aria-labelledby="devices-title"
      className="@container scroll-mt-4 space-y-4"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-1">
          <h2
            id="devices-title"
            tabIndex={-1}
            className="flex flex-wrap items-center gap-2 text-lg font-semibold leading-snug text-text focus-visible:outline-none md:text-xl"
          >
            {t("account.devicesSessionsTitle", "Devices & sign-ins")}
            {!isLoading && groups.length > 0 && (
              <Badge variant="secondary" className="tabular-nums">
                {groups.length}
              </Badge>
            )}
          </h2>
          <p className="max-w-2xl text-sm text-text-muted">
            {t(
              "account.devicesSessionsDesc",
              "Each device appears once, with the sign-ins still active on it. Sign out any device you do not recognise.",
            )}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          disabled={!hasOthers || pendingId !== null}
          isLoading={pendingId === ALL}
          onClick={() => setConfirm({ kind: "all" })}
          className="w-full shrink-0 hover:border-danger/40 hover:bg-danger/10 hover:text-danger-accent sm:w-auto"
        >
          {pendingId !== ALL && <LogOut className="h-4 w-4" aria-hidden />}
          {t("settings.devices.signOutAll", "Sign out all other devices")}
        </Button>
      </div>

      {(actionError || loadError) && (
        <div role="alert" className={settingsAlert("error")}>
          <AlertCircle className="h-5 w-5 shrink-0" aria-hidden />
          <span>
            {actionError ??
              t(
                "account.failedToLoadActiveSessions",
                "Failed to load active sessions.",
              )}
          </span>
        </div>
      )}

      {isLoading ? (
        <div
          className="grid gap-3 @2xl:grid-cols-2"
          data-testid="sessions-skeleton"
        >
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-44 w-full rounded-2xl" />
          ))}
        </div>
      ) : groups.length === 0 ? (
        loadError ? null : (
          <EmptyState
            variant="card"
            icon={MonitorSmartphone}
            title={t("account.noSignedInDevices", "No devices are signed in.")}
          />
        )
      ) : (
        <ul
          className="grid items-start gap-3 @2xl:grid-cols-2"
          aria-label={t("account.devicesSessionsTitle", "Devices & sign-ins")}
        >
          {groups.map((group, index) => {
            const label = labelOf(group.label);
            const parts = splitDeviceLabel(group.label);
            const isOpen = expanded.has(group.key);
            const detailsId = `device-details-${index}`;
            const canSignOut =
              !group.current &&
              (group.sessions.length > 0 || group.trusted.length > 0);
            const trustedSince = group.trusted[0]?.trusted_at;
            return (
              <li
                key={group.key}
                data-testid="device-row"
                className={cn(
                  "flex min-w-0 flex-col gap-4 rounded-2xl border bg-surface-card p-4 shadow-[var(--shadow-card)] sm:p-5",
                  group.current
                    ? "border-primary/40 ring-1 ring-primary/20"
                    : "border-border-subtle",
                )}
              >
                <div className="flex min-w-0 items-start gap-3">
                  <IconTile
                    icon={deviceIcon(group.label)}
                    tone={group.current ? "primary" : "muted"}
                    size="lg"
                  />
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                      <p className="min-w-0 break-words text-base font-semibold text-text">
                        {label}
                      </p>
                      {group.current && (
                        <Badge variant="success">
                          {t("settings.devices.currentBadge", "This device")}
                        </Badge>
                      )}
                      {group.trusted.length > 0 && (
                        <Badge
                          variant="primary"
                          icon={<ShieldCheck className="h-4 w-4" aria-hidden />}
                        >
                          {t("account.trustedBadge", "Trusted")}
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm text-text-muted">
                      {parts && (
                        <>
                          {parts.browser}
                          <span aria-hidden="true"> • </span>
                          <span className="sr-only">, </span>
                          {parts.os}
                          <span aria-hidden="true"> · </span>
                          <span className="sr-only">, </span>
                        </>
                      )}
                      {t("account.deviceSessionCount", {
                        count: group.sessions.length,
                        defaultValue: `${group.sessions.length} sign-ins`,
                      })}
                    </p>
                  </div>
                </div>

                <dl className="grid gap-2 rounded-xl bg-surface-muted/60 p-3 text-sm">
                  <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5">
                    <dt className="text-text-muted">
                      {t("settings.devices.lastActive", "Last active")}
                    </dt>
                    <dd className="font-medium text-text">
                      {group.current ? (
                        <span className="inline-flex items-center gap-1.5 text-success-accent">
                          <span
                            className="h-2 w-2 rounded-full bg-success"
                            aria-hidden="true"
                          />
                          {t("settings.devices.activeNow", "Active now")}
                        </span>
                      ) : group.lastSeenAt ? (
                        <time
                          dateTime={group.lastSeenAt}
                          title={formatDateTime(
                            group.lastSeenAt,
                            i18n.language,
                          )}
                        >
                          {formatRelativeTime(
                            group.lastSeenAt,
                            i18n.language,
                            now,
                          )}
                        </time>
                      ) : (
                        "—"
                      )}
                    </dd>
                  </div>
                  {trustedSince && (
                    <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5">
                      <dt className="text-text-muted">
                        {t("settings.devices.trustedSince", "Trusted since")}
                      </dt>
                      <dd className="font-medium text-text">
                        <time dateTime={trustedSince}>
                          {formatDate(trustedSince, i18n.language)}
                        </time>
                      </dd>
                    </div>
                  )}
                </dl>

                <div className="flex items-center gap-2">
                  {canSignOut ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={pendingId !== null}
                      isLoading={pendingId === group.key}
                      onClick={() => setConfirm({ kind: "device", group })}
                      aria-label={t("account.signOutDeviceAria", {
                        device: label,
                        defaultValue: `Sign out of ${label}`,
                      })}
                      className="flex-1 hover:border-danger/40 hover:bg-danger/10 hover:text-danger-accent"
                    >
                      {pendingId !== group.key && (
                        <LogOut className="h-4 w-4" aria-hidden />
                      )}
                      {t("account.signOutDevice", "Sign out")}
                    </Button>
                  ) : (
                    <Button
                      type="button"
                      variant="secondary"
                      disabled
                      className="flex-1"
                    >
                      {t("settings.devices.inUse", "In use")}
                    </Button>
                  )}
                  <DropdownMenu modal={false}>
                    <DropdownMenuTrigger asChild>
                      <Button
                        type="button"
                        variant="outline"
                        size="icon"
                        aria-label={t("settings.devices.moreActions", {
                          device: label,
                          defaultValue: `More actions for ${label}`,
                        })}
                        className="shrink-0 text-text-muted hover:text-text"
                      >
                        <MoreVertical className="h-5 w-5" aria-hidden />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => toggle(group.key)}>
                        <MonitorSmartphone
                          className="h-4 w-4 text-text-muted"
                          aria-hidden
                        />
                        {isOpen
                          ? t("settings.devices.hideSignIns", "Hide sign-ins")
                          : t("settings.devices.showSignIns", "Show sign-ins")}
                      </DropdownMenuItem>
                      {group.trusted.map((device) => (
                        <DropdownMenuItem
                          key={device.id}
                          destructive
                          disabled={pendingId !== null}
                          onSelect={() =>
                            setConfirm({ kind: "untrust", device })
                          }
                        >
                          <ShieldOff className="h-4 w-4" aria-hidden />
                          {t("account.stopTrusting", "Stop trusting")}
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {isOpen && (
                  <div
                    id={detailsId}
                    className="space-y-2 border-t border-border-subtle pt-3"
                  >
                    {group.trusted.map((device) => (
                      <p
                        key={device.id}
                        className="rounded-lg bg-surface-muted/60 px-3 py-2 text-sm text-text-muted"
                      >
                        {t("account.trustedSince", {
                          date: formatDate(device.trusted_at, i18n.language),
                          defaultValue: `Trusted since ${formatDate(device.trusted_at, i18n.language)}`,
                        })}
                        {" · "}
                        {t("account.expiresOn", {
                          date: formatDate(
                            device.idle_expires_at,
                            i18n.language,
                          ),
                          defaultValue: `Expires ${formatDate(device.idle_expires_at, i18n.language)}`,
                        })}
                      </p>
                    ))}

                    {group.sessions.length > 0 && (
                      <ul
                        className="divide-y divide-border-subtle"
                        aria-label={t("account.sessionsOnDevice", {
                          device: label,
                          defaultValue: `Sign-ins on ${label}`,
                        })}
                      >
                        {group.sessions.map((session) => {
                          const signedIn = formatDateTime(
                            session.created_at,
                            i18n.language,
                          );
                          return (
                            <li
                              key={session.id}
                              className="flex flex-wrap items-center justify-between gap-2 py-2"
                            >
                              <div className="min-w-0 text-sm text-text-muted">
                                <p className="text-text">
                                  {t("account.signedInAt", {
                                    date: signedIn,
                                    defaultValue: `Signed in ${signedIn}`,
                                  })}
                                </p>
                                <p>
                                  {t("account.lastActiveAt", {
                                    date: formatDateTime(
                                      session.last_seen_at,
                                      i18n.language,
                                    ),
                                    defaultValue: `Last active ${formatDateTime(session.last_seen_at, i18n.language)}`,
                                  })}
                                </p>
                              </div>
                              {session.current ? (
                                <Badge variant="success">
                                  {t("account.thisSession", "This session")}
                                </Badge>
                              ) : (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  disabled={pendingId !== null}
                                  isLoading={pendingId === session.id}
                                  onClick={() => {
                                    void revokeSession(session);
                                  }}
                                  aria-label={t("account.endSessionAria", {
                                    date: signedIn,
                                    defaultValue: `End the sign-in from ${signedIn}`,
                                  })}
                                  className="gap-1.5 text-text-muted hover:bg-danger/10 hover:text-danger-accent"
                                >
                                  {pendingId !== session.id && (
                                    <LogOut className="h-4 w-4" aria-hidden />
                                  )}
                                  {t("account.endSession", "End session")}
                                </Button>
                              )}
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(open) => {
          if (!open && pendingId === null) setConfirm(null);
        }}
        destructive
        keepOpenOnConfirm
        isPending={pendingId !== null}
        title={copy.title}
        description={copy.description}
        confirmLabel={copy.confirmLabel}
        onConfirm={() => {
          if (confirm?.kind === "device") void signOutDevice(confirm.group);
          else if (confirm?.kind === "untrust") void untrust(confirm.device);
          else if (confirm?.kind === "all") void signOutOthers();
        }}
      />
    </section>
  );
};
