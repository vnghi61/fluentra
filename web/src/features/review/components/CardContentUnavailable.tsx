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
    <div className="w-full min-h-[300px] p-8 rounded-3xl border-2 border-dashed border-border-subtle bg-surface-card flex flex-col items-center justify-center text-center gap-3">
      <FileQuestion className="h-10 w-10 text-text-muted" aria-hidden="true" />
      <p className="text-base font-semibold text-text">
        {t("review.contentUnavailable")}
      </p>
      <p className="text-sm text-text-muted max-w-sm">
        {t("review.contentUnavailableDesc")}
      </p>
      <code className="text-[11px] text-text-muted font-mono break-all">
        {contentVersionId}
      </code>
    </div>
  );
};
