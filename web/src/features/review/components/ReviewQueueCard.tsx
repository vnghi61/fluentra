import React from "react";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { ArrowRight, CheckCircle2, Layers, LogIn } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { StackedCardsArt } from "@/components/ui/stacked-cards-art";
import { cn } from "@/lib/utils";

export interface ReviewQueueCardProps {
  dueCount: number;
  /** A visitor without an account: reviews need one, so the card says so. */
  guest?: boolean;
  className?: string;
}

interface QueueShellProps {
  icon: LucideIcon;
  eyebrowClassName: string;
  className?: string | undefined;
  children: React.ReactNode;
  action: React.ReactNode;
}

function QueueShell({
  icon,
  eyebrowClassName,
  className,
  children,
  action,
}: QueueShellProps): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <Card
      className={cn(
        "relative flex h-full flex-col gap-4 overflow-hidden p-5 md:p-6",
        className,
      )}
    >
      <div className="flex items-start gap-4">
        <div className="min-w-0 flex-1 space-y-1.5">
          <p
            className={cn(
              "text-xs font-semibold uppercase tracking-wider",
              eyebrowClassName,
            )}
          >
            {t("practice.review.label", "Spaced repetition")}
          </p>
          {children}
        </div>
        <StackedCardsArt icon={icon} size="sm" className="mr-1 mt-1" />
      </div>
      <div className="mt-auto flex flex-wrap gap-3">{action}</div>
    </Card>
  );
}

/** The door into the review queue, or to the next lesson when nothing is due. */
export const ReviewQueueCard: React.FC<ReviewQueueCardProps> = ({
  dueCount,
  guest = false,
  className,
}) => {
  const { t } = useTranslation();

  if (guest) {
    return (
      <QueueShell
        icon={LogIn}
        eyebrowClassName="text-text-muted"
        className={className}
        action={
          <Button asChild variant="outline" className="w-full sm:w-auto">
            <Link to="/learn">
              {t("practice.review.emptyLearnBtn", "Go to a lesson")}
            </Link>
          </Button>
        }
      >
        <p className="text-base font-semibold text-text">
          {t("guest.practice.title", "Reviews need an account")}
        </p>
        <p className="text-sm text-text-muted">
          {t(
            "guest.practice.desc",
            "Sign in and the lessons you finish start filling this queue.",
          )}
        </p>
      </QueueShell>
    );
  }

  if (dueCount === 0) {
    return (
      <QueueShell
        icon={CheckCircle2}
        eyebrowClassName="text-success-accent"
        className={className}
        action={
          <Button asChild variant="outline" className="w-full sm:w-auto">
            <Link to="/learn">
              {t("practice.review.emptyLearnBtn", "Go to a lesson")}
            </Link>
          </Button>
        }
      >
        <p className="text-base font-semibold text-text">
          {t("practice.review.emptyTitle", "No cards due today.")}
        </p>
        <p className="text-sm text-text-muted">
          {t(
            "practice.review.emptyCompact",
            "Finish a lesson to add new cards to your queue.",
          )}
        </p>
      </QueueShell>
    );
  }

  return (
    <QueueShell
      icon={Layers}
      eyebrowClassName="text-primary-accent"
      className={cn(
        "border-primary/25 bg-gradient-to-br from-primary/10 via-surface-card to-surface-card",
        className,
      )}
      action={
        <Button asChild className="w-full sm:w-auto">
          <Link to="/practice/review">
            {t("practice.review.reviewNow", "Review now")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </Button>
      }
    >
      <p className="flex items-baseline gap-1.5">
        <span className="text-3xl font-bold tracking-tight tabular-nums text-text">
          {dueCount}
        </span>
        <span className="text-sm font-medium text-text-muted">
          {t("practice.review.dueCount", "cards due")}
        </span>
      </p>
      <p className="text-sm text-text-muted">
        {t(
          "practice.review.dueDesc",
          "Grade each card by how well you recalled it. The schedule adapts to your answers.",
        )}
      </p>
    </QueueShell>
  );
};
