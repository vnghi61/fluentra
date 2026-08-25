import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface CompletionScreenProps {
  score: number;
  totalActivities: number;
  timeSpentSeconds: number;
  onRetryLesson?: () => void;
}

export const CompletionScreen: React.FC<CompletionScreenProps> = ({
  score,
  totalActivities,
  timeSpentSeconds,
  onRetryLesson,
}) => {
  const { t } = useTranslation();
  // Zero of zero is zero, not full marks. The old default reported 100 %
  // accuracy for a lesson with no activities in it.
  const accuracy =
    totalActivities > 0 ? Math.round((score / totalActivities) * 100) : 0;
  const minutes = Math.floor(timeSpentSeconds / 60);
  const seconds = timeSpentSeconds % 60;
  const formattedTime = `${minutes}m ${seconds}s`;

  return (
    <div className="max-w-md mx-auto py-12 px-4 animate-in fade-in motion-reduce:animate-none">
      <Card className="text-center border-success/40 bg-gradient-to-b from-surface-card to-success/5 shadow-xl">
        <CardHeader className="space-y-3 pb-4">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-success/15 text-success">
            <CheckCircle2 className="h-10 w-10" aria-hidden="true" />
          </div>
          <CardTitle className="text-2xl md:text-3xl font-extrabold text-text">
            {t("runner.completedTitle", "Lesson Completed!")}
          </CardTitle>
          <CardDescription className="text-sm">
            {t(
              "runner.completedDesc",
              "You've completed all activities in this lesson. Your mastery scores have been updated.",
            )}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Summary Metrics Grid */}
          <div className="grid grid-cols-2 gap-3 pt-2">
            <div className="p-4 rounded-xl border border-border bg-surface-muted/50 text-center space-y-1">
              <span className="text-xs text-text-muted font-medium">
                {t("runner.accuracyLabel", "Accuracy")}
              </span>
              <p className="text-2xl font-extrabold text-text">{accuracy}%</p>
            </div>

            <div className="p-4 rounded-xl border border-border bg-surface-muted/50 text-center space-y-1">
              <span className="text-xs text-text-muted font-medium">
                {t("runner.timeSpentLabel", "Time Spent")}
              </span>
              <p className="text-2xl font-extrabold text-text">{formattedTime}</p>
            </div>
          </div>
        </CardContent>

        <CardFooter className="flex flex-col gap-3 pt-2">
          <Link to="/learn" className="w-full">
            <Button size="lg" className="w-full font-bold gap-2 min-h-[48px]">
              {t("runner.backToCourseBtn", "Back to Syllabus")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>

          {onRetryLesson && (
            <Button
              variant="outline"
              onClick={onRetryLesson}
              className="w-full gap-2 min-h-[44px]"
            >
              <RotateCcw className="h-4 w-4" aria-hidden="true" />
              {t("action.retry", "Practice Again")}
            </Button>
          )}
        </CardFooter>
      </Card>
    </div>
  );
};
