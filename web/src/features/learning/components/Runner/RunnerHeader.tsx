import React from "react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";

export interface RunnerHeaderProps {
  lessonTitle: string;
  currentStep: number;
  totalSteps: number;
  onExit: () => void;
}

export const RunnerHeader: React.FC<RunnerHeaderProps> = ({
  lessonTitle,
  currentStep,
  totalSteps,
  onExit,
}) => {
  const { t } = useTranslation();
  const progressPercent = totalSteps > 0 ? Math.round((currentStep / totalSteps) * 100) : 0;

  return (
    <header className="border-b border-border bg-surface-card sticky top-0 z-30 px-4 py-3">
      <div className="max-w-4xl mx-auto flex items-center justify-between gap-4">
        {/* Exit Button */}
        <Button
          variant="ghost"
          size="sm"
          onClick={onExit}
          className="text-text-muted hover:text-text -ml-2 h-11 w-11 p-0 rounded-full"
          aria-label={t("runner.exit", "Exit")}
        >
          <X className="h-5 w-5" aria-hidden="true" />
        </Button>

        {/* Progress Bar & Counter */}
        <div className="flex-1 max-w-md space-y-1">
          <div className="flex justify-between text-xs text-text-muted font-medium">
            <span className="truncate max-w-[200px] font-semibold text-text">
              {lessonTitle}
            </span>
            <span
              data-testid="runner-step-counter"
              data-current={currentStep}
              data-total={totalSteps}
            >
              {t("runner.stepProgress", {
                current: currentStep,
                total: totalSteps,
                defaultValue: `Activity ${currentStep} of ${totalSteps}`,
              })}
            </span>
          </div>
          <Progress
            value={progressPercent}
            aria-label={t("runner.stepProgress", {
              current: currentStep,
              total: totalSteps,
              defaultValue: `Activity ${currentStep} of ${totalSteps}`,
            })}
          />
        </div>
      </div>
    </header>
  );
};
