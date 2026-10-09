import React from "react";
import { Flag, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Progress } from "@/components/ui/progress";

export interface RunnerHeaderProps {
  lessonTitle: string;
  currentStep: number;
  totalSteps: number;
  onExit: () => void;
  /**
   * Opens the report dialog for the current item. In the header rather than
   * only under a submitted answer: on a phone the post-answer link sat below
   * the exercise's own actions and was easy never to reach.
   */
  onReport?: (() => void) | undefined;
}

const iconButton =
  "flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center rounded-xl border border-border-subtle bg-surface-card text-text-muted transition-colors hover:bg-surface-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none";

export const RunnerHeader: React.FC<RunnerHeaderProps> = ({
  lessonTitle,
  currentStep,
  totalSteps,
  onExit,
  onReport,
}) => {
  const { t } = useTranslation();
  const progressPercent =
    totalSteps > 0 ? Math.round((currentStep / totalSteps) * 100) : 0;
  const stepLabel = t("runner.stepProgress", {
    current: currentStep,
    total: totalSteps,
    defaultValue: `Activity ${currentStep} of ${totalSteps}`,
  });

  return (
    <header className="sticky top-0 z-30 border-b border-border-subtle bg-surface-card px-3 py-3 sm:px-5 md:static md:rounded-t-2xl">
      <div className="flex items-center gap-3 sm:gap-4">
        <button
          type="button"
          onClick={onExit}
          className={iconButton}
          aria-label={t("runner.exit", "Exit")}
        >
          <X className="h-5 w-5" aria-hidden="true" />
        </button>

        <div className="min-w-0 flex-1 space-y-1.5 md:flex md:items-center md:gap-4 md:space-y-0">
          <div className="flex items-center justify-between gap-2 md:contents">
            <h1 className="min-w-0 truncate text-sm font-semibold text-text md:max-w-[220px] md:shrink-0 md:text-base">
              {lessonTitle}
            </h1>
            <span
              data-testid="runner-step-counter"
              data-current={currentStep}
              data-total={totalSteps}
              className="shrink-0 text-xs font-medium text-text-muted md:order-last md:text-sm"
            >
              {stepLabel}
            </span>
          </div>
          <Progress
            value={progressPercent}
            className="h-2 md:flex-1"
            aria-label={stepLabel}
          />
        </div>

        {onReport && (
          <button
            type="button"
            onClick={onReport}
            className={iconButton}
            aria-label={t("report.reportBtn", "Report issue")}
            title={t("report.reportBtn", "Report issue")}
          >
            <Flag className="h-5 w-5" aria-hidden="true" />
          </button>
        )}
      </div>
    </header>
  );
};
