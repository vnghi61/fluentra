import * as React from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { authApi, type Challenge } from "@/features/auth/api/authApi";
import {
  forgotPasswordSchema,
  type ForgotPasswordFormData,
} from "@/features/auth/model/schemas";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export interface ForgotPasswordFormProps {
  onChallengeIssued: (challenge: Challenge, email: string) => void;
}

export function ForgotPasswordForm({
  onChallengeIssued,
}: ForgotPasswordFormProps): React.JSX.Element {
  const { t } = useTranslation();
  const [serverError, setServerError] = React.useState<string | null>(null);

  const form = useForm<ForgotPasswordFormData>({
    resolver: (zodResolver as (schema: unknown) => never)(forgotPasswordSchema),
    defaultValues: {
      email: "",
    },
  });

  const onSubmit = async (data: ForgotPasswordFormData) => {
    setServerError(null);
    try {
      // 202 Accepted response returned uniformly for both existing and non-existing emails (zero enumeration)
      const challenge = await authApi.forgotPassword({ email: data.email });
      onChallengeIssued(challenge, data.email);
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(getErrorMessage(err.problem));
      } else {
        setServerError(
          t(
            "auth.failedToRequestResetPlease",
            "Failed to request reset. Please try again.",
          ),
        );
      }
    }
  };

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      <Link
        to="/login"
        className="inline-flex items-center gap-1.5 text-xs font-medium text-text-muted hover:text-text transition-colors"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        {t("auth.backToSignIn", "Back to sign in")}
      </Link>

      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-text">
          {t("auth.resetTitle", "Reset your password")}
        </h1>
        <p className="text-sm text-text-muted">
          Enter your account email to receive a 6-digit recovery code
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
          className="space-y-4"
        >
          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel required>
                  {t("auth.emailLabel", "Email address")}
                </FormLabel>
                <FormControl>
                  <Input
                    type="email"
                    placeholder="learner@example.com"
                    autoComplete="email"
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button
            type="submit"
            isLoading={form.formState.isSubmitting}
            className="w-full"
          >
            {t("auth.sendRecoveryCode", "Send recovery code")}
          </Button>
        </form>
      </Form>
    </div>
  );
}
