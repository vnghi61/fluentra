import React, { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  learningKeys,
  usePlacementOverview,
  usePlacementSession,
  type PlacementSession,
} from "@/features/learning";
import { PlacementIntro } from "@/features/learning/components/Placement/PlacementIntro";
import { PlacementResultView } from "@/features/learning/components/Placement/PlacementResultView";
import { PlacementRunner } from "@/features/learning/components/Placement/PlacementRunner";

/**
 * The placement test, full screen. The session in progress, or else the most
 * recent result, decides what is shown; with neither, the invitation to start.
 */
export function PlacementPage(): React.JSX.Element {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const overview = usePlacementOverview();
  const [chosenId, setChosenId] = useState<string | null>(null);
  const sessionId =
    chosenId ??
    overview.data?.active_session?.id ??
    overview.data?.result?.session_id ??
    null;
  const session = usePlacementSession(sessionId);

  const showSession = (next: PlacementSession) => {
    queryClient.setQueryData(learningKeys.placementSession(next.id), next);
    setChosenId(next.id);
    if (next.status !== "in_progress") {
      void queryClient.invalidateQueries({ queryKey: learningKeys.placement() });
      void queryClient.invalidateQueries({ queryKey: learningKeys.startingPath() });
      void queryClient.invalidateQueries({ queryKey: learningKeys.weeklyPlan() });
    }
  };

  const reload = () => {
    void session.refetch();
    void queryClient.invalidateQueries({ queryKey: learningKeys.placement() });
  };

  if (overview.isLoading || session.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-surface-base px-4">
        <p className="text-sm text-text-muted">{t("placement.loading")}</p>
      </div>
    );
  }

  if (overview.isError || session.isError) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-surface-base px-4">
        <p role="alert" className="text-sm text-danger">
          {t("placement.loadFailed")}
        </p>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            void overview.refetch();
            reload();
          }}
          className="min-h-[44px] text-base"
        >
          {t("placement.retry")}
        </Button>
      </div>
    );
  }

  const current = session.data;
  if (current?.status === "in_progress") {
    return (
      <PlacementRunner session={current} onSession={showSession} onReload={reload} />
    );
  }
  if (current?.result) {
    return (
      <PlacementResultView
        session={current}
        retakeAvailableAt={overview.data?.retake_available_at}
        onSession={showSession}
        onReload={reload}
      />
    );
  }
  return (
    <PlacementIntro
      notice={
        current?.status === "expired" ? t("placement.intro.tooFewAnswers") : undefined
      }
      retakeAvailableAt={overview.data?.retake_available_at}
      onStarted={showSession}
      onConflict={() => {
        setChosenId(null);
        void overview.refetch();
      }}
    />
  );
}

export default PlacementPage;
