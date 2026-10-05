import React, { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  History,
  Laptop,
  Loader2,
  LogOut,
  Smartphone,
} from "lucide-react";
import { accountApi, type SessionSummary } from "../api/accountApi";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatDateTime,
  isMobileLabel,
  settingsAlert,
  settingsCard,
  settingsCardDesc,
  settingsCardTitle,
} from "./settingsStyles";

// GET /auth/sessions is not paginated, so the list is cut on the client: a
// learner who signs in daily had a page thousands of pixels long.
const INITIAL_VISIBLE = 5;

interface SessionsListProps {
  onLoggedOut?: (() => void) | undefined;
}

export const SessionsList: React.FC<SessionsListProps> = ({
  onLoggedOut: _onLoggedOut,
}) => {
  const { t, i18n } = useTranslation();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    let isMounted = true;
    async function loadSessions() {
      try {
        const data = await accountApi.listSessions();
        if (isMounted) {
          setSessions(data.sessions);
          setIsLoading(false);
        }
      } catch (err: unknown) {
        if (isMounted) {
          setError(
            err instanceof Error
              ? err.message
              : t(
                  "account.failedToLoadActiveSessions",
                  "Failed to load active sessions.",
                ),
          );
          setIsLoading(false);
        }
      }
    }

    void loadSessions();
    return () => {
      isMounted = false;
    };
  }, [t]);

  // The current session first, then the most recently active.
  const ordered = useMemo(
    () =>
      [...sessions].sort((a, b) => {
        if (a.current !== b.current) return a.current ? -1 : 1;
        return (
          new Date(b.last_seen_at).getTime() -
          new Date(a.last_seen_at).getTime()
        );
      }),
    [sessions],
  );
  const visible = showAll ? ordered : ordered.slice(0, INITIAL_VISIBLE);
  const hiddenCount = ordered.length - visible.length;

  const handleRevoke = async (session: SessionSummary) => {
    if (session.current || revokingId) {
      return;
    }

    setRevokingId(session.id);
    setError(null);

    try {
      await accountApi.revokeSession(session.id);
      setSessions((prev) => prev.filter((s) => s.id !== session.id));
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : t("account.failedToRevokeSession", "Failed to revoke session."),
      );
    } finally {
      setRevokingId(null);
    }
  };

  return (
    <section className={`${settingsCard} space-y-3`}>
      <div className="space-y-1">
        <h3 className={settingsCardTitle}>
          <History className="h-5 w-5 text-primary-accent" aria-hidden />
          {t("account.activeSessions", "Active Sessions")}
          {!isLoading && sessions.length > 0 && (
            <span className="rounded-full bg-surface-muted px-2 py-0.5 text-xs font-semibold tabular-nums text-text-muted">
              {sessions.length}
            </span>
          )}
        </h3>
        <p className={settingsCardDesc}>
          {t(
            "account.activeSessionsDesc",
            "Where you are currently signed in. You can revoke any session to sign it out.",
          )}
        </p>
      </div>

      {error && (
        <div role="alert" className={settingsAlert("error")}>
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          <span>{error}</span>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-2" data-testid="sessions-skeleton">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-14 w-full rounded-xl" />
          ))}
        </div>
      ) : sessions.length === 0 ? (
        <p className="text-sm text-text-muted">
          {t("account.noActiveSessionsFound", "No active sessions found.")}
        </p>
      ) : (
        <>
          <ul className="divide-y divide-border-subtle rounded-xl border border-border-subtle">
            {visible.map((session) => {
              const Icon = isMobileLabel(session.device_label)
                ? Smartphone
                : Laptop;
              const label =
                session.device_label ||
                t("account.unknownDevice", "Unknown Device");
              return (
                <li
                  key={session.id}
                  className={`flex items-center gap-3 px-3 py-3 sm:px-4 ${
                    session.current ? "bg-primary/5" : ""
                  }`}
                >
                  <span
                    className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-text-muted"
                    aria-hidden="true"
                  >
                    <Icon className="h-4 w-4" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
                      <p className="truncate text-sm font-medium text-text">
                        {label}
                      </p>
                      {session.current && (
                        <span className="shrink-0 rounded-full border border-success/20 bg-success/10 px-2 py-0.5 text-[11px] font-semibold text-success-accent">
                          {t("account.currentDevice", "Current Device")}
                        </span>
                      )}
                    </div>
                    <p className="truncate text-xs text-text-muted">
                      {t("account.lastActive", "Last active")}{" "}
                      {formatDateTime(session.last_seen_at, i18n.language)}
                    </p>
                  </div>

                  {!session.current && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        void handleRevoke(session);
                      }}
                      disabled={revokingId !== null}
                      aria-label={`${t("account.revoke", "Revoke")}: ${label}`}
                      className="min-h-[44px] shrink-0 gap-1.5 whitespace-nowrap px-2.5 text-text-muted hover:bg-danger/10 hover:text-danger-accent"
                    >
                      {revokingId === session.id ? (
                        <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                      ) : (
                        <LogOut className="h-4 w-4" aria-hidden />
                      )}
                      <span className="hidden sm:inline">
                        {t("account.revoke", "Revoke")}
                      </span>
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>

          {(hiddenCount > 0 || showAll) && ordered.length > INITIAL_VISIBLE && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setShowAll((v) => !v)}
              className="min-h-[44px] px-2 font-semibold text-primary-accent"
            >
              {showAll
                ? t("account.showFewerSessions", "Show fewer")
                : t("account.showMoreSessions", {
                    count: hiddenCount,
                    defaultValue: `Show ${hiddenCount} more sessions`,
                  })}
            </Button>
          )}
        </>
      )}
    </section>
  );
};
