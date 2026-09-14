import React, { useCallback, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { AlertCircle, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useExamAttempt } from "@/features/exam";
import { ExamSittingRunner } from "@/features/exam/components/ExamSittingRunner";

export function ExamSittingPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params: Record<string, string | undefined> = useParams({ strict: false });
  const attemptId = params.attemptId ?? "";

  const { data: attempt, isLoading, isError, refetch } = useExamAttempt(attemptId);

  const toReport = useCallback(() => {
    void navigate({ to: "/exams/$attemptId/report", params: { attemptId } });
  }, [attemptId, navigate]);

  // A sitting already submitted — by the learner, the job or its own read — shows its result.
  useEffect(() => {
    if (attempt && attempt.status !== "in_progress") toReport();
  }, [attempt, toReport]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-background p-4">
        <Loader2 className="mb-3 h-10 w-10 animate-spin text-primary" aria-hidden="true" />
        <p className="text-sm font-medium text-text-muted">{t("exam.runner.loading")}</p>
      </div>
    );
  }

  if (isError || !attempt) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-background p-6">
        <div role="alert" className="w-full max-w-md space-y-4 rounded-2xl border border-danger/30 bg-danger/10 p-6 text-center">
          <AlertCircle className="mx-auto h-10 w-10 text-danger" aria-hidden="true" />
          <h2 className="text-lg font-bold text-danger">{t("exam.runner.loadFailed")}</h2>
          <div className="flex justify-center gap-3 pt-2">
            <Button type="button" variant="outline" onClick={() => void refetch()}>
              {t("exam.runner.retry")}
            </Button>
            <Link
              to="/exams"
              className="inline-flex min-h-[44px] items-center rounded-lg bg-primary px-4 text-sm font-semibold text-primary-fg"
            >
              {t("exam.report.back")}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (attempt.status !== "in_progress") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <Loader2 className="h-10 w-10 animate-spin text-primary" aria-hidden="true" />
      </div>
    );
  }

  return <ExamSittingRunner attempt={attempt} onSubmitted={toReport} />;
}
