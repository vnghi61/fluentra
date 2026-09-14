import React from "react";

import { ExamList } from "@/features/exam/components/ExamList";
import { usePreferencesStore } from "@/stores/preferencesStore";

export function ExamsPage(): React.JSX.Element {
  const practiceLevel = usePreferencesStore(
    (s) => s.preferences?.practice_level,
  );

  return (
    <div className="w-full">
      <ExamList userPracticeLevel={practiceLevel || undefined} />
    </div>
  );
}
