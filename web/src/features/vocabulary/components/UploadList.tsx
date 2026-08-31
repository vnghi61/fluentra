import React, { useState } from "react";
import { Check, ChevronDown, Clock, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

import { useUpload, type VocabUpload } from "../api/uploadApi";

/**
 * What became of a learner's uploads.
 *
 * The counts lead, because the question a learner has after pasting is "did it
 * work" and not "which words". Opening a row fetches the words themselves —
 * three hundred of them are not worth loading for a list nobody has opened.
 *
 * A pending upload says so plainly. The checking runs hourly, and a screen that
 * showed nothing between submitting and the job running would read as the
 * upload having been lost.
 */
export interface UploadListProps {
  uploads: VocabUpload[];
  isLoading?: boolean;
}

const statusStyles: Record<string, string> = {
  verified: "border-success/30 bg-success/10 text-success-accent",
  rejected: "border-danger/30 bg-danger/10 text-danger-accent",
  pending: "border-border bg-surface-muted text-text-muted",
  failed: "border-warning/30 bg-warning/10 text-text",
};

export const UploadList: React.FC<UploadListProps> = ({
  uploads,
  isLoading = false,
}) => {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-xl" />
        <Skeleton className="h-20 w-full rounded-xl" />
      </div>
    );
  }

  if (uploads.length === 0) {
    return (
      <p className="rounded-xl border border-dashed border-border p-6 text-center text-sm text-text-muted">
        {t(
          "uploads.empty",
          "Nothing added yet. Paste a list above and we will check it for you.",
        )}
      </p>
    );
  }

  return (
    <ul className="space-y-3">
      {uploads.map((upload) => (
        <UploadRow key={upload.id} upload={upload} />
      ))}
    </ul>
  );
};

const UploadRow: React.FC<{ upload: VocabUpload }> = ({ upload }) => {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  // Only fetched once opened: three hundred words is not worth loading for a
  // row nobody has expanded.
  const detail = useUpload(open ? upload.id : undefined);

  const pending = upload.pending_count ?? 0;
  const created = new Date(upload.created_at).toLocaleString();

  return (
    <li className="rounded-xl border border-border bg-surface-card">
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-3 p-4 text-left"
      >
        <div className="space-y-1">
          <p className="text-sm font-semibold text-text">
            {upload.item_count} {t("uploads.words", "words")} · {created}
          </p>
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Count
              icon={<Check className="h-3.5 w-3.5" aria-hidden="true" />}
              label={t("uploads.verified", "added")}
              value={upload.verified_count}
              tone="verified"
            />
            {upload.rejected_count > 0 && (
              <Count
                icon={<X className="h-3.5 w-3.5" aria-hidden="true" />}
                label={t("uploads.rejected", "not found")}
                value={upload.rejected_count}
                tone="rejected"
              />
            )}
            {pending > 0 && (
              <Count
                icon={<Clock className="h-3.5 w-3.5" aria-hidden="true" />}
                label={t("uploads.checking", "being checked")}
                value={pending}
                tone="pending"
              />
            )}
          </div>
        </div>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-text-muted transition-transform",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>

      {open && (
        <div className="border-t border-border-subtle p-4">
          {detail.isLoading && <Skeleton className="h-16 w-full rounded-lg" />}
          {detail.data && (
            <ul className="space-y-1.5">
              {(detail.data.items ?? []).map((item) => (
                <li
                  key={item.term}
                  className="flex items-start justify-between gap-3 text-sm"
                >
                  <div className="space-y-0.5">
                    <span className="font-medium text-text">{item.term}</span>
                    {item.provided_meaning && (
                      <span className="text-text-muted">
                        {" "}
                        — {item.provided_meaning}
                      </span>
                    )}
                    {/* The reason is written for the learner, so it is shown
                        rather than reduced to a status chip. */}
                    {item.reason && (
                      <p className="text-xs text-text-muted">{item.reason}</p>
                    )}
                  </div>
                  <span
                    className={cn(
                      "shrink-0 rounded-md border px-2 py-0.5 text-xs font-semibold",
                      statusStyles[item.status] ?? statusStyles["pending"],
                    )}
                  >
                    {t(`uploads.status.${item.status}`, item.status)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </li>
  );
};

const Count: React.FC<{
  icon: React.ReactNode;
  label: string;
  value: number;
  tone: keyof typeof statusStyles;
}> = ({ icon, label, value, tone }) => (
  <span
    className={cn(
      "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 font-semibold",
      statusStyles[tone],
    )}
  >
    {icon}
    {value} {label}
  </span>
);
