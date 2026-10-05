import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, CheckCircle2, KeyRound, Shield } from "lucide-react";
import { ChangePasswordModal } from "./ChangePasswordModal";
import { GoogleAccountLink } from "./GoogleAccountLink";
import { DevicesList } from "./DevicesList";
import { SessionsList } from "./SessionsList";
import { Button } from "@/components/ui/button";
import {
  settingsAlert,
  settingsCard,
  settingsCardDesc,
  settingsCardTitle,
} from "./settingsStyles";

interface SecuritySettingsProps {
  onLoggedOut?: (() => void) | undefined;
}

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

      {/* Password: one compact row. No MFA block: the backend has none. */}
      <section className={settingsCard}>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="min-w-0 space-y-1">
            <h3 className={settingsCardTitle}>
              <Shield className="h-5 w-5 text-primary-accent" aria-hidden />
              {t("account.passwordAuthentication", "Password & Authentication")}
            </h3>
            <p className={settingsCardDesc}>
              {t(
                "account.ensureYourAccountIsProtectedWithAStrongUniquePassword",
                "Ensure your account is protected with a strong, unique password.",
              )}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setIsPasswordModalOpen(true)}
            className="min-h-[44px] shrink-0 self-start whitespace-nowrap rounded-xl px-4 sm:min-h-10 sm:self-auto"
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" aria-hidden />
            {t("account.changePassword", "Change Password")}
          </Button>
        </div>
      </section>

      <GoogleAccountLink />

      <SessionsList onLoggedOut={onLoggedOut} />

      <DevicesList onLoggedOut={onLoggedOut} />

      <ChangePasswordModal
        isOpen={isPasswordModalOpen}
        onClose={() => setIsPasswordModalOpen(false)}
        onSuccess={handlePasswordSuccess}
      />
    </div>
  );
};
