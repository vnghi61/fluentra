import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { placementApi, type PlacementSession } from "../../api/placement";
import { WeeklyPlanCard } from "../Dashboard/WeeklyPlanCard";
import { PlacementProductive } from "./PlacementProductive";
import { StartingPathCard } from "./StartingPathCard";

export interface PlacementResultViewProps {
  session: PlacementSession;
  retakeAvailableAt?: string | null | undefined;
  onSession: (session: PlacementSession) => void;
  onReload: () => void;
}

const MEASURED = ["vocabulary", "grammar", "reading", "listening"] as const;

/**
 * The result: the placed level, the four measured skills, writing and speaking
 * to take now or later, the recommended course and the weekly plan.
 */
export const PlacementResultView: React.FC<PlacementResultViewProps> = ({
  session,
  retakeAvailableAt,
  onSession,
  onReload,
}) => {
  const { t, i18n } = useTranslation();
  const [now] = useState(() => Date.now());
  const [retaking, setRetaking] = useState(false);
  const [retakeError, setRetakeError] = useState<string | null>(null);
  const result = session.result;
  if (!result) return null;

  const levelNames: Record<string, string> = {
    A1: t("placement.levels.A1"),
    A2: t("placement.levels.A2"),
    B1: t("placement.levels.B1"),
    B2: t("placement.levels.B2"),
    C1: t("placement.levels.C1"),
  };
  const retakeAt = retakeAvailableAt ? new Date(retakeAvailableAt) : null;
  const canRetake = retakeAt !== null && retakeAt.getTime() <= now;

  const retake = async () => {
    setRetaking(true);
    setRetakeError(null);
    try {
      onSession(await placementApi.start());
    } catch {
      setRetakeError(t("placement.result.retakeFailed"));
      setRetaking(false);
    }
  };

  return (
    <div className="min-h-screen bg-surface-base px-4 py-8 md:py-12">
      <div className="mx-auto max-w-3xl space-y-6">
        <section className="space-y-4 rounded-2xl border border-primary/30 bg-primary/5 p-6 text-center">
          <p className="text-sm font-semibold uppercase tracking-wide text-primary">
            {t("placement.result.eyebrow")}
          </p>
          <h1 className="text-5xl font-extrabold text-text">{result.level}</h1>
          <p className="text-lg text-text-muted">{levelNames[result.level]}</p>
          <dl className="grid grid-cols-2 gap-3 pt-2 sm:grid-cols-4">
            {MEASURED.map((skill) => (
              <div
                key={skill}
                className="rounded-xl border border-border-subtle bg-surface-card p-3"
              >
                <dt className="text-xs text-text-muted">
                  {t(`skills.${skill}`)}
                </dt>
                <dd className="text-lg font-bold text-text">
                  {result.per_skill[skill]?.band ??
                    t("placement.result.notMeasured")}
                </dd>
              </div>
            ))}
          </dl>
        </section>

        <PlacementProductive
          session={session}
          onSession={onSession}
          onReload={onReload}
        />
        <StartingPathCard />
        <WeeklyPlanCard />

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Link
            to="/"
            className="inline-flex min-h-[44px] items-center justify-center rounded-lg border border-border px-6 text-base font-medium text-text hover:bg-surface-muted"
          >
            {t("placement.result.toDashboard")}
          </Link>
          {canRetake ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => void retake()}
              disabled={retaking}
              className="min-h-[44px] text-base"
            >
              {t("placement.result.retake")}
            </Button>
          ) : (
            retakeAt && (
              <p className="text-sm text-text-muted">
                {t("placement.result.retakeOn", {
                  date: retakeAt.toLocaleDateString(i18n.language),
                })}
              </p>
            )
          )}
        </div>
        {retakeError && (
          <p role="alert" className="text-sm text-danger">
            {retakeError}
          </p>
        )}
      </div>
    </div>
  );
};
