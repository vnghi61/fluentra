import React, { useEffect, useState } from "react";
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
            err instanceof Error ? err.message : "Failed to load user details.",
          );
          setIsLoading(false);
        }
      }
    }

    void fetchUserDetail(userId);
    return () => {
      isMounted = false;
    };
  }, [userId]);

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
        className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4"
      >
        <div className="w-full max-w-2xl rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl space-y-6">
          <div className="flex items-center justify-between border-b border-slate-800 pb-4">
            <h2
              id="user-detail-modal-title"
              className="text-lg font-semibold text-slate-100 flex items-center gap-2"
            >
              <User className="h-5 w-5 text-indigo-400" />
              Learner Account Details
            </h2>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {isLoading ? (
            <div className="flex min-h-[250px] items-center justify-center">
              <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
            </div>
          ) : error ? (
            <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-4 text-xs text-rose-300">
              <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
              <span>{error}</span>
            </div>
          ) : detail ? (
            <div className="space-y-6">
              {/* Audited notice */}
              <div className="rounded-lg border border-indigo-500/30 bg-indigo-500/10 px-3.5 py-2.5 text-xs text-indigo-300 flex items-center gap-2">
                <Shield className="h-4 w-4 shrink-0 text-indigo-400" />
                <span>
                  This account profile read was logged to the security audit
                  trail (<code>admin.user_viewed</code>).
                </span>
              </div>

              {/* Profile Card */}
              <div className="flex items-center gap-4 rounded-xl border border-slate-800 bg-slate-950 p-4">
                <div className="h-16 w-16 overflow-hidden rounded-full border-2 border-indigo-500/40 bg-slate-800 flex items-center justify-center text-xl font-bold text-indigo-300 shrink-0">
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
                    <h3 className="text-base font-semibold text-slate-100">
                      {detail.display_name}
                    </h3>
                    <span
                      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium uppercase border ${
                        detail.status === "active"
                          ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                          : detail.status === "suspended"
                            ? "bg-rose-500/10 text-rose-400 border-rose-500/20"
                            : "bg-amber-500/10 text-amber-400 border-amber-500/20"
                      }`}
                    >
                      {detail.status}
                    </span>
                  </div>
                  <p className="text-xs text-slate-400 flex items-center gap-1.5">
                    <Mail className="h-3.5 w-3.5" />
                    {detail.email}
                  </p>
                </div>
              </div>

              {/* Meta Grid */}
              <div className="grid grid-cols-2 gap-4 text-xs">
                <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-3 space-y-1">
                  <span className="text-slate-400">User ID</span>
                  <p className="font-mono text-slate-200 truncate">
                    {detail.id}
                  </p>
                </div>
                <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-3 space-y-1">
                  <span className="text-slate-400">Language / Locale</span>
                  <p className="text-slate-200 flex items-center gap-1">
                    <Globe className="h-3.5 w-3.5 text-slate-400" />
                    {detail.locale}
                  </p>
                </div>
                <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-3 space-y-1">
                  <span className="text-slate-400">Timezone</span>
                  <p className="text-slate-200">{detail.timezone}</p>
                </div>
                <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-3 space-y-1">
                  <span className="text-slate-400">Registered</span>
                  <p className="text-slate-200 flex items-center gap-1">
                    <Clock className="h-3.5 w-3.5 text-slate-400" />
                    {new Date(detail.created_at).toLocaleString()}
                  </p>
                </div>
              </div>

              {/* Administrative Actions */}
              <div className="border-t border-slate-800 pt-4 space-y-3">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  Administrative Controls
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
                      Suspend User
                    </Button>
                  ) : detail.status === "suspended" &&
                    can(PERMISSIONS.userReinstate) ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setActionType("reinstate")}
                      className="border-emerald-500/40 text-emerald-400 hover:bg-emerald-500/10"
                    >
                      <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                      Reinstate User
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
                      Revoke Active Sessions
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
              ? "Suspend Account"
              : actionType === "reinstate"
                ? "Reinstate Account"
                : "Revoke Active Sessions"
          }
          description={
            actionType === "suspend"
              ? "Suspending will immediately block login and terminate all active sessions."
              : actionType === "reinstate"
                ? "Reinstating restores full account access. Active sessions are not restored."
                : "Signs the user out of all active devices without changing account status."
          }
          actionButtonLabel={
            actionType === "suspend"
              ? "Confirm Suspension"
              : actionType === "reinstate"
                ? "Confirm Reinstatement"
                : "Revoke All Sessions"
          }
          variant={actionType === "suspend" ? "destructive" : "primary"}
          onClose={() => setActionType(null)}
          onConfirm={handleExecuteAction}
        />
      )}
    </>
  );
};
