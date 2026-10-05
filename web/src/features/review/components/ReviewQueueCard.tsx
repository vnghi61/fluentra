import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, ChevronRight, Layers } from "lucide-react";
import { useTranslation } from "react-i18next";

export interface ReviewQueueCardProps {
  dueCount: number;
}

/** A compact door to either the review queue or the next lesson. */
export const ReviewQueueCard: React.FC<ReviewQueueCardProps> = ({
  dueCount,
}) => {
  const { t } = useTranslation();

  if (dueCount === 0) {
    return (
      <Link
        to="/learn"
        className="group flex min-h-[44px] min-w-0 items-center gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4 shadow-sm transition duration-150 hover:-translate-y-px hover:bg-surface-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none motion-reduce:hover:translate-y-0 md:p-5"
      >
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-success/10 text-success">
          <CheckCircle2 className="h-5 w-5" aria-hidden="true" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-base font-semibold text-text">
            {t("practice.review.emptyTitle", "No cards due today.")}
          </span>
          <span className="hidden text-sm text-text-muted md:block md:line-clamp-2">
            {t(
              "practice.review.emptyCompact",
              "Finish a lesson to add new cards to your queue.",
            )}
          </span>
        </span>
        <span className="hidden rounded-lg border border-border-subtle px-3 py-2 text-sm font-medium text-text md:inline">
          {t("practice.review.emptyLearnBtn", "Go to a lesson")}
        </span>
        <ChevronRight
          className="h-5 w-5 shrink-0 text-text-muted"
          aria-hidden="true"
        />
      </Link>
    );
  }

  return (
    <Link
      to="/practice/review"
      className="group flex min-h-[44px] min-w-0 items-center gap-3 rounded-2xl border border-primary/30 bg-primary/5 p-4 shadow-sm transition duration-150 hover:-translate-y-px hover:bg-primary/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none motion-reduce:hover:translate-y-0 md:p-5"
    >
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        <Layers className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline gap-1.5">
          <span className="text-2xl font-extrabold tracking-tight text-text">
            {dueCount}
          </span>
          <span className="truncate text-sm font-medium text-text-muted">
            {t("practice.review.dueCount", "cards due")}
          </span>
        </span>
        <span className="hidden text-sm text-text-muted md:block md:line-clamp-2">
          {t(
            "practice.review.dueDesc",
            "Grade each card by how well you recalled it. The schedule adapts to your answers.",
          )}
        </span>
      </span>
      <span className="hidden rounded-lg border border-primary/30 px-3 py-2 text-sm font-medium text-primary md:inline">
        {t("practice.review.reviewNow", "Review now →")}
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-primary"
        aria-hidden="true"
      />
    </Link>
  );
};
