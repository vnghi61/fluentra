import React, { useRef, useState } from "react";
import { Clock } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  placementApi,
  placementProblemCode,
  type PlacementSession,
} from "../../api/placement";
import { PlacementItemView } from "./PlacementItemView";
import {
  emptyAnswer,
  formatClock,
  isComplete,
  toResponse,
  type ItemAnswer,
} from "./placementItem";
import { useCountdown } from "./useCountdown";

export interface PlacementRunnerProps {
  session: PlacementSession;
  onSession: (session: PlacementSession) => void;
  onReload: () => void;
}

/** Answers that mean the session moved on without this screen. */
const RELOAD_CODES = new Set([
  "PLACEMENT_SESSION_EXPIRED",
  "PLACEMENT_NOT_CURRENT_ITEM",
  "PLACEMENT_SESSION_CHANGED",
]);

/**
 * The adaptive part: one item at a time, the server's clock, the stage as
 * progress, and no going back. At zero it says so and asks the server for the
 * result, which finishes the session with what it has.
 */
export const PlacementRunner: React.FC<PlacementRunnerProps> = ({
  session,
  onSession,
  onReload,
}) => {
  const { t } = useTranslation();
  const item = session.current_item;
  const [answer, setAnswer] = useState<ItemAnswer>(emptyAnswer);
  const [answerFor, setAnswerFor] = useState(item?.activity_id ?? "");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const keys = useRef(new Map<string, string>());
  const remaining = useCountdown(
    session.remaining_seconds,
    `${session.id}:${session.responses}`,
  );

  if (item && item.activity_id !== answerFor) {
    setAnswerFor(item.activity_id);
    setAnswer(emptyAnswer);
  }

  const stageLabels: Record<PlacementSession["stage"], string> = {
    vocabulary_grammar: t("placement.stage.vocabularyGrammar"),
    reading_listening: t("placement.stage.readingListening"),
    refine: t("placement.stage.refine"),
    done: t("placement.stage.done"),
  };
  const progress = Math.min(
    100,
    Math.round((session.responses / Math.max(1, session.max_responses)) * 100),
  );

  const submit = async () => {
    if (!item || submitting) return;
    const key = keys.current.get(item.activity_id) ?? crypto.randomUUID();
    keys.current.set(item.activity_id, key);
    setSubmitting(true);
    setError(null);
    try {
      onSession(
        await placementApi.answer(
          session.id,
          item.activity_id,
          toResponse(item, answer),
          key,
        ),
      );
    } catch (err: unknown) {
      const code = placementProblemCode(err);
      if (code !== undefined && RELOAD_CODES.has(code)) {
        onReload();
      } else {
        setError(t("placement.runner.answerFailed"));
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-surface-base">
      <header className="sticky top-0 z-20 border-b border-border bg-surface-card px-4 py-3">
        <div className="mx-auto flex max-w-3xl items-center justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <p className="truncate text-sm font-semibold text-text">
              {stageLabels[session.stage]}
            </p>
            <div
              className="h-1.5 w-40 max-w-full overflow-hidden rounded-full bg-border-subtle"
              role="progressbar"
              aria-label={t("placement.runner.progress")}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress}
            >
              <div
                className="h-full rounded-full bg-primary"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
          <p
            className={cn(
              "flex items-center gap-1.5 rounded-full border px-3 py-1 font-mono text-sm font-bold",
              remaining < 120
                ? "border-danger text-danger"
                : "border-border text-text",
            )}
          >
            <Clock className="h-4 w-4" aria-hidden="true" />
            <span className="sr-only">{t("placement.runner.timeLeft")}</span>
            <span>{formatClock(remaining)}</span>
          </p>
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 space-y-6 px-4 py-6 md:py-8">
        {item && (
          <PlacementItemView
            key={item.activity_id}
            item={item}
            sessionId={session.id}
            answer={answer}
            onChange={setAnswer}
          />
        )}
        {error && (
          <p
            role="alert"
            className="rounded-lg bg-danger/10 p-3 text-sm text-danger"
          >
            {error}
          </p>
        )}
        <div className="flex flex-col-reverse gap-3 border-t border-border-subtle pt-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-text-muted">
            {t("placement.runner.noGoingBack")}
          </p>
          <Button
            type="button"
            onClick={() => void submit()}
            disabled={!item || !isComplete(item, answer) || submitting}
            className="min-h-[44px] px-8 text-base"
          >
            {submitting
              ? t("placement.runner.submitting")
              : t("placement.runner.next")}
          </Button>
        </div>
      </main>

      {remaining === 0 && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="placement-time-up"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 p-4"
        >
          <div className="w-full max-w-sm space-y-4 rounded-2xl border border-border bg-surface-card p-6 text-center shadow-xl">
            <h2 id="placement-time-up" className="text-xl font-bold text-text">
              {t("placement.timeUp.title")}
            </h2>
            <p className="text-sm text-text-muted">
              {t("placement.timeUp.description")}
            </p>
            <Button
              type="button"
              onClick={onReload}
              className="min-h-[44px] w-full text-base"
            >
              {t("placement.timeUp.action")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
};
