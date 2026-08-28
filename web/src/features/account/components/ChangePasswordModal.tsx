import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { KeyRound, Loader2, ShieldCheck, X, AlertCircle } from "lucide-react";
import { accountApi } from "../api/accountApi";
import {
  changePasswordSchema,
  type ChangePasswordFormValues,
} from "../model/schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ChangePasswordModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (revokedSessionsCount: number) => void;
}

export const ChangePasswordModal: React.FC<ChangePasswordModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<ChangePasswordFormValues>({
    resolver: (zodResolver as (schema: unknown) => never)(changePasswordSchema),
    defaultValues: {
      current_password: "",
      new_password: "",
      confirm_new_password: "",
    },
  });

  if (!isOpen) return null;

  const onSubmit = async (values: ChangePasswordFormValues) => {
    setIsSubmitting(true);
    setError(null);

    try {
      const result = await accountApi.changePassword({
        current_password: values.current_password,
        new_password: values.new_password,
      });

      reset();
      onSuccess(result.sessions_revoked);
      onClose();
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : t(
              "account.failedToChangePasswordPlease",
              "Failed to change password. Please check your current password.",
            ),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="change-password-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-border-subtle pb-4">
          <h2
            id="change-password-title"
            className="text-lg font-semibold text-text flex items-center gap-2"
          >
            <KeyRound className="h-5 w-5 text-primary-accent" />
            {t("account.changePassword", "Change Password")}
          </h2>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded-lg p-1 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3 text-xs text-danger-accent">
            <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <div className="flex items-start gap-2.5 rounded-lg border border-primary/30 bg-primary/10 p-3 text-xs text-primary-accent">
          <ShieldCheck className="h-4 w-4 shrink-0 text-primary-accent mt-0.5" />
          <span>
            Changing your password will keep this session active while revoking
            all other signed-in devices.
          </span>
        </div>

        <form
          onSubmit={(e) => {
            void handleSubmit(onSubmit)(e);
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="current_password">
              {t("account.currentPassword", "Current Password")}
            </Label>
            <Input
              id="current_password"
              type="password"
              {...register("current_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.current_password}
            />
            {errors.current_password && (
              <p className="text-xs text-danger-accent">
                {errors.current_password.message}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="new_password">
              {t("account.newPassword", "New Password")}
            </Label>
            <Input
              id="new_password"
              type="password"
              {...register("new_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.new_password}
            />
            {errors.new_password && (
              <p className="text-xs text-danger-accent">
                {errors.new_password.message}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="confirm_new_password">
              {t("account.confirmNewPassword", "Confirm New Password")}
            </Label>
            <Input
              id="confirm_new_password"
              type="password"
              {...register("confirm_new_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.confirm_new_password}
            />
            {errors.confirm_new_password && (
              <p className="text-xs text-danger-accent">
                {errors.confirm_new_password.message}
              </p>
            )}
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-border-subtle pt-4 mt-6">
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              disabled={isSubmitting}
            >
              {t("account.cancel", "Cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t("account.updating", "Updating...")}
                </>
              ) : (
                t("account.updatePassword", "Update Password")
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
