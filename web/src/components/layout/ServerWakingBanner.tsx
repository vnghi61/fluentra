import React from "react";
import { Loader2, PlugZap } from "lucide-react";
import { useTranslation } from "react-i18next";

export interface ServerWakingBannerProps {
  /** True while the host is being woken; false once it answers. */
  waking: boolean;
  /** True when the wake budget ran out without an answer. */
  unreachable: boolean;
}

/**
 * Says out loud that the server is starting.
 *
 * The API sleeps when idle, and its first request after that takes the length
 * of a cold boot. Without this the app showed skeletons for a minute, which
 * reads as broken rather than as waiting — and the visitor most likely to meet
 * it is the one who has not signed up yet.
 *
 * Deliberately a banner and not a blocking overlay: everything already on the
 * screen still works, and the pages that need no data are fully usable while
 * this is up.
 */
export const ServerWakingBanner: React.FC<ServerWakingBannerProps> = ({
  waking,
  unreachable,
}) => {
  const { t } = useTranslation();

  if (!waking && !unreachable) return null;

  if (unreachable) {
    return (
      <div
        role="status"
        className="flex items-center gap-2 border-b border-danger/30 bg-danger/10 px-4 py-2 text-xs text-danger-accent"
      >
        <PlugZap className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>
          {t(
            "server.unreachable",
            "The server is not responding. Reload the page to try again.",
          )}
        </span>
      </div>
    );
  }

  return (
    <div
      role="status"
      className="flex items-center gap-2 border-b border-border-subtle bg-surface-muted px-4 py-2 text-xs text-text-muted"
    >
      <Loader2 className="h-4 w-4 shrink-0 animate-spin" aria-hidden="true" />
      <span>
        {t(
          "server.waking",
          "Starting the server — this takes up to a minute after a quiet spell.",
        )}
      </span>
    </div>
  );
};
