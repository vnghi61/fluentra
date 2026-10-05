import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  CheckCircle2,
  Download,
  FileArchive,
  Loader2,
  Trash2,
  Undo2,
  AlertCircle,
} from "lucide-react";
import {
  accountApi,
  type DeletionResponse,
  type ExportResponse,
  type UserProfile,
} from "../api/accountApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  settingsAlert,
  settingsCard,
  settingsCardDesc,
  settingsCardTitle,
} from "./settingsStyles";

interface DataPrivacySettingsProps {
  initialProfile: UserProfile;
  onProfileUpdated?: ((profile: UserProfile) => void) | undefined;
}

export const DataPrivacySettings: React.FC<DataPrivacySettingsProps> = ({
  initialProfile,
  onProfileUpdated,
}) => {
  const { t } = useTranslation();
  const [profile, setProfile] = useState<UserProfile>(initialProfile);
  const [exportState, setExportState] = useState<ExportResponse | null>(null);
  const [deletionState, setDeletionState] = useState<DeletionResponse | null>(
    null,
  );
  const [isExporting, setIsExporting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);
  const [isConfirmModalOpen, setIsConfirmModalOpen] = useState(false);
  const [confirmInput, setConfirmInput] = useState("");
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const handleRequestExport = async () => {
    setIsExporting(true);
    setStatusMessage(null);

    try {
      const res = await accountApi.requestExport();
      setExportState(res);
      setStatusMessage({
        type: "success",
        text: t(
          "account.dataExportRequestedWeWill",
          "Data export requested. We will prepare your ZIP archive and send a secure link to your email.",
        ),
      });
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t(
                "account.failedToRequestDataExport",
                "Failed to request data export.",
              ),
      });
    } finally {
      setIsExporting(false);
    }
  };

  const handleRequestDeletion = async () => {
    setIsDeleting(true);
    setStatusMessage(null);

    try {
      const res = await accountApi.requestDeletion();
      setDeletionState(res);
      const updatedProfile: UserProfile = {
        ...profile,
        status: "pending_deletion",
      };
      setProfile(updatedProfile);
      onProfileUpdated?.(updatedProfile);
      setIsConfirmModalOpen(false);
      setConfirmInput("");
      setStatusMessage({
        type: "success",
        text: t(
          "account.accountDeletionRequestedYour30",
          "Account deletion requested. Your 30-day grace period has begun.",
        ),
      });
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t(
                "account.failedToRequestAccountDeletion",
                "Failed to request account deletion.",
              ),
      });
    } finally {
      setIsDeleting(false);
    }
  };

  const handleCancelDeletion = async () => {
    setIsCancelling(true);
    setStatusMessage(null);

    try {
      const res = await accountApi.cancelDeletion();
      setDeletionState(res);
      const updatedProfile: UserProfile = {
        ...profile,
        status: "active",
      };
      setProfile(updatedProfile);
      onProfileUpdated?.(updatedProfile);
      setStatusMessage({
        type: "success",
        text: t(
          "account.accountDeletionCancelledYourAccount",
          "Account deletion cancelled. Your account is fully restored to active status.",
        ),
      });
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t("account.failedToCancelDeletion", "Failed to cancel deletion."),
      });
    } finally {
      setIsCancelling(false);
    }
  };

  const isPendingDeletion =
    profile.status === "pending_deletion" ||
    deletionState?.status === "pending" ||
    deletionState?.status === "processing";

  return (
    <div className="space-y-4">
      {statusMessage && (
        <div role="status" className={settingsAlert(statusMessage.type)}>
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          ) : (
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Data Export (GDPR) */}
      <section className={`${settingsCard} space-y-3`}>
        <div className="space-y-1">
          <h3 className={settingsCardTitle}>
            <FileArchive className="h-5 w-5 text-primary-accent" aria-hidden />
            {t(
              "account.exportPersonalData",
              "Export Your Personal Data (GDPR Article 20)",
            )}
          </h3>
          <p className={`${settingsCardDesc} max-w-2xl`}>
            {t(
              "account.exportPersonalDataDesc",
              "You have the right to receive a copy of all personal data, study history, and vocabulary progress in a machine-readable ZIP format.",
            )}
          </p>
        </div>

        <div className="flex flex-col items-start justify-between gap-3 rounded-xl border border-border-subtle p-3 sm:flex-row sm:items-center sm:p-4">
          <div className="space-y-1">
            <p className="text-sm font-medium text-text">
              {t(
                "account.downloadPersonalArchive",
                "Download personal archive",
              )}
            </p>
            <p className="text-xs text-text-muted">
              {exportState
                ? `${t("account.exportStatus", "Export status")}: ${t(`account.exportState.${exportState.status}`, exportState.status)}`
                : t(
                    "account.includesAccountDetailsPreferencesStudy",
                    "Includes account details, preferences, study logs, and exercise records",
                  )}
            </p>
          </div>

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              void handleRequestExport();
            }}
            disabled={isExporting}
            className="min-h-[44px] shrink-0 whitespace-nowrap rounded-xl px-4 sm:min-h-10"
          >
            {isExporting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t("account.requesting", "Requesting...")}
              </>
            ) : (
              <>
                <Download className="mr-2 h-4 w-4" />
                {t("account.requestDataExport", "Request Data Export")}
              </>
            )}
          </Button>
        </div>
      </section>

      {/* Account Deletion (GDPR Article 17) */}
      <section className={`${settingsCard} space-y-3`}>
        <div className="space-y-1">
          <h3 className="flex items-center gap-2 text-base font-semibold text-danger-accent">
            <Trash2 className="h-5 w-5" aria-hidden />
            {t(
              "account.deleteAccountTitle",
              "Delete Account (GDPR Article 17)",
            )}
          </h3>
          <p className={`${settingsCardDesc} max-w-2xl`}>
            {t(
              "account.deleteAccountDesc",
              "Permanently erase your account, login credentials, learning stats, and personal data.",
            )}
          </p>
        </div>

        {isPendingDeletion ? (
          <div className="space-y-3 rounded-xl border border-warning/30 bg-warning/10 p-3 sm:p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle className="h-5 w-5 shrink-0 text-warning-accent mt-0.5" />
              <div className="space-y-1">
                <h4 className="text-sm font-semibold text-warning-accent">
                  {t(
                    "account.accountDeletionIsCurrentlyScheduled",
                    "Account deletion is currently scheduled",
                  )}
                </h4>
                <p className="text-xs text-warning-accent leading-relaxed">
                  {t(
                    "account.gracePeriodBody",
                    "Your account is in the 30-day grace period. All active sessions have been revoked, and all data across all modules will be completely purged when the grace period ends. You may cancel this deletion request at any time during this window to restore full access.",
                  )}
                </p>
                {deletionState?.execute_at && (
                  <p className="text-xs font-semibold text-warning-accent pt-1">
                    {t("account.scheduledExecution", "Scheduled execution")}:{" "}
                    {new Date(deletionState.execute_at).toLocaleString()}
                  </p>
                )}
              </div>
            </div>

            <div className="flex justify-end pt-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  void handleCancelDeletion();
                }}
                disabled={isCancelling}
                className="border-warning/40 text-warning-accent hover:bg-warning/20"
              >
                {isCancelling ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t("account.cancelling", "Cancelling...")}
                  </>
                ) : (
                  <>
                    <Undo2 className="mr-1.5 h-3.5 w-3.5" />
                    {t(
                      "account.cancelDeletionRestoreAccount",
                      "Cancel Deletion & Restore Account",
                    )}
                  </>
                )}
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-start justify-between gap-3 rounded-xl border border-danger/25 p-3 sm:flex-row sm:items-center sm:p-4">
            <div className="space-y-1">
              <p className="text-sm font-medium text-text">
                {t(
                  "account.initiateAccountErasure",
                  "Initiate account erasure",
                )}
              </p>
              <p className="text-xs text-text-muted">
                {t(
                  "account.initiateAccountErasureDesc",
                  "Begins a 30-day grace period during which you can cancel if you change your mind.",
                )}
              </p>
            </div>

            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => setIsConfirmModalOpen(true)}
              className="min-h-[44px] shrink-0 whitespace-nowrap rounded-xl px-4 sm:min-h-10"
            >
              {t("account.deleteMyAccount", "Delete My Account")}
            </Button>
          </div>
        )}
      </section>

      {/* Deletion Confirmation Modal */}
      {isConfirmModalOpen && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 backdrop-blur-sm p-4"
        >
          <div className="w-full max-w-md space-y-5 rounded-2xl border border-border-subtle bg-surface-card p-5 shadow-2xl sm:p-6">
            <div className="flex items-center gap-3 text-danger-accent">
              <AlertTriangle className="h-6 w-6" />
              <h4 className="text-lg font-semibold text-text">
                {t("account.areYouAbsolutelySure", "Are you absolutely sure?")}
              </h4>
            </div>

            <div className="space-y-3 text-xs text-text-muted leading-relaxed">
              <p>
                {t(
                  "account.deletionModalNotice",
                  "Requesting deletion will immediately schedule your account and all associated data for permanent erasure across all system modules.",
                )}
              </p>
              <ul className="list-disc pl-4 space-y-1 text-text-muted">
                <li>
                  {t(
                    "account.deletionGracePeriodNotice",
                    "A 30-day grace period begins immediately.",
                  )}
                </li>
                <li>
                  {t(
                    "account.deletionSessionsNotice",
                    "All other active sessions and trusted devices will be signed out.",
                  )}
                </li>
                <li>
                  {t(
                    "account.deletionCancelNotice",
                    "You can cancel the deletion and restore your account at any time before the grace period ends.",
                  )}
                </li>
              </ul>
            </div>

            <div className="space-y-2">
              <Label
                htmlFor="confirm-delete"
                className="text-xs text-text-muted"
              >
                {t("account.typeDeleteToConfirm", "Type DELETE to confirm:")}
              </Label>
              <Input
                id="confirm-delete"
                value={confirmInput}
                onChange={(e) => setConfirmInput(e.target.value)}
                placeholder="DELETE"
              />
            </div>

            <div className="flex items-center justify-end gap-3 border-t border-border-subtle pt-4">
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setIsConfirmModalOpen(false);
                  setConfirmInput("");
                }}
                disabled={isDeleting}
              >
                {t("account.cancel", "Cancel")}
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={confirmInput !== "DELETE" || isDeleting}
                onClick={() => {
                  void handleRequestDeletion();
                }}
              >
                {isDeleting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t("account.schedulingDeletion", "Scheduling Deletion...")}
                  </>
                ) : (
                  t(
                    "account.confirmDeletionRequest",
                    "Confirm Deletion Request",
                  )
                )}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
