import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import {
  AlertCircle,
  CheckCircle2,
  Layers,
  Loader2,
  Play,
  Sparkles,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/api/client";
import { cn } from "@/lib/utils";
import {
  examApi,
  useExamVersions,
  type ExamVersion,
  type MockTest,
} from "../api/examApi";

/**
 * Compose and sit a mock test from the bank (WO 21 Stage E).
 *
 * The composition is the server's: it draws published, unseen-preferring
 * activities per part and stores them, so a retake replays exactly the same
 * test and only "new test" draws again (BR-EXAM-12). A bank that cannot fill
 * a part refuses with that part's name rather than handing back a shorter
 * test.
 */

const MODES = ["fixed", "random", "weak_topic", "custom"] as const;
type MockMode = (typeof MODES)[number];

function problemDetail(err: unknown): string | undefined {
  if (!(err instanceof ApiError)) return undefined;
  return err.problem.detail ?? err.problem.title;
}

function modeLabelKey(mode: MockMode): string {
  switch (mode) {
    case "fixed":
      return "mockTest.modeFixed";
    case "random":
      return "mockTest.modeRandom";
    case "weak_topic":
      return "mockTest.modeWeak";
    case "custom":
      return "mockTest.modeCustom";
  }
}

function modeHintKey(mode: MockMode): string {
  switch (mode) {
    case "fixed":
      return "mockTest.modeFixedHint";
    case "random":
      return "mockTest.modeRandomHint";
    case "weak_topic":
      return "mockTest.modeWeakHint";
    case "custom":
      return "mockTest.modeCustomHint";
  }
}

/** A version's own title, or its code when it was seeded without one. */
function versionLabel(version: ExamVersion): string {
  return version.title || version.code;
}

export function MockTestComposer(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const versions = useExamVersions();

  const [versionId, setVersionId] = useState("");
  const [blueprintId, setBlueprintId] = useState("");
  const [mode, setMode] = useState<MockMode>("random");
  const [chosenParts, setChosenParts] = useState<number[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [composed, setComposed] = useState<MockTest | null>(null);
  const [isComposing, setIsComposing] = useState(false);
  const [isStarting, setIsStarting] = useState(false);

  const version = useMemo(() => {
    const items = versions.data?.items ?? [];
    return (
      items.find((v) => v.id === versionId) ??
      // A version with no blueprint cannot be composed from, so opening on one
      // would offer a disabled button as the first thing a learner sees.
      items.find((v) => (v.blueprints ?? []).length > 0) ??
      items[0]
    );
  }, [versions.data, versionId]);

  const blueprint = useMemo(() => {
    const list = version?.blueprints ?? [];
    return list.find((b) => b.id === blueprintId) ?? list[0];
  }, [version, blueprintId]);

  const compose = async () => {
    if (!blueprint) return;
    setIsComposing(true);
    setError(null);
    setComposed(null);
    try {
      const test = await examApi.composeMockTest({
        blueprint_id: blueprint.id,
        mode,
        ...(mode === "custom" && chosenParts.length > 0
          ? { parts: chosenParts }
          : {}),
      });
      setComposed(test);
    } catch (err) {
      setError(
        problemDetail(err) ??
          t("mockTest.composeFailed", "The test could not be composed."),
      );
    } finally {
      setIsComposing(false);
    }
  };

  const sit = async () => {
    if (!composed) return;
    setIsStarting(true);
    setError(null);
    try {
      const attempt = await examApi.startMockTestAttempt(composed.id);
      void navigate({
        to: "/exams/$attemptId",
        params: { attemptId: attempt.id },
      });
    } catch (err) {
      setError(
        problemDetail(err) ??
          t("mockTest.startFailed", "The sitting could not be started."),
      );
    } finally {
      setIsStarting(false);
    }
  };

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

  if (versions.isError || !version) {
    return (
      <div
        role="alert"
        className="flex items-center gap-3 rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger-accent"
      >
        <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
        <p className="text-sm">
          {t("mockTest.versionsError", "Exam versions could not be loaded.")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h1 className="text-2xl font-extrabold tracking-tight text-text">
          {t("mockTest.title", "Mock tests")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "mockTest.subtitle",
            "Compose a test from the published bank, sit it under the real timer, and read the report. A retake replays the same test; a new test draws again.",
          )}
        </p>
      </div>

      <div className="space-y-3 rounded-xl border border-border bg-card p-4">
        <label className="block space-y-1 text-sm">
          <span className="font-medium text-text">
            {t("mockTest.version", "Exam version")}
          </span>
          <select
            value={version.id}
            onChange={(e) => {
              setVersionId(e.target.value);
              setBlueprintId("");
              setChosenParts([]);
              setComposed(null);
            }}
            className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
          >
            {(versions.data?.items ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {versionLabel(item)}
              </option>
            ))}
          </select>
        </label>

        <div className="flex flex-wrap items-center gap-2 text-xs text-text-muted">
          <Badge variant="outline">
            {t("mockTest.totalMinutes", {
              minutes: version.total_minutes,
              defaultValue: `${version.total_minutes} minutes`,
            })}
          </Badge>
          <Badge variant="secondary">
            {t("mockTest.distinctTests", {
              count: version.distinct_tests_possible,
              defaultValue: `${version.distinct_tests_possible} distinct tests available`,
            })}
          </Badge>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label className="space-y-1 text-sm">
            <span className="font-medium text-text">
              {t("mockTest.blueprint", "Blueprint")}
            </span>
            <select
              value={blueprint?.id ?? ""}
              onChange={(e) => {
                setBlueprintId(e.target.value);
                setComposed(null);
              }}
              className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
            >
              {(version.blueprints ?? []).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium text-text">
              {t("mockTest.mode", "Mode")}
            </span>
            <select
              value={mode}
              onChange={(e) => {
                setMode(e.target.value as MockMode);
                setComposed(null);
              }}
              className="h-11 w-full rounded-lg border border-input bg-background px-3 text-base focus:outline-none focus:ring-1 focus:ring-ring"
            >
              {MODES.map((value) => (
                <option key={value} value={value}>
                  {t(modeLabelKey(value), value)}
                </option>
              ))}
            </select>
          </label>
        </div>

        <p className="text-xs text-text-muted">{t(modeHintKey(mode))}</p>

        {mode === "custom" && version.parts.length > 0 && (
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-text">
              {t("mockTest.parts", "Parts")}
            </legend>
            <div className="flex flex-wrap gap-2">
              {version.parts.map((part) => {
                const checked = chosenParts.includes(part.part_number);
                return (
                  <label
                    key={part.part_number}
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
                        setChosenParts((prev) =>
                          checked
                            ? prev.filter((n) => n !== part.part_number)
                            : [...prev, part.part_number],
                        )
                      }
                      className="accent-primary"
                    />
                    {part.section} {part.part_number}
                    <span className="text-xs">({part.kind})</span>
                  </label>
                );
              })}
            </div>
          </fieldset>
        )}

        {error && (
          <p
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-danger/20 bg-danger/10 p-3 text-sm text-danger-accent"
          >
            <AlertCircle
              className="mt-0.5 h-4 w-4 shrink-0"
              aria-hidden="true"
            />
            <span>{error}</span>
          </p>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button
            onClick={() => void compose()}
            isLoading={isComposing}
            disabled={isComposing || !blueprint}
            className="gap-2"
          >
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            {composed
              ? t("mockTest.composeNew", "Compose a new test")
              : t("mockTest.composeBtn", "Compose the test")}
          </Button>
        </div>
      </div>

      {composed && (
        <div className="space-y-3 rounded-xl border border-success/30 bg-success/5 p-4">
          <p className="flex items-center gap-2 text-sm font-medium text-success-accent">
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
            {t("mockTest.composed", "Your test is ready.")}
          </p>
          <ul className="space-y-1 text-sm text-text">
            {composed.composition.map((part) => (
              <li key={part.part_id} className="flex items-center gap-2">
                <Layers
                  className="h-3.5 w-3.5 text-text-muted"
                  aria-hidden="true"
                />
                {t("mockTest.partItems", {
                  count: part.activity_ids.length,
                  defaultValue: `${part.activity_ids.length} items`,
                })}
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => void sit()}
              isLoading={isStarting}
              className="gap-2"
            >
              <Play className="h-4 w-4" aria-hidden="true" />
              {t("mockTest.sitBtn", "Sit this test")}
            </Button>
            <p className="self-center text-xs text-text-muted">
              {t(
                "mockTest.retakeNote",
                "Sitting it again replays this same test; compose a new one to draw different items.",
              )}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
