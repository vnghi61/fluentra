import * as React from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";
import { AlertCircle, ArrowRight, Loader2 } from "lucide-react";

import { authApi } from "@/features/auth";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export function OAuthCallbackPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [errorCode, setErrorCode] = React.useState<string | null>(null);

  const calledRef = React.useRef(false);

  React.useEffect(() => {
    async function processCallback() {
      if (calledRef.current) return;
      calledRef.current = true;

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
        setError(
          t(
            "page.missingAuthenticationParametersInCallback",
            "Missing authentication parameters in callback URL.",
          ),
        );
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
            // Fallback in case window.close() is prevented by browser policy
            setTimeout(() => {
              void navigate({ to: "/" });
            }, 600);
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
          setError(
            t(
              "page.failedToCompleteGoogleAuthentication",
              "Failed to complete Google authentication. Please try again.",
            ),
          );
        }
      }
    }

    void processCallback();
  }, [navigate, t]);

  if (isLoading) {
    return (
      <div className="flex min-h-[calc(100vh-4rem)] flex-col items-center justify-center space-y-4 px-4">
        <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
        <p className="text-sm font-medium text-text-muted">
          {t("page.authenticatingWithGoogle", "Authenticating with Google...")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
      <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-border-subtle bg-surface-card/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-danger/10 text-danger-accent">
          <AlertCircle className="h-6 w-6" />
        </div>

        <div className="space-y-2">
          <h1 className="text-xl font-bold tracking-tight text-text">
            {t("page.authenticationIssue", "Authentication Issue")}
          </h1>
          <p className="text-sm text-text-muted leading-relaxed">{error}</p>
        </div>

        {errorCode === "OAUTH_ACCOUNT_CONFLICT" && (
          <div className="rounded-lg border border-warning/30 bg-warning/10 p-3.5 text-left text-xs font-medium text-warning-accent space-y-2">
            <p>
              To protect your account, email ownership must be confirmed before
              connecting Google sign-in.
            </p>
            <Link
              to="/register"
              className="inline-flex items-center gap-1 text-primary-accent hover:text-primary-accent underline font-semibold"
            >
              {t("page.verifyYourEmailNow", "Verify your email now")}
              <ArrowRight className="h-3 w-3" />
            </Link>
          </div>
        )}

        <div className="pt-2">
          <Link
            to="/login"
            className="inline-flex h-11 w-full items-center justify-center rounded-lg bg-primary px-4 text-sm font-medium text-white transition-colors hover:bg-primary-hover"
          >
            {t("page.returnToSignIn", "Return to Sign in")}
          </Link>
        </div>
      </div>
    </div>
  );
}
