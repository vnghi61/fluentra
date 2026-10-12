import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Ban,
  Clock,
  Globe,
  Loader2,
  LogOut,
  Mail,
  RotateCcw,
  Shield,
  User,
  X,
} from "lucide-react";
import { adminApi, type AdminUserDetail } from "../api/adminApi";
import { AdminActionReasonModal } from "./AdminActionReasonModal";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { Button } from "@/components/ui/button";
import * as Dialog from "@radix-ui/react-dialog";

interface AdminUserDetailModalProps {
  userId: string | null;
  onClose: () => void;
  onUserStatusChanged?: () => void;
}

export const AdminUserDetailModal: React.FC<AdminUserDetailModalProps> = ({
  userId,
  onClose,
  onUserStatusChanged,
}) => {
  const { t } = useTranslation();
  const [detail, setDetail] = useState<AdminUserDetail | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionType, setActionType] = useState<
    "suspend" | "reinstate" | "revoke_sessions" | null
  >(null);

  // Suspend, reinstate and session revocation are three separate permissions
  // (user.suspend, user.reinstate, user.manage_sessions), so an administrator
  // granted one is not shown the other two. The server checks each of them on
  // its own operation; this only stops the button existing.
  const { can } = usePermissions();

  useEffect(() => {
    if (!userId) return;

    let isMounted = true;
    async function fetchUserDetail(id: string) {
      setIsLoading(true);
      setError(null);
      try {
        const data = await adminApi.getUser(id);
        if (isMounted) {
          setDetail(data);
          setIsLoading(false);
        }
      } catch (err: unknown) {
        if (isMounted) {
          setError(
            err instanceof Error
              ? err.message
              : t("admin.failedToLoadUserDetails"),
          );
          setIsLoading(false);
        }
      }
    }

    void fetchUserDetail(userId);
    return () => {
      isMounted = false;
    };
  }, [userId, t]);

  if (!userId) return null;

  const handleExecuteAction = async (reason: string) => {
    if (!detail) return;

    if (actionType === "suspend") {
      await adminApi.suspendUser(detail.id, reason);
      setDetail((prev) => (prev ? { ...prev, status: "suspended" } : null));
    } else if (actionType === "reinstate") {
      await adminApi.reinstateUser(detail.id, reason);
      setDetail((prev) => (prev ? { ...prev, status: "active" } : null));
    } else if (actionType === "revoke_sessions") {
      await adminApi.revokeUserSessions(detail.id, reason);
    }

    onUserStatusChanged?.();
    setActionType(null);
  };

  return (
    <>
      <Dialog.Root
        open
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 z-50 bg-overlay/80 backdrop-blur-sm" />
          <Dialog.Content className="fixed inset-x-0 bottom-0 z-50 max-h-[90dvh] w-full overflow-y-auto rounded-t-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl md:inset-auto md:left-1/2 md:top-1/2 md:w-[min(100%-2rem,42rem)] md:-translate-x-1/2 md:-translate-y-1/2 md:rounded-2xl space-y-4">
            <div className="flex items-center justify-between gap-2 border-b border-border-subtle pb-4">
              <Dialog.Title className="flex min-w-0 items-center gap-2 break-words text-lg font-semibold leading-tight text-text">
                <User
                  aria-hidden="true"
                  className="h-5 w-5 shrink-0 text-primary-accent"
                />
                {t("admin.learnerAccountDetails")}
              </Dialog.Title>
              <button
                type="button"
                onClick={onClose}
                aria-label={t("common.close", "Close")}
                className="flex h-11 w-11 min-h-[44px] min-w-[44px] shrink-0 items-center justify-center rounded-lg text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
              >
                <X aria-hidden="true" className="h-5 w-5" />
              </button>
            </div>

            {isLoading ? (
              <div className="flex min-h-[250px] items-center justify-center">
                <Loader2
                  aria-hidden="true"
                  className="h-8 w-8 animate-spin text-primary-accent motion-reduce:animate-none"
                />
              </div>
            ) : error ? (
              <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-4 text-sm text-danger-accent">
                <AlertCircle
                  aria-hidden="true"
                  className="h-5 w-5 shrink-0 text-danger-accent"
                />
                <span>{error}</span>
              </div>
            ) : detail ? (
              <div className="space-y-6">
                {/* Audited notice */}
                <div className="flex items-start gap-2 rounded-lg border border-primary/30 bg-primary/10 px-3.5 py-2.5 text-sm leading-relaxed text-primary-accent">
                  <Shield
                    aria-hidden="true"
                    className="mt-0.5 h-5 w-5 shrink-0 text-primary-accent"
                  />
                  <span>
                    {t(
                      "admin.userViewedAuditNotice",
                      "This account profile read was logged to the security audit trail",
                    )}{" "}
                    (<code className="break-all">admin.user_viewed</code>).
                  </span>
                </div>

                {/* Profile Card */}
                <div className="flex items-center gap-4 rounded-xl border border-border-subtle bg-surface-muted p-4">
                  <div className="h-16 w-16 overflow-hidden rounded-full border-2 border-primary/40 bg-surface-muted flex items-center justify-center text-xl font-bold text-primary-accent shrink-0">
                    {detail.avatar_url ? (
                      <img
                        src={detail.avatar_url}
                        alt={detail.display_name}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <span>{detail.display_name.charAt(0).toUpperCase()}</span>
                    )}
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="min-w-0 break-words text-base font-semibold leading-snug text-text">
                        {detail.display_name}
                      </h3>
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium uppercase border ${
                          detail.status === "active"
                            ? "bg-success/10 text-success-accent border-success/20"
                            : detail.status === "suspended"
                              ? "bg-danger/10 text-danger-accent border-danger/20"
                              : "bg-warning/10 text-warning-accent border-warning/20"
                        }`}
                      >
                        {detail.status}
                      </span>
                    </div>
                    <p className="flex min-w-0 items-center gap-1.5 text-sm text-text-muted">
                      <Mail aria-hidden="true" className="h-4 w-4 shrink-0" />
                      <span className="min-w-0 break-all">{detail.email}</span>
                    </p>
                  </div>
                </div>

                {/* Meta Grid */}
                <div className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 sm:gap-4">
                  <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                    <span className="block text-text-muted">
                      {t("admin.userId")}
                    </span>
                    <p className="font-mono text-text truncate">{detail.id}</p>
                  </div>
                  <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                    <span className="block text-text-muted">
                      {t("admin.languageLocale")}
                    </span>
                    <p className="text-text flex items-center gap-1">
                      <Globe
                        aria-hidden="true"
                        className="h-4 w-4 shrink-0 text-text-muted"
                      />
                      {detail.locale}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                    <span className="block text-text-muted">
                      {t("admin.timezone")}
                    </span>
                    <p className="break-words text-text">{detail.timezone}</p>
                  </div>
                  <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                    <span className="block text-text-muted">
                      {t("admin.registered")}
                    </span>
                    <p className="text-text flex items-center gap-1">
                      <Clock
                        aria-hidden="true"
                        className="h-4 w-4 shrink-0 text-text-muted"
                      />
                      {new Date(detail.created_at).toLocaleString()}
                    </p>
                  </div>
                </div>

                {/* Administrative Actions */}
                <div className="border-t border-border-subtle pt-4 space-y-3">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                    {t("admin.administrativeControls")}
                  </h4>
                  <div className="flex flex-wrap gap-3">
                    {detail.status === "active" &&
                    can(PERMISSIONS.userSuspend) ? (
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        onClick={() => setActionType("suspend")}
                      >
                        <Ban aria-hidden="true" className="h-4 w-4" />
                        {t("admin.suspendUser")}
                      </Button>
                    ) : detail.status === "suspended" &&
                      can(PERMISSIONS.userReinstate) ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setActionType("reinstate")}
                        className="border-success/40 text-success-accent hover:bg-success/10"
                      >
                        <RotateCcw aria-hidden="true" className="h-4 w-4" />
                        {t("admin.reinstateUser")}
                      </Button>
                    ) : null}

                    {can(PERMISSIONS.userManageSessions) && (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setActionType("revoke_sessions")}
                      >
                        <LogOut aria-hidden="true" className="h-4 w-4" />
                        {t("admin.revokeActiveSessions")}
                      </Button>
                    )}
                  </div>
                </div>
              </div>
            ) : null}
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>

      {actionType && (
        <AdminActionReasonModal
          isOpen={true}
          title={
            actionType === "suspend"
              ? t("admin.suspendAccount")
              : actionType === "reinstate"
                ? t("admin.reinstateAccount")
                : t("admin.revokeActiveSessions")
          }
          description={
            actionType === "suspend"
              ? t("admin.suspendingWillImmediatelyBlockLogin")
              : actionType === "reinstate"
                ? t("admin.reinstatingRestoresFullAccountAccess")
                : t("admin.signsTheUserOutOf")
          }
          actionButtonLabel={
            actionType === "suspend"
              ? t("admin.confirmSuspension")
              : actionType === "reinstate"
                ? t("admin.confirmReinstatement")
                : t("admin.revokeAllSessions")
          }
          variant={actionType === "suspend" ? "destructive" : "primary"}
          onClose={() => setActionType(null)}
          onConfirm={handleExecuteAction}
        />
      )}
    </>
  );
};
