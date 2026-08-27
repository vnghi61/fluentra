import React, { useState } from "react";
import { CheckCircle2, KeyRound, Shield, AlertCircle } from "lucide-react";
import { ChangePasswordModal } from "./ChangePasswordModal";
import { GoogleAccountLink } from "./GoogleAccountLink";
import { DevicesList } from "./DevicesList";
import { SessionsList } from "./SessionsList";
import { Button } from "@/components/ui/button";

interface SecuritySettingsProps {
  onLoggedOut?: (() => void) | undefined;
}

export const SecuritySettings: React.FC<SecuritySettingsProps> = ({
  onLoggedOut,
}) => {
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const handlePasswordSuccess = (revokedCount: number) => {
    setStatusMessage({
      type: "success",
      text: `Password updated successfully. ${revokedCount} other active session(s) were signed out.`,
    });
  };

  return (
    <div className="space-y-8">
      {statusMessage && (
        <div
          role="status"
          className={`flex items-start gap-2.5 rounded-lg p-3.5 text-xs ${
            statusMessage.type === "success"
              ? "border border-success/30 bg-success/10 text-success-accent"
              : "border border-danger/30 bg-danger/10 text-danger-accent"
          }`}
        >
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="h-4 w-4 shrink-0 text-success-accent mt-0.5" />
          ) : (
            <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Password Management */}
      <div className="rounded-xl border border-border-subtle bg-surface-card/60 p-6 space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-text flex items-center gap-2">
              <Shield className="h-5 w-5 text-primary-accent" />
              Password & Authentication
            </h3>
            <p className="text-xs text-text-muted">
              Ensure your account is protected with a strong, unique password.
            </p>
          </div>
        </div>

        {/* Wraps at 320 px: the label and a 89 px button do not fit on one
            line there, and without the wrap the button pushed the page 5 px
            wider than the viewport (R6). */}
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-subtle bg-surface-card/40 p-4">
          <div className="min-w-0">
            <p className="text-sm font-medium text-text">
              Account Password
            </p>
            <p className="text-xs text-text-muted">••••••••••••••••</p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setIsPasswordModalOpen(true)}
          >
            <KeyRound className="mr-1.5 h-3.5 w-3.5" />
            Change Password
          </Button>
        </div>
      </div>

      {/* Connected Google Account */}
      <GoogleAccountLink />

      {/* Trusted Devices */}
      <DevicesList onLoggedOut={onLoggedOut} />

      {/* Active Sessions */}
      <SessionsList onLoggedOut={onLoggedOut} />

      <ChangePasswordModal
        isOpen={isPasswordModalOpen}
        onClose={() => setIsPasswordModalOpen(false)}
        onSuccess={handlePasswordSuccess}
      />
    </div>
  );
};
