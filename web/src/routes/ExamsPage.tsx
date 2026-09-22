import React, { useState } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { ExamList } from "@/features/exam/components/ExamList";
import { MockTestComposer } from "@/features/exam/components/MockTestComposer";
import { usePreferencesStore } from "@/stores/preferencesStore";

/**
 * Two doors to an exam: the authored templates, and a test composed from the
 * published bank. They are tabs rather than separate routes because they are
 * the same intent — "give me something to sit" — and the runner, report and
 * attempt history are shared.
 */
export function ExamsPage(): React.JSX.Element {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"templates" | "mock">("templates");
  const practiceLevel = usePreferencesStore(
    (s) => s.preferences?.practice_level,
  );

  return (
    <div className="w-full space-y-4">
      <div className="flex gap-2 border-b border-border-subtle pb-px">
        {(
          [
            ["templates", t("exam.tabTemplates", "Mock Exams")],
            ["mock", t("exam.tabMockTests", "Mock tests")],
          ] as const
        ).map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => setTab(key)}
            className={cn(
              "min-h-[44px] whitespace-nowrap border-b-2 px-4 py-2 text-sm font-medium transition-colors",
              tab === key
                ? "border-primary text-primary-accent"
                : "border-transparent text-text-muted hover:text-text",
            )}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "templates" ? (
        <ExamList userPracticeLevel={practiceLevel || undefined} />
      ) : (
        <MockTestComposer />
      )}
    </div>
  );
}
