import React, { useEffect, useState } from "react";
import {
  History,
  Laptop,
  Loader2,
  LogOut,
  Smartphone,
  AlertCircle,
} from "lucide-react";
import { accountApi, type SessionSummary } from "../api/accountApi";
import { Button } from "@/components/ui/button";

interface SessionsListProps {
  onLoggedOut?: (() => void) | undefined;
}

export const SessionsList: React.FC<SessionsListProps> = ({ onLoggedOut }) => {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

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
              : "Failed to load active sessions.",
          );
          setIsLoading(false);
        }
      }
    }

    void loadSessions();
    return () => {
      isMounted = false;
    };
  }, []);

  const handleRevoke = async (session: SessionSummary) => {
    if (
      session.current &&
      !confirm(
        "Revoking your current session will sign you out immediately. Proceed?",
      )
    ) {
      return;
    }

    setRevokingId(session.id);
    setError(null);

    try {
      await accountApi.revokeSession(session.id);
      if (session.current) {
        onLoggedOut?.();
      } else {
        setSessions((prev) => prev.filter((s) => s.id !== session.id));
      }
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to revoke session.",
      );
    } finally {
      setRevokingId(null);
    }
  };

  return (
    <div className="rounded-xl border border-border-subtle bg-surface-card/60 p-6 space-y-4">
      <div className="space-y-1">
        <h3 className="text-base font-semibold text-text flex items-center gap-2">
          <History className="h-5 w-5 text-primary-accent" />
          Active Sessions
        </h3>
        <p className="text-xs text-text-muted">
          Where you are currently signed in. You can revoke any session to sign
          it out.
        </p>
      </div>

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3.5 text-xs text-danger-accent">
          <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-primary-accent" />
        </div>
      ) : sessions.length === 0 ? (
        <p className="text-xs text-text-muted py-4 text-center">
          No active sessions found.
        </p>
      ) : (
        <div className="space-y-3">
          {sessions.map((session) => {
            const isMobile =
              session.device_label?.toLowerCase().includes("mobile") ||
              session.device_label?.toLowerCase().includes("android") ||
              session.device_label?.toLowerCase().includes("iphone");

            return (
              <div
                key={session.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-subtle bg-surface-card/40 p-4"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-muted text-text-muted">
                    {isMobile ? (
                      <Smartphone className="h-5 w-5" />
                    ) : (
                      <Laptop className="h-5 w-5" />
                    )}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium text-text">
                        {session.device_label || "Unknown Device"}
                      </p>
                      {session.current && (
                        <span className="rounded-full bg-success/10 px-2 py-0.5 text-xs font-semibold text-success-accent border border-success/20">
                          Current Device
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-text-muted mt-0.5">
                      <span>
                        Signed in:{" "}
                        {new Date(session.created_at).toLocaleDateString()}
                      </span>
                      <span>
                        Last active:{" "}
                        {new Date(session.last_seen_at).toLocaleTimeString([], {
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                    </div>
                  </div>
                </div>

                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    void handleRevoke(session);
                  }}
                  disabled={revokingId === session.id}
                  className="text-text-muted hover:text-danger-accent hover:bg-danger/10"
                >
                  {revokingId === session.id ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <>
                      <LogOut className="mr-1.5 h-3.5 w-3.5" />
                      Revoke
                    </>
                  )}
                </Button>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
