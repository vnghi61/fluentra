import React, { useState } from "react";
import {
  CheckCircle2,
  Link2,
  Loader2,
  Unlink,
  AlertCircle,
} from "lucide-react";
import { accountApi } from "../api/accountApi";
import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";

interface GoogleAccountLinkProps {
  initialLinkedEmail?: string | null;
}

export const GoogleAccountLink: React.FC<GoogleAccountLinkProps> = ({
  initialLinkedEmail,
}) => {
  const [linkedEmail, setLinkedEmail] = useState<string | null>(
    initialLinkedEmail || null,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleStartLink = async () => {
    setIsLoading(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await accountApi.startGoogleLink();
      window.location.href = response.authorization_url;
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to start Google linking flow.",
      );
      setIsLoading(false);
    }
  };

  const handleUnlink = async () => {
    if (!confirm("Are you sure you want to disconnect your Google account?")) {
      return;
    }

    setIsLoading(true);
    setError(null);
    setSuccess(null);

    try {
      await accountApi.unlinkGoogle();
      setLinkedEmail(null);
      setSuccess("Google account unlinked successfully.");
    } catch (err: unknown) {
      if (
        err instanceof ApiError &&
        err.problem.code === "LAST_SIGN_IN_METHOD"
      ) {
        setError(
          "Cannot unlink Google account: it is your only sign-in method. Set a password first before unlinking.",
        );
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to unlink Google account.",
        );
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-6 space-y-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
            <Link2 className="h-5 w-5 text-indigo-400" />
            Connected Accounts
          </h3>
          <p className="text-xs text-slate-400">
            Link your Google account for quick, one-tap sign in.
          </p>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3.5 text-xs text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {success && (
        <div className="flex items-start gap-2.5 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3.5 text-xs text-emerald-300">
          <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-400 mt-0.5" />
          <span>{success}</span>
        </div>
      )}

      <div className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-900/40 p-4">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-slate-800 text-slate-200">
            <svg className="h-5 w-5" viewBox="0 0 24 24">
              <path
                fill="currentColor"
                d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
              />
              <path
                fill="currentColor"
                d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
              />
              <path
                fill="currentColor"
                d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
              />
              <path
                fill="currentColor"
                d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
              />
            </svg>
          </div>
          <div>
            <p className="text-sm font-medium text-slate-200">Google</p>
            <p className="text-xs text-slate-400">
              {linkedEmail ? `Connected as ${linkedEmail}` : "Not connected"}
            </p>
          </div>
        </div>

        {linkedEmail ? (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            onClick={() => {
              void handleUnlink();
            }}
            disabled={isLoading}
          >
            {isLoading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <Unlink className="mr-1.5 h-3.5 w-3.5" />
                Unlink
              </>
            )}
          </Button>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              void handleStartLink();
            }}
            disabled={isLoading}
          >
            {isLoading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              "Connect Google"
            )}
          </Button>
        )}
      </div>
    </div>
  );
};
