import React, { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useExamAttempt } from "@/features/exam";
import { ExamSittingRunner } from "@/features/exam/components/ExamSittingRunner";

export function ExamSittingPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params = useParams({ strict: false }) as { attemptId?: string };
  const attemptId = params.attemptId || "";

  const { data: attempt, isLoading, error, refetch } = useExamAttempt(attemptId);

  // If already completed or expired, navigate to report view
  useEffect(() => {
    if (attempt && (attempt.status === "completed" || attempt.status === "expired")) {
      void navigate({
        to: "/exams/$attemptId/report",
        params: { attemptId },
      });
    }
  }, [attempt, attemptId, navigate]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center p-4 bg-background">
        <Loader2 className="h-10 w-10 animate-spin text-primary mb-3" />
        <p className="text-sm text-text-muted font-medium">
          {t("exam.runner.loadingSitting", "Preparing your exam sitting...")}
        </p>
      </div>
    );
  }

  if (error || !attempt) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center p-6 bg-background">
        <div className="max-w-md w-full rounded-2xl border border-danger/30 bg-danger/10 p-6 text-center space-y-4">
          <AlertCircle className="h-10 w-10 text-danger mx-auto" />
          <h2 className="text-lg font-bold text-danger">
            {t("exam.runner.loadSittingFailed", "Unable to load exam sitting")}
          </h2>
          <p className="text-xs text-danger/80">
            {error instanceof Error ? error.message : t("common.unknownError", "An unexpected error occurred.")}
          </p>
          <div className="pt-2 flex justify-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void refetch()}
              className="min-h-[44px]"
            >
              {t("common.retry", "Retry")}
            </Button>
            <Link to="/exams">
              <Button type="button" className="min-h-[44px] bg-primary text-white">
                <ArrowLeft className="h-4 w-4 mr-1.5" />
                {t("exam.report.returnToExams", "Back to Exams")}
              </Button>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      <ExamSittingRunner
        attempt={attempt}
        onSubmitted={() => {
          void navigate({
            to: "/exams/$attemptId/report",
            params: { attemptId },
          });
        }}
      />
    </div>
  );
}
