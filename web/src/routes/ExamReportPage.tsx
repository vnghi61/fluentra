import React from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useExamReport } from "@/features/exam";
import { ExamReport } from "@/features/exam/components/ExamReport";

export function ExamReportPage(): React.JSX.Element {
  const { t } = useTranslation();
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const attemptId = params.attemptId ?? "";

  const {
    data: report,
    isLoading,
    isError,
    refetch,
  } = useExamReport(attemptId);

  if (isLoading) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center p-6">
        <Loader2
          className="mb-3 h-10 w-10 animate-spin text-primary"
          aria-hidden="true"
        />
        <p className="text-sm font-medium text-text-muted">
          {t("exam.report.loading")}
        </p>
      </div>
    );
  }

  if (isError || !report) {
    return (
      <div
        role="alert"
        className="mx-auto my-12 max-w-md space-y-4 rounded-2xl border border-danger/30 bg-danger/10 p-6 text-center"
      >
        <AlertCircle
          className="mx-auto h-10 w-10 text-danger"
          aria-hidden="true"
        />
        <h2 className="text-lg font-bold text-danger">
          {t("exam.report.loadFailed")}
        </h2>
        <div className="flex justify-center gap-3 pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => void refetch()}
          >
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
    );
  }

  return <ExamReport report={report} />;
}
