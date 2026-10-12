import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Copy } from "lucide-react";

/**
 * A truncated identifier with a copy button.
 *
 * Names are what an administrator reads; the id is what they paste into a
 * ticket or a log search. So it is shown short, the full value is on the title
 * and the accessible name, and copying is one tap at the 44 px floor.
 */
export function CopyableId({
  id,
  compact = false,
}: {
  id: string;
  compact?: boolean;
}): React.JSX.Element {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  const copy = () => {
    // Absent in insecure contexts and some embedded webviews: do nothing
    // rather than throw from a click handler.
    if (typeof navigator === "undefined" || !navigator.clipboard) return;
    void navigator.clipboard
      .writeText(id)
      .then(() => {
        setCopied(true);
        clearTimeout(timer.current);
        timer.current = setTimeout(() => setCopied(false), 1500);
      })
      .catch(() => undefined);
  };

  return (
    <span className="inline-flex min-w-0 items-center gap-0.5">
      {!compact && (
        <span title={id} className="truncate font-mono text-xs text-text-muted">
          {id.slice(0, 8)}…
        </span>
      )}
      <button
        type="button"
        onClick={copy}
        title={id}
        aria-label={`${t("admin.copyId")}: ${id}`}
        className="-my-1.5 flex h-11 min-h-[44px] w-11 min-w-[44px] shrink-0 cursor-pointer items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        {copied ? (
          <Check className="h-4 w-4" aria-hidden="true" />
        ) : (
          <Copy className="h-4 w-4" aria-hidden="true" />
        )}
      </button>
      <span aria-live="polite" className="sr-only">
        {copied ? t("app.copied") : ""}
      </span>
    </span>
  );
}
