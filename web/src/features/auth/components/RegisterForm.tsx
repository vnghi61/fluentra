import * as React from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";

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
import { authApi, type Challenge } from "@/features/auth/api/authApi";
import { GoogleButton } from "@/features/auth/components/GoogleButton";
import {
  registerSchema,
  type RegisterFormData,
} from "@/features/auth/model/schemas";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export interface RegisterFormProps {
  onChallengeIssued: (challenge: Challenge, email: string) => void;
}

export function RegisterForm({
  onChallengeIssued,
}: RegisterFormProps): React.JSX.Element {
  const { t } = useTranslation();
  const [serverError, setServerError] = React.useState<string | null>(null);

  const form = useForm<RegisterFormData>({
    resolver: (zodResolver as (schema: unknown) => never)(registerSchema),
    defaultValues: {
      email: "",
      display_name: "",
      password: "",
      locale: "en",
      timezone:
        typeof Intl !== "undefined"
          ? Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
          : "UTC",
    },
  });

  const onSubmit = async (data: RegisterFormData) => {
    setServerError(null);
    try {
      const challenge = await authApi.register({
        email: data.email,
        display_name: data.display_name,
        password: data.password,
        locale: data.locale,
        timezone: data.timezone,
      });
      // The server returns 201 Challenge for both fresh registrations and already-registered addresses (indistinguishable)
      onChallengeIssued(challenge, data.email);
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(getErrorMessage(err.problem));
      } else {
        setServerError(
          "Failed to create account. Please check your connection and try again.",
        );
      }
    }
  };

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-text">
          {t("auth.createAccountTitle", "Create your account")}
        </h1>
        <p className="text-sm text-text-muted">
          Start mastering all 6 English competencies today
        </p>
      </div>

      <GoogleButton onError={(msg) => setServerError(msg)} />

      <div className="relative flex items-center justify-center">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-border-subtle" />
        </div>
        <span className="relative bg-surface-card px-3 text-xs uppercase tracking-wider text-text-muted">
          {t("auth.orRegisterEmail", "Or register with email")}
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
            name="display_name"
            render={({ field }) => (
              <FormItem>
                <FormLabel required>
                  {t("auth.displayNameLabel", "Display Name")}
                </FormLabel>
                <FormControl>
                  <Input
                    type="text"
                    placeholder="Your name or nickname"
                    autoComplete="name"
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
                <FormLabel required>
                  {t("auth.passwordLabel", "Password")}
                </FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="At least 12 characters"
                    autoComplete="new-password"
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  Must be at least 12 characters and not easily guessable.
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
            {t("auth.createAccount", "Create account")}
          </Button>
        </form>
      </Form>

      <p className="text-center text-sm text-text-muted">
        Already have an account?{" "}
        <Link
          to="/login"
          className="font-medium text-primary-accent hover:text-primary-accent hover:underline"
        >
          {t("auth.signIn", "Sign in")}
        </Link>
      </p>
    </div>
  );
}
