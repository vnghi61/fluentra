import React, { useState } from "react";
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
          : "Failed to change password. Please check your current password.",
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
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <h2
            id="change-password-title"
            className="text-lg font-semibold text-slate-100 flex items-center gap-2"
          >
            <KeyRound className="h-5 w-5 text-indigo-400" />
            Change Password
          </h2>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <div className="flex items-start gap-2.5 rounded-lg border border-indigo-500/30 bg-indigo-500/10 p-3 text-xs text-indigo-200">
          <ShieldCheck className="h-4 w-4 shrink-0 text-indigo-400 mt-0.5" />
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
            <Label htmlFor="current_password">Current Password</Label>
            <Input
              id="current_password"
              type="password"
              {...register("current_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.current_password}
            />
            {errors.current_password && (
              <p className="text-xs text-rose-400">
                {errors.current_password.message}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="new_password">New Password</Label>
            <Input
              id="new_password"
              type="password"
              {...register("new_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.new_password}
            />
            {errors.new_password && (
              <p className="text-xs text-rose-400">
                {errors.new_password.message}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="confirm_new_password">Confirm New Password</Label>
            <Input
              id="confirm_new_password"
              type="password"
              {...register("confirm_new_password")}
              placeholder="••••••••"
              aria-invalid={!!errors.confirm_new_password}
            />
            {errors.confirm_new_password && (
              <p className="text-xs text-rose-400">
                {errors.confirm_new_password.message}
              </p>
            )}
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-slate-800 pt-4 mt-6">
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Updating...
                </>
              ) : (
                "Update Password"
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
