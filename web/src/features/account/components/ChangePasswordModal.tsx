import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { KeyRound, Loader2, ShieldCheck, AlertCircle } from "lucide-react";
import { accountApi } from "../api/accountApi";
import {
  changePasswordSchema,
  type ChangePasswordFormValues,
} from "../model/schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

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
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !isSubmitting) onClose();
      }}
    >
      <DialogContent className="max-w-md" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound
              aria-hidden="true"
              className="h-5 w-5 text-primary-accent"
            />
            {t("account.changePassword", "Change Password")}
          </DialogTitle>
        </DialogHeader>

        <DialogBody className="space-y-6">
          {error && (
            <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm leading-relaxed text-danger-accent">
              <AlertCircle
                aria-hidden="true"
                className="h-5 w-5 shrink-0 text-danger-accent"
              />
              <span>{error}</span>
            </div>
          )}

          <div className="flex items-start gap-2.5 rounded-lg border border-primary/30 bg-primary/10 p-3 text-sm leading-relaxed text-primary-accent">
            <ShieldCheck
              aria-hidden="true"
              className="h-5 w-5 shrink-0 text-primary-accent"
            />
            <span>
              {t(
                "account.changePasswordNotice",
                "Changing your password will keep this session active while revoking all other signed-in devices.",
              )}
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
                <p className="text-sm text-danger-accent">
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
                <p className="text-sm text-danger-accent">
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
                <p className="text-sm text-danger-accent">
                  {errors.confirm_new_password.message}
                </p>
              )}
            </div>

            <div className="flex flex-wrap items-center justify-end gap-3 border-t border-border-subtle pt-4 mt-6">
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
                    <Loader2
                      aria-hidden="true"
                      className="mr-2 h-4 w-4 animate-spin"
                    />
                    {t("account.updating", "Updating...")}
                  </>
                ) : (
                  t("account.updatePassword", "Update Password")
                )}
              </Button>
            </div>
          </form>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
};
