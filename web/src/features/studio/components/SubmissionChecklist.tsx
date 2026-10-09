import React from "react";
import { useTranslation } from "react-i18next";
import { CheckCircle2, Circle, FileUp, ShieldCheck } from "lucide-react";

import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { formatBytes } from "@/features/resource";
import { cn } from "@/lib/utils";
import { LIMITS, type CourseMetrics } from "../model/courseStructure";

export interface ChecklistItem {
  key: string;
  done: boolean;
  label: string;
  detail?: string | undefined;
}

interface Props {
  items: ChecklistItem[];
  metrics: CourseMetrics;
}

/**
 * What Gate 1 will check, ticked off live, so a creator learns about a gap
 * here rather than one submission round-trip at a time.
 */
export function SubmissionChecklist({
  items,
  metrics,
}: Props): React.JSX.Element {
  const { t } = useTranslation();
  const done = items.filter((i) => i.done).length;
  const pct = Math.round((done / items.length) * 100);

  const stats = [
    { label: t("studio.editor.stat.units"), value: metrics.units },
    { label: t("studio.editor.stat.lessons"), value: metrics.lessons },
    {
      label: t("studio.editor.stat.exercises"),
      value: `${metrics.exercises}/${LIMITS.minExercises}`,
    },
    { label: t("studio.editor.stat.materials"), value: metrics.materials },
  ];

  return (
    <div className="space-y-4">
      <Card className="p-4 sm:p-5">
        <div className="flex items-baseline justify-between gap-2">
          <h2 className="text-lg font-semibold leading-snug text-text">
            {t("studio.editor.requirementsTitle")}
          </h2>
          <span className="text-sm font-semibold tabular-nums text-text-muted">
            {done}/{items.length}
          </span>
        </div>
        <Progress
          value={pct}
          variant={pct === 100 ? "success" : "primary"}
          className="mt-3 h-2"
          aria-label={t("studio.editor.requirementsProgress")}
        />
        <ul className="mt-4 space-y-2.5">
          {items.map((item) => (
            <li key={item.key} className="flex items-start gap-2.5 text-sm">
              {item.done ? (
                <CheckCircle2
                  className="mt-0.5 h-4 w-4 shrink-0 text-success"
                  aria-hidden="true"
                />
              ) : (
                <Circle
                  className="mt-0.5 h-4 w-4 shrink-0 text-text-muted"
                  aria-hidden="true"
                />
              )}
              <span className="min-w-0">
                <span
                  className={cn(item.done ? "text-text" : "text-text-muted")}
                >
                  {item.label}
                </span>
                <span className="sr-only">
                  {item.done
                    ? t("studio.editor.checkDone")
                    : t("studio.editor.checkTodo")}
                </span>
                {item.detail && (
                  <span className="block text-sm text-text-muted">
                    {item.detail}
                  </span>
                )}
              </span>
            </li>
          ))}
        </ul>

        <dl className="mt-5 grid grid-cols-2 gap-2 border-t border-border-subtle pt-4">
          {stats.map((s) => (
            <div
              key={s.label}
              className="rounded-xl bg-surface-muted/70 px-3 py-2"
            >
              <dt className="text-xs text-text-muted">{s.label}</dt>
              <dd className="text-lg font-bold tabular-nums text-text">
                {s.value}
              </dd>
            </div>
          ))}
        </dl>
      </Card>

      <Card className="space-y-3 p-4 text-sm sm:p-5">
        <h2 className="flex items-center gap-2 text-lg font-semibold leading-snug text-text">
          <FileUp
            className="h-5 w-5 shrink-0 text-primary-accent"
            aria-hidden="true"
          />
          {t("studio.material.guideTitle")}
        </h2>
        <ul className="space-y-1.5 text-sm text-text-muted">
          <li>
            {t("studio.material.guideDocs", {
              max: formatBytes(20 * 1024 * 1024),
            })}
          </li>
          <li>
            {t("studio.material.guideImages", {
              max: formatBytes(10 * 1024 * 1024),
            })}
          </li>
          <li>
            {t("studio.material.guideVideo", {
              max: formatBytes(50 * 1024 * 1024),
            })}
          </li>
        </ul>
        <p className="flex items-start gap-2 rounded-lg bg-surface-muted/70 p-2.5 text-sm text-text-muted">
          <ShieldCheck
            className="mt-0.5 h-4 w-4 shrink-0 text-success"
            aria-hidden="true"
          />
          {t("studio.material.guideSecurity")}
        </p>
      </Card>
    </div>
  );
}
