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
              : t(
                  "admin.failedToLoadUserDetails",
                  "Failed to load user details.",
                ),
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
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="user-detail-modal-title"
        className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 backdrop-blur-sm p-4"
      >
        <div className="w-full max-w-2xl rounded-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-6">
          <div className="flex items-center justify-between border-b border-border-subtle pb-4">
            <h2
              id="user-detail-modal-title"
              className="text-lg font-semibold text-text flex items-center gap-2"
            >
              <User className="h-5 w-5 text-primary-accent" />
              {t("admin.learnerAccountDetails", "Learner Account Details")}
            </h2>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-1 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {isLoading ? (
            <div className="flex min-h-[250px] items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
            </div>
          ) : error ? (
            <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-4 text-xs text-danger-accent">
              <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
              <span>{error}</span>
            </div>
          ) : detail ? (
            <div className="space-y-6">
              {/* Audited notice */}
              <div className="rounded-lg border border-primary/30 bg-primary/10 px-3.5 py-2.5 text-xs text-primary-accent flex items-center gap-2">
                <Shield className="h-4 w-4 shrink-0 text-primary-accent" />
                <span>
                  This account profile read was logged to the security audit
                  trail (<code>admin.user_viewed</code>).
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
                <div className="space-y-1">
                  <div className="flex items-center gap-2.5">
                    <h3 className="text-base font-semibold text-text">
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
                  <p className="text-xs text-text-muted flex items-center gap-1.5">
                    <Mail className="h-3.5 w-3.5" />
                    {detail.email}
                  </p>
                </div>
              </div>

              {/* Meta Grid */}
              <div className="grid grid-cols-2 gap-4 text-xs">
                <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                  <span className="text-text-muted">
                    {t("admin.userId", "User ID")}
                  </span>
                  <p className="font-mono text-text truncate">{detail.id}</p>
                </div>
                <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                  <span className="text-text-muted">
                    {t("admin.languageLocale", "Language / Locale")}
                  </span>
                  <p className="text-text flex items-center gap-1">
                    <Globe className="h-3.5 w-3.5 text-text-muted" />
                    {detail.locale}
                  </p>
                </div>
                <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                  <span className="text-text-muted">
                    {t("admin.timezone", "Timezone")}
                  </span>
                  <p className="text-text">{detail.timezone}</p>
                </div>
                <div className="rounded-lg border border-border-subtle bg-surface-card/40 p-3 space-y-1">
                  <span className="text-text-muted">
                    {t("admin.registered", "Registered")}
                  </span>
                  <p className="text-text flex items-center gap-1">
                    <Clock className="h-3.5 w-3.5 text-text-muted" />
                    {new Date(detail.created_at).toLocaleString()}
                  </p>
                </div>
              </div>

              {/* Administrative Actions */}
              <div className="border-t border-border-subtle pt-4 space-y-3">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                  {t("admin.administrativeControls", "Administrative Controls")}
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
                      <Ban className="mr-1.5 h-3.5 w-3.5" />
                      {t("admin.suspendUser", "Suspend User")}
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
                      <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                      {t("admin.reinstateUser", "Reinstate User")}
                    </Button>
                  ) : null}

                  {can(PERMISSIONS.userManageSessions) && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setActionType("revoke_sessions")}
                    >
                      <LogOut className="mr-1.5 h-3.5 w-3.5" />
                      {t(
                        "admin.revokeActiveSessions",
                        "Revoke Active Sessions",
                      )}
                    </Button>
                  )}
                </div>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {actionType && (
        <AdminActionReasonModal
          isOpen={true}
          title={
            actionType === "suspend"
              ? t("admin.suspendAccount", "Suspend Account")
              : actionType === "reinstate"
                ? t("admin.reinstateAccount", "Reinstate Account")
                : t("admin.revokeActiveSessions", "Revoke Active Sessions")
          }
          description={
            actionType === "suspend"
              ? t(
                  "admin.suspendingWillImmediatelyBlockLogin",
                  "Suspending will immediately block login and terminate all active sessions.",
                )
              : actionType === "reinstate"
                ? t(
                    "admin.reinstatingRestoresFullAccountAccess",
                    "Reinstating restores full account access. Active sessions are not restored.",
                  )
                : t(
                    "admin.signsTheUserOutOf",
                    "Signs the user out of all active devices without changing account status.",
                  )
          }
          actionButtonLabel={
            actionType === "suspend"
              ? t("admin.confirmSuspension", "Confirm Suspension")
              : actionType === "reinstate"
                ? t("admin.confirmReinstatement", "Confirm Reinstatement")
                : t("admin.revokeAllSessions", "Revoke All Sessions")
          }
          variant={actionType === "suspend" ? "destructive" : "primary"}
          onClose={() => setActionType(null)}
          onConfirm={handleExecuteAction}
        />
      )}
    </>
  );
};
