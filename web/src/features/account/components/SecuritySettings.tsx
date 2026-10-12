import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CheckCircle2,
  ChevronRight,
  Info,
  KeyRound,
  LogOut,
  Shield,
} from "lucide-react";
import { ChangePasswordModal } from "./ChangePasswordModal";
import { GoogleAccountLink } from "./GoogleAccountLink";
import { DeviceSessionsList } from "./DeviceSessionsList";
import { SecurityOverview } from "./SecurityOverview";
import { SettingsCardHeader } from "./SettingsCard";
import { useSessions } from "../api/accountApi";
import { useGoogleLinkStatus } from "../api/securityQueries";
import { securityStatus } from "../model/securityStatus";
import { Button } from "@/components/ui/button";
import { settingsAlert, settingsCard } from "./settingsStyles";

interface SecuritySettingsProps {
  onLoggedOut?: (() => void) | undefined;
}

/** Password row and the change-password action. No two-factor: the API has none. */
const PasswordCard: React.FC<{ onChangePassword: () => void }> = ({
  onChangePassword,
}) => {
  const { t } = useTranslation();
  const google = useGoogleLinkStatus();
  const status = google.data ? securityStatus(google.data) : null;

  return (
    <section className={`${settingsCard} flex flex-col gap-4`}>
      <SettingsCardHeader
        icon={Shield}
        title={t("account.passwordAuthentication", "Password & Authentication")}
        description={t(
          "account.ensureYourAccountIsProtectedWithAStrongUniquePassword",
          "Ensure your account is protected with a strong, unique password.",
        )}
      />
      <button
        type="button"
        onClick={onChangePassword}
        className="flex min-h-[56px] w-full cursor-pointer items-center gap-3 rounded-xl border border-border-subtle px-3 py-2.5 text-left transition-colors hover:bg-surface-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none"
      >
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-text"
          aria-hidden="true"
        >
          <KeyRound className="h-5 w-5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-text">
            {t("settings.security.passwordRow", "Password")}
          </span>
          <span className="block text-sm text-text-muted">
            {status === null
              ? t(
                  "settings.security.passwordRowHint",
                  "Changing it signs out your other devices",
                )
              : status.hasPassword
                ? t(
                    "settings.security.passwordRowSet",
                    "Set. Changing it signs out your other devices",
                  )
                : t(
                    "settings.security.passwordRowNone",
                    "Not set. You sign in with Google",
                  )}
          </span>
        </span>
        <ChevronRight
          className="h-4 w-4 shrink-0 text-text-muted"
          aria-hidden="true"
        />
      </button>
      <div className="mt-auto flex flex-wrap gap-2 border-t border-border-subtle pt-4">
        <Button
          type="button"
          variant="outline"
          onClick={onChangePassword}
          className="w-full sm:w-auto"
        >
          <KeyRound className="h-4 w-4" aria-hidden />
          {t("account.changePassword", "Change Password")}
        </Button>
      </div>
    </section>
  );
};

/**
 * In place of an "unusual session lock": the remote sign-out the API really
 * offers. It counts the other sessions and jumps to the device list.
 */
const RemoteSignOutCard: React.FC = () => {
  const { t } = useTranslation();
  const sessions = useSessions();
  const others = (sessions.data?.sessions ?? []).filter(
    (s) => !s.current,
  ).length;

  return (
    <section className={`${settingsCard} flex flex-col gap-4`}>
      <SettingsCardHeader
        icon={LogOut}
        title={t("settings.security.remoteTitle", "Remote sign-out")}
        description={t(
          "settings.security.remoteDesc",
          "Lost a phone or used a shared computer? Sign that device out from here.",
        )}
      />
      {sessions.data && (
        <p className="rounded-xl bg-surface-muted/60 px-3 py-2.5 text-sm font-medium text-text">
          {others > 0
            ? t("settings.security.remoteOthers", {
                count: others,
                defaultValue: `${others} other active ${others === 1 ? "session" : "sessions"}`,
              })
            : t(
                "settings.security.remoteNone",
                "Only this device is signed in",
              )}
        </p>
      )}
      <ul className="space-y-2 text-sm text-text-muted">
        {[
          t(
            "settings.security.remotePoint1",
            "A device you sign out cannot renew its sign-in and is signed out within minutes.",
          ),
          t(
            "settings.security.remotePoint2",
            "Changing your password also signs out every other device.",
          ),
        ].map((point) => (
          <li key={point} className="flex items-start gap-2">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            <span>{point}</span>
          </li>
        ))}
      </ul>
      <div className="mt-auto border-t border-border-subtle pt-4">
        <Button
          type="button"
          variant="outline"
          className="w-full sm:w-auto"
          onClick={() => {
            const target = document.getElementById("devices");
            target?.scrollIntoView?.({ block: "start" });
            // Move focus with the view, so a keyboard user lands there too.
            document.getElementById("devices-title")?.focus({
              preventScroll: true,
            });
          }}
        >
          {t("settings.security.reviewDevices", "Review devices")}
        </Button>
      </div>
    </section>
  );
};

export const SecuritySettings: React.FC<SecuritySettingsProps> = ({
  onLoggedOut,
}) => {
  const { t } = useTranslation();
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const handlePasswordSuccess = (revokedCount: number) => {
    setStatusMessage({
      type: "success",
      text: t("account.passwordUpdatedRevoked", {
        count: revokedCount,
        defaultValue: `Password updated successfully. ${revokedCount} other active session(s) were signed out.`,
      }),
    });
  };

  return (
    <div className="space-y-6">
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

      <SecurityOverview />

      {/* Three across where each keeps ~320 px, two on a tablet, one on a phone. */}
      <div className="@container">
        <div className="grid items-stretch gap-4 @2xl:grid-cols-2 @5xl:grid-cols-3">
          <PasswordCard onChangePassword={() => setIsPasswordModalOpen(true)} />
          <GoogleAccountLink />
          <div className="@2xl:col-span-2 @5xl:col-span-1 [&>section]:h-full">
            <RemoteSignOutCard />
          </div>
        </div>
      </div>

      <DeviceSessionsList onLoggedOut={onLoggedOut} />

      <ChangePasswordModal
        isOpen={isPasswordModalOpen}
        onClose={() => setIsPasswordModalOpen(false)}
        onSuccess={handlePasswordSuccess}
      />
    </div>
  );
};
