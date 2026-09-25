import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import {
  AlertCircle,
  ArrowLeft,
  BookOpenCheck,
  ChevronRight,
  Dumbbell,
  FileText,
  Loader2,
  Play,
  Shuffle,
  Sparkles,
} from "lucide-react";

import { ApiError } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  examApi,
  useExamVersionTests,
  useExamVersions,
  type ExamVersion,
  type FixedTestSummary,
} from "../api/examApi";
import { MockTestComposer } from "./MockTestComposer";
import type { ExamMode, StartSittingRequest } from "../types";

/**
 * The exam hub (WO 22 Stage L): exams → a version's numbered tests → one test,
 * sat full and timed or practised with chosen sections and no limit. A random
 * test composes a fresh one and starts it straight away.
 */

/** The section positions in the order a sitting holds them. */
const SECTION_ORDER: Record<string, number> = {
  listening: 1,
  reading: 2,
  writing: 3,
  speaking: 4,
};

function problemDetail(err: unknown): string | undefined {
  if (!(err instanceof ApiError)) return undefined;
  return err.problem.detail ?? err.problem.title;
}

function versionLabel(version: ExamVersion): string {
  return version.title || version.code;
}

/** The distinct sections a version holds, by position, in sitting order. */
function versionSections(
  version: ExamVersion,
): { position: number; skill: string }[] {
  const seen = new Map<number, string>();
  for (const part of version.parts ?? []) {
    const position = SECTION_ORDER[part.section] ?? 0;
    if (position > 0 && !seen.has(position)) seen.set(position, part.section);
  }
  return [...seen.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([position, skill]) => ({ position, skill }));
}

/** A version's total question count, from its parts. */
function questionCount(version: ExamVersion): number {
  return (version.parts ?? []).reduce(
    (sum, part) => sum + part.question_count,
    0,
  );
}

export function ExamHub(): React.JSX.Element {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<ExamVersion | null>(null);
  const [customOpen, setCustomOpen] = useState(false);

  if (customOpen) {
    return (
      <div className="space-y-4">
        <BackButton
          label={t("exam.hub.backToTests", "Back to the tests")}
          onClick={() => setCustomOpen(false)}
        />
        <MockTestComposer />
      </div>
    );
  }

  if (selected) {
    return (
      <VersionTests
        version={selected}
        onBack={() => setSelected(null)}
        onCustom={() => setCustomOpen(true)}
      />
    );
  }

  return <VersionList onSelect={setSelected} />;
}

function BackButton({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void;
}): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex min-h-[44px] items-center gap-1.5 text-sm font-medium text-text-muted hover:text-text"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {label}
    </button>
  );
}

