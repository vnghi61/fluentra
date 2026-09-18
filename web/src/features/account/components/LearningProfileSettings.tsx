import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { Compass, Loader2, Target } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  usePlacementOverview,
  type PlacementOverview,
} from "@/features/learning/api/placement";
import { cn } from "@/lib/utils";
import {
  useLearningProfile,
  useUpdateLearningProfile,
  type LearningProfile,
} from "../api/accountApi";

const EXAMS = ["none", "ielts", "toeic"] as const;
const LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"] as const;
const MINUTE_PRESETS = [60, 90, 150, 300] as const;
const MIN_WEEKLY_MINUTES = 15;
const MAX_WEEKLY_MINUTES = 10080;

type Exam = (typeof EXAMS)[number];
type Level = (typeof LEVELS)[number];

const MEASURED = ["vocabulary", "grammar", "reading", "listening", "writing", "speaking"] as const;

const linkClass =
  "inline-flex min-h-[44px] items-center justify-center rounded-lg bg-primary px-6 text-base font-medium text-primary-fg hover:bg-primary-hover";

const choiceClass = (selected: boolean) =>
  cn(
    "min-h-[44px] rounded-xl border px-3 py-2 text-base font-medium transition-colors",
    selected
      ? "border-primary bg-primary/10 text-text"
      : "border-border-subtle bg-surface-base text-text hover:border-border",
  );

