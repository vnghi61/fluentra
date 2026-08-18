import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
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
        setServerError("Failed to sign in. Please check your connection and try again.");
      }
    }
  };

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-slate-100">
          Sign in to Fluentra
        </h1>
        <p className="text-sm text-slate-400">
          Continue your personalized English learning path
        </p>
      </div>

      <GoogleButton onError={(msg) => setServerError(msg)} />

      <div className="relative flex items-center justify-center">
        <div className="w-full border-t border-slate-800" />
        <span className="bg-slate-900 px-3 text-xs uppercase tracking-wider text-slate-500">
          Or continue with email
        </span>
      </div>

      {serverError && (
        <div
          role="alert"
          className="rounded-lg border border-rose-500/50 bg-rose-500/10 p-3 text-sm font-medium text-rose-300"
        >
          {serverError}
        </div>
      )}

      <Form {...form}>
        <form onSubmit={(e) => void form.handleSubmit(onSubmit)(e)} className="space-y-4">
          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel required>Email address</FormLabel>
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
                  <FormLabel required>Password</FormLabel>
                  <Link
                    to="/forgot-password"
                    className="text-xs font-medium text-indigo-400 hover:text-indigo-300 hover:underline"
                  >
                    Forgot password?
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
            Sign in
          </Button>
        </form>
      </Form>

      <p className="text-center text-sm text-slate-400">
        Don&apos;t have an account?{" "}
        <Link
          to="/register"
          className="font-medium text-indigo-400 hover:text-indigo-300 hover:underline"
        >
          Create one now
        </Link>
      </p>
    </div>
  );
}
