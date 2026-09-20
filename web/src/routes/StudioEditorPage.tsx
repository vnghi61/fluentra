import React from "react";
import { useParams } from "@tanstack/react-router";
import { CourseEditor } from "@/features/studio";

export function StudioEditorPage(): React.JSX.Element {
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const draftId = params["draftId"];

  return <CourseEditor draftId={draftId} />;
}

export default StudioEditorPage;
