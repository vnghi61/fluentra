import * as React from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, ShieldAlert } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { OtpInput } from "@/components/ui/otp-input";
import { authApi, type PasswordChanged } from "@/features/auth/api/authApi";
import {
  resetPasswordSchema,
  type ResetPasswordFormData,
} from "@/features/auth/model/schemas";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export interface ResetPasswordFormProps {
  challengeId: string;
  email: string;
  onSuccess?: ((result: PasswordChanged) => void) | undefined;
}

export function ResetPasswordForm({
  challengeId,
  email,
  onSuccess,
}: ResetPasswordFormProps): React.JSX.Element {
  const { t } = useTranslation();
  const [serverError, setServerError] = React.useState<string | null>(null);
  const [successResult, setSuccessResult] =
    React.useState<PasswordChanged | null>(null);

  const form = useForm<ResetPasswordFormData>({
    resolver: (zodResolver as (schema: unknown) => never)(resetPasswordSchema),
    defaultValues: {
      code: "",
      password: "",
    },
  });

  const onSubmit = async (data: ResetPasswordFormData) => {
    setServerError(null);
    try {
      const result = await authApi.resetPassword({
        challenge_id: challengeId,
        code: data.code,
        password: data.password,
      });
      setSuccessResult(result);
      onSuccess?.(result);
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(getErrorMessage(err.problem));
      } else {
        setServerError(
          t(
            "auth.failedToResetPasswordPlease",
            "Failed to reset password. Please check your connection.",
          ),
        );
      }
    }
  };

  if (successResult) {
    return (
      <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-success/10 text-success-accent">
          <CheckCircle2 className="h-6 w-6" />
        </div>

        <div className="space-y-2">
          <h1 className="text-2xl font-bold tracking-tight text-text">
            {t("auth.resetDoneTitle", "Password reset complete")}
          </h1>
          <p className="text-sm text-text-muted">
            {t(
              "auth.resetDoneBody",
              "Your password has been successfully updated.",
            )}
          </p>
        </div>

        {successResult.sessions_revoked > 0 && (
          <div
            role="status"
            className="flex items-start gap-2.5 rounded-lg border border-warning/30 bg-warning/10 p-3.5 text-left text-xs font-medium text-warning-accent"
          >
            <ShieldAlert className="h-4 w-4 shrink-0 text-warning-accent mt-0.5" />
            <span>
              For your security, {successResult.sessions_revoked} active session
              {successResult.sessions_revoked > 1 ? "s were" : " was"} signed
              out across your devices.
            </span>
          </div>
        )}

        <Link
          to="/login"
          className="inline-flex h-11 w-full items-center justify-center rounded-lg bg-primary px-4 text-sm font-medium text-white transition-colors hover:bg-primary-hover"
        >
          {t("auth.signInNewPassword", "Sign in with new password")}
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-text">
          {t("auth.newPasswordTitle", "Create new password")}
        </h1>
        <p className="text-sm text-text-muted">
          Enter the 6-digit code sent to{" "}
          <span className="font-semibold text-text">{email}</span> and your new
          password
        </p>
      </div>

      {serverError && (
        <div
          role="alert"
          className="rounded-lg border border-danger/50 bg-danger/10 p-3 text-sm font-medium text-danger-accent"
        >
          {serverError}
        </div>
      )}

      <Form {...form}>
        <form
          onSubmit={(e) => void form.handleSubmit(onSubmit)(e)}
          className="space-y-5"
        >
          <FormField
            control={form.control}
            name="code"
            render={({ field }) => (
              <FormItem className="space-y-3">
                <FormLabel required className="justify-center">
                  6-Digit Reset Code
                </FormLabel>
                <FormControl>
                  <OtpInput
                    value={field.value}
                    onChange={field.onChange}
                    error={!!form.formState.errors.code}
                  />
                </FormControl>
                <FormMessage className="text-center" />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <FormLabel required>
                  {t("auth.newPasswordLabel", "New Password")}
                </FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t(
                      "auth.atLeast12Characters",
                      "At least 12 characters",
                    )}
                    autoComplete="new-password"
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  Must be at least 12 characters and different from previous
                  passwords.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button
            type="submit"
            isLoading={form.formState.isSubmitting}
            className="w-full"
          >
            {t("auth.resetPassword", "Reset password")}
          </Button>
        </form>
      </Form>
    </div>
  );
}