function VersionList({
  onSelect,
}: {
  onSelect: (version: ExamVersion) => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const versions = useExamVersions();

  if (versions.isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2
          className="h-6 w-6 animate-spin text-primary"
          aria-hidden="true"
        />
      </div>
    );
  }

  const items = versions.data?.items ?? [];
  if (versions.isError || items.length === 0) {
    return (
      <div
        role="alert"
        className="flex items-center gap-3 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger-accent"
      >
        <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
        <p className="text-sm">
          {t("exam.hub.versionsError", "Exams could not be loaded.")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h1 className="text-2xl font-extrabold tracking-tight text-text">
          {t("exam.hub.examsTitle", "Exams")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "exam.hub.examsSubtitle",
            "Pick an exam, then a numbered test or a random one, and sit it full and timed or practise a section at a time.",
          )}
        </p>
      </div>

      <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {items.map((version) => {
          const count = questionCount(version);
          return (
            <li key={version.id}>
              <button
                type="button"
                onClick={() => onSelect(version)}
                className="flex w-full items-center gap-4 rounded-2xl border border-border bg-card p-4 text-left transition-colors hover:border-primary/40 hover:bg-surface-muted"
              >
                <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
                  <BookOpenCheck className="h-5 w-5" aria-hidden="true" />
                </span>
                <span className="min-w-0 flex-1 space-y-1">
                  <span className="block truncate font-bold text-text">
                    {versionLabel(version)}
                  </span>
                  <span className="block text-xs text-text-muted">
                    {t("exam.hub.format", {
                      count,
                      minutes: version.total_minutes,
                      defaultValue: `${count} questions · ${version.total_minutes} min`,
                    })}
                  </span>
                  <span className="flex flex-wrap gap-1.5 pt-0.5">
                    <Badge variant="secondary">
                      {t("exam.hub.testCount", {
                        count: version.fixed_test_count,
                        defaultValue: `${version.fixed_test_count} tests`,
                      })}
                    </Badge>
                    {version.best_score !== undefined && (
                      <Badge variant="primary">
                        {t("exam.hub.bestScore", {
                          score: Math.round(version.best_score),
                          defaultValue: `Best ${Math.round(version.best_score)}%`,
                        })}
                      </Badge>
                    )}
                  </span>
                </span>
                <ChevronRight
                  className="h-5 w-5 shrink-0 text-text-muted"
                  aria-hidden="true"
                />
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function VersionTests({
  version,
  onBack,
  onCustom,
}: {
  version: ExamVersion;
  onBack: () => void;
  onCustom: () => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const tests = useExamVersionTests(version.id);
  const [randomBusy, setRandomBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const blueprint = version.blueprints?.[0];

  const startRandom = async () => {
    if (!blueprint) return;
    setRandomBusy(true);
    setError(null);
    try {
      const test = await examApi.composeMockTest({
        blueprint_id: blueprint.id,
        mode: "random",
      });
      const attempt = await examApi.startMockTestAttempt(test.id, {
        mode: "exam",
      });
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err) {
      setError(
        problemDetail(err) ??
          t("exam.hub.randomFailed", "A random test could not be started."),
      );
    } finally {
      setRandomBusy(false);
    }
  };

  const items = tests.data?.items ?? [];

  return (
    <div className="space-y-4">
      <BackButton
        label={t("exam.hub.backToExams", "All exams")}
        onClick={onBack}
      />
      <div className="space-y-1">
        <h1 className="text-2xl font-extrabold tracking-tight text-text">
          {versionLabel(version)}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "exam.hub.testsSubtitle",
            "Choose a test, or let one be drawn for you.",
          )}
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button
          onClick={() => void startRandom()}
          isLoading={randomBusy}
          disabled={randomBusy || !blueprint}
          className="gap-2"
        >
          <Shuffle className="h-4 w-4" aria-hidden="true" />
          {t("exam.hub.random", "Random test")}
        </Button>
        <Button variant="outline" onClick={onCustom} className="gap-2">
          <Sparkles className="h-4 w-4" aria-hidden="true" />
          {t("exam.hub.custom", "Build a custom test")}
        </Button>
      </div>

      {error && (
        <p
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-danger/20 bg-danger/10 p-3 text-sm text-danger-accent"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}

      {tests.isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2
            className="h-6 w-6 animate-spin text-primary"
            aria-hidden="true"
          />
        </div>
      ) : items.length === 0 ? (
        <p className="rounded-xl border border-border-subtle bg-surface-muted p-4 text-sm text-text-muted">
          {t(
            "exam.hub.empty",
            "No tests yet — this exam is being written. Try a random test.",
          )}
        </p>
      ) : (
        <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {items.map((test) => (
            <TestCard key={test.id} test={test} version={version} />
          ))}
        </ul>
      )}
    </div>
  );
}

function TestCard({
  test,
  version,
}: {
  test: FixedTestSummary;
  version: ExamVersion;
}): React.JSX.Element {
  const { t } = useTranslation();
  const [sheetOpen, setSheetOpen] = useState(false);
  const attempt = test.latest_attempt;

  return (
    <li className="rounded-2xl border border-border bg-card p-4">
      <div className="flex items-center gap-4">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-surface-muted text-text-muted">
          <FileText className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate font-bold text-text">{test.title}</p>
          <p className="text-xs text-text-muted">
            {t("exam.hub.testFormat", {
              count: test.question_count,
              minutes: test.minutes,
              defaultValue: `${test.question_count} questions · ${test.minutes} min`,
            })}
          </p>
          {attempt && (
            <p className="pt-0.5 text-xs font-medium text-success-accent">
              {attempt.score !== undefined && attempt.score !== null
                ? t("exam.hub.doneScore", {
                    score: attempt.score,
                    defaultValue: `Done · ${attempt.score}`,
                  })
                : t("exam.hub.done", "Done")}
            </p>
          )}
        </div>
        <Button
          size="sm"
          variant={attempt ? "outline" : "primary"}
          onClick={() => setSheetOpen((open) => !open)}
          className="shrink-0"
        >
          {t("exam.hub.options", "Options")}
        </Button>
      </div>
      {sheetOpen && <SitSheet version={version} test={test} />}
    </li>
  );
}

/** The "Thi thử" / "Luyện tập" choice, with practice's settings. */
function SitSheet({
  version,
  test,
}: {
  version: ExamVersion;
  test: FixedTestSummary;
}): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const sections = useMemo(() => versionSections(version), [version]);
  const [mode, setMode] = useState<ExamMode>("exam");
  const [chosenSections, setChosenSections] = useState<number[]>([]);
  const [unlimited, setUnlimited] = useState(false);
  const [duration, setDuration] = useState(60);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const start = async () => {
    setBusy(true);
    setError(null);
    const req: StartSittingRequest =
      mode === "exam"
        ? { mode: "exam" }
        : {
            mode: "practice",
            unlimited,
            ...(unlimited ? {} : { chosen_duration_minutes: duration }),
            ...(chosenSections.length > 0 ? { sections: chosenSections } : {}),
          };
    try {
      const attempt = await examApi.startMockTestAttempt(test.id, req);
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err) {
      setError(
        problemDetail(err) ??
          t("exam.hub.startFailed", "The sitting could not be started."),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-3 space-y-3 rounded-xl border border-border-subtle bg-surface-muted/60 p-3">
      <div className="flex gap-2">
        {(
          [
            ["exam", t("exam.hub.examMode", "Exam (timed)"), Play],
            ["practice", t("exam.hub.practiceMode", "Practice"), Dumbbell],
          ] as const
        ).map(([key, label, Icon]) => (
          <button
            key={key}
            type="button"
            onClick={() => setMode(key)}
            className={cn(
              "flex min-h-[44px] flex-1 items-center justify-center gap-2 rounded-lg border px-3 text-sm font-medium",
              mode === key
                ? "border-primary bg-primary/10 text-primary-accent"
                : "border-border bg-card text-text-muted hover:text-text",
            )}
          >
            <Icon className="h-4 w-4" aria-hidden="true" />
            {label}
          </button>
        ))}
      </div>

      {mode === "practice" && (
        <div className="space-y-3">
          <fieldset className="space-y-2">
            <legend className="text-xs font-medium text-text-muted">
              {t("exam.hub.sections", "Sections")}
            </legend>
            <div className="flex flex-wrap gap-2">
              {sections.map((section) => {
                const checked = chosenSections.includes(section.position);
                return (
                  <label
                    key={section.position}
                    className={cn(
                      "flex min-h-[44px] cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm",
                      checked
                        ? "border-primary bg-primary/5 text-text"
                        : "border-border-subtle text-text-muted",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() =>
                        setChosenSections((prev) =>
                          checked
                            ? prev.filter((n) => n !== section.position)
                            : [...prev, section.position],
                        )
                      }
                      className="accent-primary"
                    />
                    {t(`exam.sections.${section.skill}`, section.skill)}
                  </label>
                );
              })}
            </div>
          </fieldset>

          <label className="flex min-h-[44px] cursor-pointer items-center gap-2 text-sm text-text">
            <input
              type="checkbox"
              checked={unlimited}
              onChange={(e) => setUnlimited(e.target.checked)}
              className="accent-primary"
            />
            {t("exam.hub.noLimit", "No time limit")}
          </label>

          {!unlimited && (
            <label className="flex items-center gap-3 text-sm text-text">
              {t("exam.hub.durationMinutes", "Minutes")}
              <input
                type="number"
                min={10}
                max={180}
                value={duration}
                onChange={(e) => setDuration(Number(e.target.value))}
                className="h-11 w-24 rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </label>
          )}
        </div>
      )}

      {error && (
        <p role="alert" className="text-sm text-danger-accent">
          {error}
        </p>
      )}

      <div className="flex justify-end">
        <Button
          onClick={() => void start()}
          isLoading={busy}
          disabled={busy}
          className="gap-2"
        >
          <Play className="h-4 w-4" aria-hidden="true" />
          {t("exam.hub.go", "Start")}
        </Button>
      </div>
    </div>
  );
}
