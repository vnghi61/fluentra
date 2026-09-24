import React from "react";

import { ExamHub } from "@/features/exam/components/ExamHub";

/**
 * The exam hub (WO 22 Stage L): exams, their numbered tests, and one test sat
 * full and timed or practised. The old templates/mock tabs are absorbed: the
 * custom composer is one door inside a version's test list.
 */
export function ExamsPage(): React.JSX.Element {
  return (
    <div className="w-full">
      <ExamHub />
    </div>
  );
}