const PlacementSummary: React.FC<{
  overview: PlacementOverview | undefined;
  declaredLevel: string | null;
}> = ({ overview, declaredLevel }) => {
  const { t, i18n } = useTranslation();
  const [now] = useState(() => Date.now());
  const result = overview?.result;
  const retakeAt = overview?.retake_available_at
    ? new Date(overview.retake_available_at)
    : null;
  const canRetake = retakeAt === null || retakeAt.getTime() <= now;

  return (
    <Card className="border-border bg-surface-card">
      <CardHeader className="space-y-1">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Compass className="h-5 w-5 text-primary" aria-hidden="true" />
            <CardTitle className="text-lg font-bold text-text">
              {t("placement.settings.title")}
            </CardTitle>
          </div>
          {result && <Badge variant="primary">{result.level}</Badge>}
        </div>
        <CardDescription className="text-sm text-text-muted">
          {result
            ? t("placement.settings.placedOn", {
                date: new Date(result.taken_at).toLocaleDateString(i18n.language),
              })
            : t("placement.settings.noResult")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {result ? (
          <dl className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            {MEASURED.map((skill) => (
              <div key={skill} className="rounded-lg border border-border-subtle px-3 py-2">
                <dt className="text-xs text-text-muted">{t(`skills.${skill}`)}</dt>
                <dd className="text-base font-bold text-text">
                  {result.per_skill[skill]?.band ?? t("placement.result.notMeasured")}
                </dd>
              </div>
            ))}
          </dl>
        ) : (
          declaredLevel && (
            <p className="text-sm text-text-muted">
              {t("placement.settings.declared", { level: declaredLevel })}
            </p>
          )
        )}
      </CardContent>
      <CardFooter className="flex flex-wrap items-center gap-3">
        {overview?.active_session ? (
          <Link to="/placement" className={linkClass}>
            {t("placement.settings.resume")}
          </Link>
        ) : result && canRetake ? (
          <Link to="/placement" className={linkClass}>
            {t("placement.settings.retake")}
          </Link>
        ) : result && retakeAt ? (
          <p className="text-sm text-text-muted">
            {t("placement.settings.retakeOn", {
              date: retakeAt.toLocaleDateString(i18n.language),
            })}
          </p>
        ) : overview?.invite_available ? (
          <Link to="/placement" className={linkClass}>
            {t("placement.settings.start")}
          </Link>
        ) : null}
      </CardFooter>
    </Card>
  );
};

function isExam(value: string | undefined): value is Exam {
  return EXAMS.some((exam) => exam === value);
}

function isLevel(value: string | null | undefined): value is Level {
  return LEVELS.some((level) => level === value);
}

const LearningProfileForm: React.FC<{ initial: LearningProfile | null }> = ({
  initial,
}) => {
  const { t } = useTranslation();
  const saveProfile = useUpdateLearningProfile();
  const [exam, setExam] = useState<Exam>(isExam(initial?.target_exam) ? initial.target_exam : "none");
  const [target, setTarget] = useState<Level | null>(
    isLevel(initial?.target_level) ? initial.target_level : null,
  );
  const [minutes, setMinutes] = useState<number>(initial?.weekly_minutes_goal ?? 90);
  const [status, setStatus] = useState<"idle" | "saved" | "failed">("idle");

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setStatus("idle");
    try {
      await saveProfile.mutateAsync({
        declared_level: isLevel(initial?.declared_level) ? initial.declared_level : null,
        target_level: target,
        target_exam: exam,
        weekly_minutes_goal: Math.min(MAX_WEEKLY_MINUTES, Math.max(MIN_WEEKLY_MINUTES, minutes)),
        motivations: initial?.motivations ?? [],
      });
      setStatus("saved");
    } catch {
      setStatus("failed");
    }
  };

  return (
    <Card className="border-border bg-surface-card">
      <form onSubmit={(event) => void save(event)}>
        <CardHeader className="space-y-1">
          <div className="flex items-center gap-2">
            <Target className="h-5 w-5 text-primary" aria-hidden="true" />
            <CardTitle className="text-lg font-bold text-text">{t("profile.title")}</CardTitle>
          </div>
          <CardDescription className="text-sm text-text-muted">{t("profile.subtitle")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <p className="text-sm font-semibold text-text">{t("profile.examLabel")}</p>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-3" role="group" aria-label={t("profile.examLabel")}>
              {EXAMS.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={exam === value}
                  onClick={() => setExam(value)}
                  className={choiceClass(exam === value)}
                >
                  {value === "none"
                    ? t("welcome.exam.none")
                    : value === "ielts"
                      ? t("welcome.exam.ielts")
                      : t("welcome.exam.toeic")}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-2">
            <p className="text-sm font-semibold text-text">{t("profile.targetLabel")}</p>
            <div className="grid grid-cols-3 gap-2 sm:grid-cols-6" role="group" aria-label={t("profile.targetLabel")}>
              {LEVELS.map((level) => (
                <button
                  key={level}
                  type="button"
                  aria-pressed={target === level}
                  onClick={() => setTarget(level)}
                  className={choiceClass(target === level)}
                >
                  {level}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="weekly-minutes" className="text-sm font-semibold text-text">
              {t("profile.minutesLabel")}
            </Label>
            <div className="flex flex-wrap items-center gap-2">
              <Input
                id="weekly-minutes"
                type="number"
                inputMode="numeric"
                min={MIN_WEEKLY_MINUTES}
                max={MAX_WEEKLY_MINUTES}
                value={minutes}
                onChange={(event) => setMinutes(Number.parseInt(event.target.value, 10) || MIN_WEEKLY_MINUTES)}
                className="min-h-[44px] w-32 text-base"
              />
              {MINUTE_PRESETS.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  aria-pressed={minutes === preset}
                  onClick={() => setMinutes(preset)}
                  className={choiceClass(minutes === preset)}
                >
                  {t("profile.minutesPreset", { minutes: preset })}
                </button>
              ))}
            </div>
          </div>
          {status !== "idle" && (
            <p
              role={status === "failed" ? "alert" : "status"}
              className={cn("text-sm", status === "failed" ? "text-danger" : "text-success")}
            >
              {status === "saved" ? t("profile.saved") : t("profile.saveFailed")}
            </p>
          )}
        </CardContent>
        <CardFooter>
          <Button type="submit" disabled={saveProfile.isPending} className="min-h-[44px] text-base">
            {saveProfile.isPending ? t("profile.saving") : t("profile.save")}
          </Button>
        </CardFooter>
      </form>
    </Card>
  );
};

/** Settings: the placement result with its retake date, and the learning profile. */
export const LearningProfileSettings: React.FC = () => {
  const profile = useLearningProfile();
  const overview = usePlacementOverview();

  if (profile.isLoading || overview.isLoading) {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-primary" aria-hidden="true" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <PlacementSummary
        overview={overview.data}
        declaredLevel={profile.data?.declared_level ?? null}
      />
      <LearningProfileForm
        key={profile.data?.updated_at ?? "new"}
        initial={profile.data ?? null}
      />
    </div>
  );
};
