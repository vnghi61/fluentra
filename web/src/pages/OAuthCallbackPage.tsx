import * as React from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertCircle, ArrowRight, Loader2 } from "lucide-react";

import { authApi } from "@/features/auth";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export function OAuthCallbackPage(): React.JSX.Element {
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [errorCode, setErrorCode] = React.useState<string | null>(null);

  React.useEffect(() => {
    async function processCallback() {
      const urlParams = new URLSearchParams(window.location.search);
      const code = urlParams.get("code");
      const state = urlParams.get("state");
      const errorParam = urlParams.get("error");

      if (errorParam) {
        setIsLoading(false);
        setError(`Google authorization was denied or cancelled: ${errorParam}`);
        return;
      }

      if (!code || !state) {
        setIsLoading(false);
        setError("Missing authentication parameters in callback URL.");
        return;
      }

      try {
        const session = await authApi.googleCallback({ code, state });

        // If running inside an OAuth popup window, notify the opener
        if (window.opener && window.opener !== window) {
          try {
            const openerWindow = window.opener as Window;
            openerWindow.postMessage(
              { type: "GOOGLE_AUTH_SUCCESS", session },
              window.location.origin,
            );
            window.close();
            return;
          } catch {
            // If postMessage fails, proceed with standard navigation
          }
        }

        void navigate({ to: "/" });
      } catch (err) {
        setIsLoading(false);
        if (err instanceof ApiError) {
          setError(getErrorMessage(err.problem));
          setErrorCode(err.problem.code ?? null);
        } else {
          setError("Failed to complete Google authentication. Please try again.");
        }
      }
    }

    void processCallback();
  }, [navigate]);

  if (isLoading) {
    return (
      <div className="flex min-h-[calc(100vh-4rem)] flex-col items-center justify-center space-y-4 px-4">
        <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
        <p className="text-sm font-medium text-slate-300">
          Authenticating with Google...
        </p>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
      <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-rose-500/10 text-rose-400">
          <AlertCircle className="h-6 w-6" />
        </div>

        <div className="space-y-2">
          <h1 className="text-xl font-bold tracking-tight text-slate-100">
            Authentication Issue
          </h1>
          <p className="text-sm text-slate-300 leading-relaxed">
            {error}
          </p>
        </div>

        {errorCode === "OAUTH_ACCOUNT_CONFLICT" && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3.5 text-left text-xs font-medium text-amber-200 space-y-2">
            <p>
              To protect your account, email ownership must be confirmed before connecting Google sign-in.
            </p>
            <Link
              to="/register"
              className="inline-flex items-center gap-1 text-indigo-400 hover:text-indigo-300 underline font-semibold"
            >
              Verify your email now <ArrowRight className="h-3 w-3" />
            </Link>
          </div>
        )}

        <div className="pt-2">
          <Link
            to="/login"
            className="inline-flex h-11 w-full items-center justify-center rounded-lg bg-indigo-600 px-4 text-sm font-medium text-white transition-colors hover:bg-indigo-700"
          >
            Return to Sign in
          </Link>
        </div>
      </div>
    </div>
  );
}
