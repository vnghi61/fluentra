import React from "react";
import { FileQuestion } from "lucide-react";
import { useTranslation } from "react-i18next";

export interface CardContentUnavailableProps {
  /** The scheduling record the queue did return, so the learner can still skip it. */
  contentVersionId: string;
}

/**
 * Rendered when a due card arrives with no renderable content.
 *
 * `GET /reviews/session` resolves each card's content version and returns the
 * authored body, so this is the archived, unauthored or half-authored case — a
 * version that no longer exists, or one whose body carries no word to put on the
 * front. The card stays gradable, because the schedule behind it is real.
 *
 * It exists because the first version of this screen filled that gap with a
 * hard-coded "meticulous", and every card in every learner's queue displayed it.
 */
export const CardContentUnavailable: React.FC<CardContentUnavailableProps> = ({
  contentVersionId,
}) => {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-[300px] w-full flex-col items-center justify-center gap-3 rounded-[var(--radius-card)] border border-dashed border-border bg-surface-card p-5 text-center sm:min-h-[340px] sm:p-8">
      <span className="flex h-12 w-12 items-center justify-center rounded-xl bg-surface-muted text-text-muted">
        <FileQuestion className="h-6 w-6" aria-hidden="true" />
      </span>
      <h2 className="text-base font-semibold text-text">
        {t("review.contentUnavailable")}
      </h2>
      <p className="text-sm leading-relaxed text-text-muted max-w-sm">
        {t("review.contentUnavailableDesc")}
      </p>
      <code className="text-xs text-text-muted font-mono break-all">
        {contentVersionId}
      </code>
    </div>
  );
};
