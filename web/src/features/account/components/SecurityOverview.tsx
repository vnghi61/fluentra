import React, { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CheckCircle2,
  KeyRound,
  MonitorSmartphone,
  ShieldAlert,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";

import { useSessions, useTrustedDevices } from "../api/accountApi";
import { useGoogleLinkStatus } from "../api/securityQueries";
import { groupDeviceSessions } from "../model/deviceSessions";
import { securityStatus } from "../model/securityStatus";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { IconTile } from "./SettingsCard";
import { GoogleGlyph } from "./GoogleAccountLink";

type ChipTone = "ok" | "attention" | "unknown";

const StatusChip: React.FC<{
  icon: LucideIcon | React.FC<{ className?: string }>;
  tone: ChipTone;
  title: string;
  detail: string;
}> = ({ icon: Icon, tone, title, detail }) => {
  const { t } = useTranslation();
  const StateIcon = tone === "ok" ? CheckCircle2 : AlertCircle;
  return (
    <li className="flex min-w-0 items-start gap-3 rounded-xl border border-border-subtle bg-surface-card/80 p-3">
      <span
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-text"
        aria-hidden="true"
      >
        <Icon className="h-5 w-5" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold leading-snug text-text">{title}</p>
        <p className="text-sm text-text-muted">{detail}</p>
      </div>
      {tone !== "unknown" && (
        <StateIcon
          className={cn(
            "mt-0.5 h-5 w-5 shrink-0",
            tone === "ok" ? "text-success-accent" : "text-warning-accent",
          )}
          aria-label={
            tone === "ok"
              ? t("settings.security.chipOk", "OK")
              : t("settings.security.chipAttention", "Needs attention")
          }
          role="img"
        />
      )}
    </li>
  );
};

/**
 * The security page's summary: one sentence about the account, and three
 * chips (password, sessions, Google), each derived from a real read. Nothing
 * here claims monitoring, strength checks or two-factor: the API has none.
 */
export const SecurityOverview: React.FC = () => {
  const { t } = useTranslation();
  const google = useGoogleLinkStatus();
  const sessions = useSessions();
  const devices = useTrustedDevices();

  const deviceCount = useMemo(
    () =>
      groupDeviceSessions(
        sessions.data?.sessions ?? [],
        devices.data?.devices ?? [],
      ).length,
    [sessions.data, devices.data],
  );
  const sessionCount = sessions.data?.sessions.length ?? 0;
  const status = google.data ? securityStatus(google.data) : null;
  const allSet = status !== null && status.suggestions.length === 0;

  return (
    <section
      aria-labelledby="security-overview-title"
      className="@container rounded-2xl border border-border-subtle bg-gradient-to-br from-primary/10 via-surface-card to-surface-card p-4 shadow-[var(--shadow-card)] sm:p-6"
    >
      <div className="flex flex-col gap-5 @4xl:flex-row @4xl:items-center">
        <div className="flex min-w-0 flex-1 items-start gap-4">
          <IconTile
            icon={status && !allSet ? ShieldAlert : ShieldCheck}
            tone={status && !allSet ? "warning" : "success"}
            size="lg"
          />
          <div className="min-w-0 space-y-1">
            <h2
              id="security-overview-title"
              className="text-lg font-semibold leading-snug text-text md:text-xl"
            >
              {status === null
                ? t("settings.security.statusTitle", "Account security")
                : allSet
                  ? t(
                      "settings.security.statusGood",
                      "All your sign-in protections are in place",
                    )
                  : t("settings.security.statusAttention", {
                      count: status.suggestions.length,
                      defaultValue: `Your account is protected, with ${status.suggestions.length} ${status.suggestions.length === 1 ? "suggestion" : "suggestions"}`,
                    })}
            </h2>
            <p className="text-sm text-text-muted">
              {status === null || allSet
                ? t(
                    "settings.security.statusGoodDesc",
                    "Review the devices signed in to your account from time to time, and sign out any you do not recognise.",
                  )
                : t(
                    "settings.security.statusAttentionDesc",
                    "Signing in works normally. Follow the suggestions to add a second way in.",
                  )}
            </p>
          </div>
        </div>

        <ul
          className="grid gap-2 @lg:grid-cols-3 @4xl:w-[min(62%,46rem)] @4xl:shrink-0"
          aria-label={t("settings.security.statusLabel", "Security status")}
        >
          {google.isPending ? (
            <li className="@lg:col-span-3">
              <Skeleton className="h-[60px] w-full rounded-xl" />
            </li>
          ) : status ? (
            <StatusChip
              icon={KeyRound}
              tone={status.hasPassword ? "ok" : "attention"}
              title={
                status.hasPassword
                  ? t("settings.security.passwordSet", "Password set")
                  : t("settings.security.passwordNone", "No password yet")
              }
              detail={
                status.hasPassword
                  ? t(
                      "settings.security.passwordSetDetail",
                      "Can be used to sign in",
                    )
                  : t(
                      "settings.security.passwordNoneDetail",
                      "You sign in with Google only",
                    )
              }
            />
          ) : (
            <StatusChip
              icon={KeyRound}
              tone="unknown"
              title={t("account.accountPassword", "Account Password")}
              detail={t("settings.security.unavailable", "Unavailable")}
            />
          )}

          {sessions.isPending ? (
            <li>
              <Skeleton className="h-[60px] w-full rounded-xl" />
            </li>
          ) : (
            <StatusChip
              icon={MonitorSmartphone}
              tone={sessions.error ? "unknown" : "ok"}
              title={
                sessions.error
                  ? t("account.activeSessions", "Active Sessions")
                  : t("settings.security.sessionsChip", {
                      count: sessionCount,
                      defaultValue: `${sessionCount} active ${sessionCount === 1 ? "session" : "sessions"}`,
                    })
              }
              detail={
                sessions.error
                  ? t("settings.security.unavailable", "Unavailable")
                  : t("settings.security.devicesChip", {
                      count: deviceCount,
                      defaultValue: `on ${deviceCount} ${deviceCount === 1 ? "device" : "devices"}`,
                    })
              }
            />
          )}

          {google.isPending ? null : status ? (
            <StatusChip
              icon={GoogleGlyph}
              tone={status.googleLinked ? "ok" : "attention"}
              title={
                status.googleLinked
                  ? t("settings.security.googleOn", "Google connected")
                  : t("settings.security.googleOff", "Google not connected")
              }
              detail={
                status.googleLinked
                  ? t(
                      "settings.security.googleOnDetail",
                      "One-tap sign-in is on",
                    )
                  : t(
                      "settings.security.googleOffDetail",
                      "Connect for quicker sign-in",
                    )
              }
            />
          ) : (
            <StatusChip
              icon={GoogleGlyph}
              tone="unknown"
              title="Google"
              detail={t("settings.security.unavailable", "Unavailable")}
            />
          )}
        </ul>
      </div>
    </section>
  );
};
