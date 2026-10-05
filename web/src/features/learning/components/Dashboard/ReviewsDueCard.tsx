import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2, Layers } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export interface ReviewsDueCardProps {
  dueCount: number;
}

export const ReviewsDueCard: React.FC<ReviewsDueCardProps> = ({ dueCount }) => {
  const { t } = useTranslation();

  if (dueCount === 0) {
    return (
      <Card className="bg-surface-card border border-border-subtle rounded-2xl shadow-[0_2px_8px_rgba(15,23,42,0.04)] p-4 md:p-5">
        <div className="flex flex-row items-center justify-between gap-3">
          <div className="flex items-center gap-2.5 min-w-0">
            <CheckCircle2
              className="h-5 w-5 text-success shrink-0"
              aria-hidden="true"
            />
            <span className="text-sm font-medium text-text">
              {t(
                "dashboard.reviews.emptyCompact",
                "All caught up! No cards due right now.",
              )}
            </span>
          </div>
          <Link
            to="/practice/my-words"
            className="shrink-0 text-sm font-medium text-primary-accent hover:underline inline-flex items-center min-h-[44px] min-w-[44px] px-1"
          >
            {t("dashboard.reviews.vocabLink", "View vocabulary →")}
          </Link>
        </div>
      </Card>
    );
  }

  return (
    <Card className="bg-surface-card border border-primary/30 rounded-2xl shadow-[0_2px_8px_rgba(15,23,42,0.04)] p-4 md:p-5 transition-all duration-150 hover:-translate-y-0.5 hover:shadow-md motion-reduce:transform-none">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary-accent">
            <Layers className="h-4 w-4" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("dashboard.reviews.title", "Reviews Due")}
            </span>
          </div>
          <h3 className="text-base md:text-lg font-bold text-text">
            {t("dashboard.reviews.dueCount", {
              count: dueCount,
              defaultValue: `${dueCount} words to review`,
            })}
          </h3>
        </div>
        <Link to="/practice/review" className="shrink-0">
          <Button variant="primary" className="gap-2">
            {t("dashboard.reviews.reviewNow", "Review now")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </Link>
      </div>
    </Card>
  );
};
