import React from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useExamReport } from "@/features/exam";
import { ExamReport } from "@/features/exam/components/ExamReport";

export function ExamReportPage(): React.JSX.Element {
  const { t } = useTranslation();
  const params = useParams({ strict: false }) as { attemptId?: string };
  const attemptId = params.attemptId || "";

  const { data: report, isLoading, error, refetch } = useExamReport(attemptId);

  if (isLoading) {
    return (
      <div className="min-h-[60vh] flex flex-col items-center justify-center p-6">
        <Loader2 className="h-10 w-10 animate-spin text-primary mb-3" />
        <p className="text-sm text-text-muted font-medium">
          {t("exam.report.loadingReport", "Loading score report...")}
        </p>
      </div>
    );
  }

  if (error || !report) {
    return (
      <div className="max-w-md mx-auto my-12 rounded-2xl border border-danger/30 bg-danger/10 p-6 text-center space-y-4">
        <AlertCircle className="h-10 w-10 text-danger mx-auto" />
        <h2 className="text-lg font-bold text-danger">
          {t("exam.report.loadReportFailed", "Unable to load score report")}
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
    );
  }

  return (
    <div className="w-full">
      <ExamReport report={report} onRefresh={() => void refetch()} />
    </div>
  );
}
