import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { AlertCircle, Check, CheckCircle2, Link2, Unlink } from "lucide-react";
import { accountApi } from "../api/accountApi";
import { accountKeys } from "../api/keys";
import { useGoogleLinkStatus } from "../api/securityQueries";
import { ApiError } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { SettingsCardHeader } from "./SettingsCard";
import { formatDate, settingsAlert, settingsCard } from "./settingsStyles";

/** Google's "G", drawn in the current text colour so it follows the theme. */
export const GoogleGlyph: React.FC<{ className?: string }> = ({
  className,
}) => (
  <svg className={className} viewBox="0 0 24 24" aria-hidden="true">
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
);

/** "Social sign-in": link or unlink Google, with what linking is good for. */
export const GoogleAccountLink: React.FC<{ className?: string }> = ({
  className,
}) => {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const statusQuery = useGoogleLinkStatus();
  const status = statusQuery.data;
  const [isLoading, setIsLoading] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const linked = status?.linked ?? false;
  const canUnlink = status?.can_unlink ?? false;
  const readError =
    statusQuery.error &&
    (statusQuery.error instanceof Error && statusQuery.error.message
      ? statusQuery.error.message
      : t(
          "account.couldNotReadYourGoogle",
          "Could not read your Google link.",
        ));

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
          : t(
              "account.failedToStartGoogleLinking",
              "Failed to start Google linking flow.",
            ),
      );
      setIsLoading(false);
    }
  };

  const handleUnlink = async () => {
    // The UI prevents the common case; the server prevents the crafted one. The
    // button is disabled when `can_unlink` is false, and this guard means a
    // programmatic call cannot slip past the disabled attribute either.
    if (!canUnlink) return;

    setIsLoading(true);
    setError(null);
    setSuccess(null);

    try {
      await accountApi.unlinkGoogle();
      await queryClient.invalidateQueries({
        queryKey: accountKeys.googleLink(),
      });
      setSuccess(
        t(
          "account.googleAccountUnlinkedSuccessfully",
          "Google account unlinked successfully.",
        ),
      );
    } catch (err: unknown) {
      if (
        err instanceof ApiError &&
        err.problem.code === "LAST_SIGN_IN_METHOD"
      ) {
        setError(
          t(
            "account.cannotUnlinkLastMethod",
            "Cannot unlink Google account: it is your only sign-in method. Set a password first before unlinking.",
          ),
        );
      } else {
        setError(
          err instanceof Error
            ? err.message
            : t(
                "account.failedToUnlinkGoogleAccount",
                "Failed to unlink Google account.",
              ),
        );
      }
    } finally {
      setConfirmOpen(false);
      setIsLoading(false);
    }
  };

  const benefits = [
    t(
      "settings.security.benefitOneTap",
      "Sign in with one tap, no password to remember",
    ),
    t(
      "settings.security.benefitBackup",
      "A second way in if you forget your password",
    ),
    t(
      "settings.security.benefitNoPassword",
      "Your Google password is never shared with Fluentra",
    ),
  ];

  return (
    <section
      className={`${settingsCard} flex flex-col gap-4 ${className ?? ""}`}
    >
      <SettingsCardHeader
        icon={Link2}
        title={t("settings.security.socialTitle", "Social sign-in")}
        description={t(
          "account.linkYourGoogleAccountForQuickOneTapSignIn",
          "Link your Google account for quick, one-tap sign in.",
        )}
      />

      {(error ?? readError) && (
        <div role="alert" className={settingsAlert("error")}>
          <AlertCircle className="h-5 w-5 shrink-0" aria-hidden />
          <span>{error ?? readError}</span>
        </div>
      )}

      {success && (
        <div role="status" className={settingsAlert("success")}>
          <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden />
          <span>{success}</span>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border-subtle p-3">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <span
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-surface-muted text-text"
            aria-hidden="true"
          >
            <GoogleGlyph className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-text">
              Google
              {status && (
                <Badge variant={linked ? "success" : "secondary"}>
                  {linked
                    ? t("account.connected", "Connected")
                    : t("account.notConnected", "Not connected")}
                </Badge>
              )}
            </p>
            {statusQuery.isPending ? (
              <Skeleton className="mt-1 h-3 w-24" />
            ) : (
              linked &&
              status?.linked_at && (
                <p className="truncate text-sm text-text-muted">
                  {t("account.connectedSince", {
                    date: formatDate(status.linked_at, i18n.language),
                    defaultValue: `Connected since ${formatDate(status.linked_at, i18n.language)}`,
                  })}
                </p>
              )
            )}
          </div>
        </div>

        {linked ? (
          <Button
            type="button"
            variant="outline"
            className="w-full shrink-0 whitespace-nowrap hover:border-danger/40 hover:bg-danger/10 hover:text-danger-accent sm:w-auto"
            onClick={() => setConfirmOpen(true)}
            disabled={isLoading || !canUnlink}
            title={
              canUnlink
                ? undefined
                : t(
                    "account.googleIsYourOnlySign",
                    "Google is your only sign-in method. Set a password before unlinking.",
                  )
            }
          >
            <Unlink className="h-4 w-4" aria-hidden />
            {t("account.unlink", "Unlink")}
          </Button>
        ) : (
          <Button
            type="button"
            variant="primary"
            className="w-full shrink-0 whitespace-nowrap sm:w-auto"
            onClick={() => {
              void handleStartLink();
            }}
            disabled={isLoading || statusQuery.isPending || !status}
            isLoading={isLoading}
          >
            {t("account.connectGoogle", "Connect Google")}
          </Button>
        )}
      </div>

      {linked && !canUnlink && (
        <p className="text-sm text-text-muted">
          {t(
            "account.googleIsYourOnlySign",
            "Google is your only sign-in method. Set a password before unlinking.",
          )}
        </p>
      )}

      <ul className="space-y-2">
        {benefits.map((benefit) => (
          <li
            key={benefit}
            className="flex items-start gap-2 text-sm text-text"
          >
            <Check
              className="mt-0.5 h-4 w-4 shrink-0 text-success-accent"
              aria-hidden
            />
            <span>{benefit}</span>
          </li>
        ))}
      </ul>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => {
          if (!open && !isLoading) setConfirmOpen(false);
        }}
        destructive
        keepOpenOnConfirm
        isPending={isLoading}
        title={t(
          "account.confirmUnlinkGoogle",
          "Are you sure you want to disconnect your Google account?",
        )}
        description={t(
          "settings.security.unlinkDesc",
          "You can still sign in with your password, and connect Google again at any time.",
        )}
        confirmLabel={t("account.unlink", "Unlink")}
        onConfirm={() => {
          void handleUnlink();
        }}
      />
    </section>
  );
};
