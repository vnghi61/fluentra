import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { Clock, Compass, Target } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
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
import { IconTile, SettingsCardHeader } from "./SettingsCard";
import { settingsCard } from "./settingsStyles";

const EXAMS = ["none", "ielts", "toeic"] as const;
const LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"] as const;
const MINUTE_PRESETS = [60, 90, 150, 300] as const;
const MIN_WEEKLY_MINUTES = 15;
const MAX_WEEKLY_MINUTES = 10080;

type Exam = (typeof EXAMS)[number];
type Level = (typeof LEVELS)[number];

const MEASURED = [
  "vocabulary",
  "grammar",
  "reading",
  "listening",
  "writing",
  "speaking",
] as const;

const linkClass =
  "inline-flex min-h-[44px] w-full items-center justify-center whitespace-nowrap rounded-xl bg-primary px-5 text-sm font-semibold text-primary-fg hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 sm:w-auto";

const choiceClass = (selected: boolean) =>
  cn(
    "min-h-[44px] min-w-0 whitespace-nowrap rounded-[10px] border px-2 py-2 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none",
    selected
      ? "border-primary bg-primary/10 text-primary-accent"
      : "border-border-subtle bg-surface-muted/60 text-text hover:border-border",
  );

/** The placement result as the page's hero: level, per-skill bands, retake. */
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

  const action = overview?.active_session ? (
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
  ) : null;

  return (
    <section
      aria-labelledby="placement-summary-title"
      className="@container rounded-2xl border border-border-subtle bg-gradient-to-br from-primary/10 via-surface-card to-surface-card p-4 shadow-[var(--shadow-card)] sm:p-6"
    >
      <div className="flex flex-col gap-4 @2xl:flex-row @2xl:items-center">
        <div className="flex min-w-0 flex-1 items-start gap-4">
          <IconTile icon={Compass} size="lg" />
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h2
                id="placement-summary-title"
                className="text-lg font-semibold leading-snug text-text md:text-xl"
              >
                {t("placement.settings.title")}
              </h2>
              {result && (
                <Badge variant="primary" className="text-sm">
                  {result.level}
                </Badge>
              )}
            </div>
            <p className="text-sm text-text-muted">
              {result
                ? t("placement.settings.placedOn", {
                    date: new Date(result.taken_at).toLocaleDateString(
                      i18n.language,
                    ),
                  })
                : t("placement.settings.noResult")}
            </p>
            {!result && declaredLevel && (
              <p className="text-sm text-text-muted">
                {t("placement.settings.declared", { level: declaredLevel })}
              </p>
            )}
          </div>
        </div>
        {action && <div className="shrink-0">{action}</div>}
      </div>

      {result && (
        <dl className="mt-5 grid grid-cols-2 gap-2 @sm:grid-cols-3 @3xl:grid-cols-6">
          {MEASURED.map((skill) => (
            <div
              key={skill}
              className="rounded-xl border border-border-subtle bg-surface-card/80 px-3 py-2"
            >
              <dt className="truncate text-sm text-text-muted">
                {t(`skills.${skill}`)}
              </dt>
              <dd className="text-base font-bold text-text">
                {result.per_skill[skill]?.band ??
                  t("placement.result.notMeasured")}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </section>
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
  const [exam, setExam] = useState<Exam>(
    isExam(initial?.target_exam) ? initial.target_exam : "none",
  );
  const [target, setTarget] = useState<Level | null>(
    isLevel(initial?.target_level) ? initial.target_level : null,
  );
  const [minutes, setMinutes] = useState<number>(
    initial?.weekly_minutes_goal ?? 90,
  );
  const [custom, setCustom] = useState<boolean>(
    !MINUTE_PRESETS.some((preset) => preset === minutes),
  );
  const [status, setStatus] = useState<"idle" | "saved" | "failed">("idle");
  const perDay = Math.round(
    Math.min(MAX_WEEKLY_MINUTES, Math.max(MIN_WEEKLY_MINUTES, minutes)) / 7,
  );

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setStatus("idle");
    try {
      await saveProfile.mutateAsync({
        declared_level: isLevel(initial?.declared_level)
          ? initial.declared_level
          : null,
        target_level: target,
        target_exam: exam,
        weekly_minutes_goal: Math.min(
          MAX_WEEKLY_MINUTES,
          Math.max(MIN_WEEKLY_MINUTES, minutes),
        ),
        motivations: initial?.motivations ?? [],
      });
      setStatus("saved");
    } catch {
      setStatus("failed");
    }
  };

  return (
    <form onSubmit={(event) => void save(event)} className="space-y-4">
      <div className="@container">
        <div className="grid items-stretch gap-4 @3xl:grid-cols-2">
          <section className={`${settingsCard} flex flex-col gap-5`}>
            <SettingsCardHeader
              icon={Target}
              title={t("profile.title")}
              description={t("profile.subtitle")}
            />
            <div className="space-y-2">
              <p className="text-sm font-semibold text-text">
                {t("profile.examLabel")}
              </p>
              <div
                className="grid grid-cols-1 gap-2 min-[360px]:grid-cols-3"
                role="group"
                aria-label={t("profile.examLabel")}
              >
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
              <p className="text-sm font-semibold text-text">
                {t("profile.targetLabel")}
              </p>
              <div
                className="grid grid-cols-3 gap-2 sm:grid-cols-6"
                role="group"
                aria-label={t("profile.targetLabel")}
              >
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
          </section>

          <section className={`${settingsCard} flex flex-col gap-5`}>
            <SettingsCardHeader
              icon={Clock}
              title={t("settings.learning.weeklyTitle", "Weekly study time")}
              description={t(
                "settings.learning.weeklyDesc",
                "Your weekly plan is built to fit this much time.",
              )}
            />
            <div className="space-y-2">
              <p className="text-sm font-semibold text-text">
                {t("profile.minutesLabel")}
              </p>
              {/* One source of truth: a preset, or "Custom" which reveals the
                  number box. Showing both at once let "90" and "90 min" disagree. */}
              <div
                className="grid grid-cols-3 gap-2 sm:grid-cols-5"
                role="group"
                aria-label={t("profile.minutesLabel")}
              >
                {MINUTE_PRESETS.map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    aria-pressed={!custom && minutes === preset}
                    onClick={() => {
                      setCustom(false);
                      setMinutes(preset);
                    }}
                    className={choiceClass(!custom && minutes === preset)}
                  >
                    {t("profile.minutesPreset", { minutes: preset })}
                  </button>
                ))}
                <button
                  type="button"
                  aria-pressed={custom}
                  onClick={() => setCustom(true)}
                  className={choiceClass(custom)}
                >
                  {t("profile.minutesCustom", "Custom")}
                </button>
              </div>
              {custom && (
                <div className="flex items-center gap-2 pt-1">
                  <Input
                    id="weekly-minutes"
                    type="number"
                    inputMode="numeric"
                    min={MIN_WEEKLY_MINUTES}
                    max={MAX_WEEKLY_MINUTES}
                    value={minutes}
                    onChange={(event) =>
                      setMinutes(
                        Number.parseInt(event.target.value, 10) ||
                          MIN_WEEKLY_MINUTES,
                      )
                    }
                    aria-label={t("profile.minutesLabel")}
                    className="min-h-[44px] w-32 text-base"
                  />
                  <span className="text-sm text-text-muted">
                    {t("profile.minutesPerWeek", "min / week")}
                  </span>
                </div>
              )}
            </div>
            <p className="mt-auto rounded-xl bg-surface-muted/60 px-3 py-2.5 text-sm text-text">
              {t("settings.learning.perDay", {
                minutes: perDay,
                defaultValue: `About ${perDay} min a day`,
              })}
            </p>
          </section>
        </div>
      </div>

      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-end">
        {status !== "idle" && (
          <p
            role={status === "failed" ? "alert" : "status"}
            className={cn(
              "text-sm",
              status === "failed"
                ? "text-danger-accent"
                : "text-success-accent",
            )}
          >
            {status === "saved" ? t("profile.saved") : t("profile.saveFailed")}
          </p>
        )}
        <Button
          type="submit"
          disabled={saveProfile.isPending}
          className="w-full sm:w-auto"
        >
          {saveProfile.isPending ? t("profile.saving") : t("profile.save")}
        </Button>
      </div>
    </form>
  );
};

/** Settings: the placement result with its retake date, and the learning profile. */
export const LearningProfileSettings: React.FC = () => {
  const profile = useLearningProfile();
  const overview = usePlacementOverview();

  if (profile.isLoading || overview.isLoading) {
    return (
      <div className="space-y-4" data-testid="learning-profile-skeleton">
        <Skeleton className="h-32 w-full rounded-2xl" />
        <div className="grid gap-4 lg:grid-cols-2">
          <Skeleton className="h-72 w-full rounded-2xl" />
          <Skeleton className="h-72 w-full rounded-2xl" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
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
