import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  CheckCircle2,
  Download,
  FileArchive,
  FileSpreadsheet,
  FileText,
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
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
  const { t, i18n } = useTranslation();
  const [profile, setProfile] = useState<UserProfile>(initialProfile);
  const [reportFormat, setReportFormat] = useState<"xlsx" | "pdf" | null>(null);
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

  // The writers (and their libraries) load only when a button is pressed, so
  // neither jsPDF nor write-excel-file is part of the settings chunk.
  const handleDownloadReport = async (format: "xlsx" | "pdf") => {
    if (reportFormat) return;
    setReportFormat(format);
    setStatusMessage(null);
    try {
      if (format === "xlsx") {
        const { exportAccountXlsx } = await import("../export/reportXlsx");
        await exportAccountXlsx(t, i18n.language);
      } else {
        const { exportAccountPdf } = await import("../export/reportPdf");
        await exportAccountPdf(t, i18n.language);
      }
      setStatusMessage({
        type: "success",
        text: t("account.reportDownloaded", "Your report has been downloaded."),
      });
    } catch {
      setStatusMessage({
        type: "error",
        text: t(
          "account.reportFailed",
          "Could not create the report. Please try again.",
        ),
      });
    } finally {
      setReportFormat(null);
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
            <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden />
          ) : (
            <AlertCircle className="h-5 w-5 shrink-0" aria-hidden />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Learning report: statistics + tables, built in the browser */}
      <section className={`${settingsCard} space-y-4`}>
        <div className="space-y-1">
          <h3 className={settingsCardTitle}>
            <Download className="h-5 w-5 text-primary-accent" aria-hidden />
            {t("account.downloadReportTitle", "Download your learning report")}
          </h3>
          <p className={`${settingsCardDesc} max-w-2xl`}>
            {t(
              "account.downloadReportDesc",
              "A summary of your statistics followed by detailed tables: courses, skills, XP and streak, reviews, vocabulary decks and mock tests. Only figures stored in your account are included.",
            )}
          </p>
        </div>
        <div className="grid gap-2 sm:flex sm:flex-wrap">
          <Button
            type="button"
            onClick={() => {
              void handleDownloadReport("xlsx");
            }}
            disabled={reportFormat !== null}
            isLoading={reportFormat === "xlsx"}
            className="w-full sm:w-auto"
          >
            {reportFormat !== "xlsx" && (
              <FileSpreadsheet className="h-4 w-4" aria-hidden />
            )}
            {reportFormat === "xlsx"
              ? t("account.preparingReport", "Preparing report…")
              : t("account.downloadXlsx", "Download Excel (.xlsx)")}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              void handleDownloadReport("pdf");
            }}
            disabled={reportFormat !== null}
            isLoading={reportFormat === "pdf"}
            className="w-full sm:w-auto"
          >
            {reportFormat !== "pdf" && (
              <FileText className="h-4 w-4" aria-hidden />
            )}
            {reportFormat === "pdf"
              ? t("account.preparingReport", "Preparing report…")
              : t("account.downloadPdf", "Download PDF")}
          </Button>
        </div>
      </section>

      {/* Full archive (GDPR Article 20), prepared by the server and e-mailed */}
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
            <p className="text-sm text-text-muted">
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
            className="shrink-0 whitespace-nowrap rounded-xl px-4"
          >
            {isExporting ? (
              <>
                <Loader2
                  aria-hidden="true"
                  className="mr-2 h-4 w-4 animate-spin"
                />
                {t("account.requesting", "Requesting...")}
              </>
            ) : (
              <>
                <Download aria-hidden="true" className="mr-2 h-4 w-4" />
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
              <AlertTriangle
                aria-hidden="true"
                className="h-5 w-5 shrink-0 text-warning-accent"
              />
              <div className="space-y-1">
                <h4 className="text-sm font-semibold text-warning-accent">
                  {t(
                    "account.accountDeletionIsCurrentlyScheduled",
                    "Account deletion is currently scheduled",
                  )}
                </h4>
                <p className="text-sm text-warning-accent leading-relaxed">
                  {t(
                    "account.gracePeriodBody",
                    "Your account is in the 30-day grace period. All active sessions have been revoked, and all data across all modules will be completely purged when the grace period ends. You may cancel this deletion request at any time during this window to restore full access.",
                  )}
                </p>
                {deletionState?.execute_at && (
                  <p className="text-sm font-semibold text-warning-accent pt-1">
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
                    <Loader2
                      aria-hidden="true"
                      className="mr-2 h-4 w-4 animate-spin"
                    />
                    {t("account.cancelling", "Cancelling...")}
                  </>
                ) : (
                  <>
                    <Undo2 aria-hidden="true" className="mr-1.5 h-4 w-4" />
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
              <p className="text-sm text-text-muted">
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
              className="shrink-0 whitespace-nowrap rounded-xl px-4"
            >
              {t("account.deleteMyAccount", "Delete My Account")}
            </Button>
          </div>
        )}
      </section>

      {/* Deletion Confirmation Modal */}
      <Dialog
        open={isConfirmModalOpen}
        onOpenChange={(open) => {
          if (!open && !isDeleting) {
            setIsConfirmModalOpen(false);
            setConfirmInput("");
          }
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-3">
              <AlertTriangle
                className="h-6 w-6 shrink-0 text-danger-accent"
                aria-hidden="true"
              />
              {t("account.areYouAbsolutelySure", "Are you absolutely sure?")}
            </DialogTitle>
          </DialogHeader>

          <DialogBody className="space-y-5">
            <div className="space-y-3 text-sm text-text-muted leading-relaxed">
              <DialogDescription>
                {t(
                  "account.deletionModalNotice",
                  "Requesting deletion will immediately schedule your account and all associated data for permanent erasure across all system modules.",
                )}
              </DialogDescription>
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
              <Label htmlFor="confirm-delete" className="text-text-muted">
                {t("account.typeDeleteToConfirm", "Type DELETE to confirm:")}
              </Label>
              <Input
                id="confirm-delete"
                value={confirmInput}
                onChange={(e) => setConfirmInput(e.target.value)}
                placeholder="DELETE"
              />
            </div>
          </DialogBody>

          <DialogFooter>
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
                  <Loader2
                    aria-hidden="true"
                    className="mr-2 h-4 w-4 animate-spin"
                  />
                  {t("account.schedulingDeletion", "Scheduling Deletion...")}
                </>
              ) : (
                t("account.confirmDeletionRequest", "Confirm Deletion Request")
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};
