import * as React from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { authApi } from "@/features/auth/api/authApi";
import { GoogleButton } from "@/features/auth/components/GoogleButton";
import { loginSchema, type LoginFormData } from "@/features/auth/model/schemas";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";
import { useAuthStore } from "@/stores/authStore";

export interface LoginFormProps {
  onSuccess?: (() => void) | undefined;
}

export function LoginForm({ onSuccess }: LoginFormProps): React.JSX.Element {
  const { t } = useTranslation();
  const [serverError, setServerError] = React.useState<string | null>(null);
  const deviceId = useAuthStore((s) => s.deviceId);

  const form = useForm<LoginFormData>({
    resolver: (zodResolver as (schema: unknown) => never)(loginSchema),
    defaultValues: {
      email: "",
      password: "",
      remember_device: true,
    },
  });

  const onSubmit = async (data: LoginFormData) => {
    setServerError(null);
    try {
      await authApi.login({
        email: data.email,
        password: data.password,
        remember_device: data.remember_device,
        device_id: deviceId,
      });
      onSuccess?.();
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(getErrorMessage(err.problem));
      } else {
        setServerError(
          "Failed to sign in. Please check your connection and try again.",
        );
      }
    }
  };

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-text">
          {t("auth.signInTitle", "Sign in to Fluentra")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "auth.signInSubtitle",
            "Continue your personalized English learning path",
          )}
        </p>
      </div>

      <GoogleButton onError={(msg) => setServerError(msg)} />

      <div className="relative flex items-center justify-center">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-border-subtle" />
        </div>
        <span className="relative bg-surface-card px-3 text-xs uppercase tracking-wider text-text-muted">
          {t("auth.orContinueEmail", "Or continue with email")}
        </span>
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

          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <div className="flex items-center justify-between">
                  <FormLabel required>
                    {t("auth.passwordLabel", "Password")}
                  </FormLabel>
                  <Link
                    to="/forgot-password"
                    // A standalone control, not a link inside a sentence, so
                    // R1's 44 px minimum applies to it. The negative margin
                    // keeps the row's visual rhythm while the hit area grows.
                    className="inline-flex min-h-11 items-center -my-3 text-xs font-medium text-primary-accent hover:text-primary-accent hover:underline"
                  >
                    {t("auth.forgotPassword", "Forgot password?")}
                  </Link>
                </div>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="Enter your password"
                    autoComplete="current-password"
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="remember_device"
            render={({ field }) => (
              <FormItem className="space-y-0">
                <FormControl>
                  <Checkbox
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    label="Stay signed in on this device"
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <Button
            type="submit"
            isLoading={form.formState.isSubmitting}
            className="w-full"
          >
            {t("auth.signIn", "Sign in")}
          </Button>
        </form>
      </Form>

      <p className="text-center text-sm text-text-muted">
        Don&apos;t have an account?{" "}
        <Link
          to="/register"
          className="font-medium text-primary-accent hover:text-primary-accent hover:underline"
        >
          {t("auth.createOne", "Create one now")}
        </Link>
      </p>
    </div>
  );
}
