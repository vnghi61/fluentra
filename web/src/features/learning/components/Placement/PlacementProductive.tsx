import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { SpeakingRecorder } from "@/features/exam/components/SpeakingRecorder";
import {
  placementApi,
  placementProblemCode,
  type PlacementProductiveItem,
  type PlacementResponse,
  type PlacementSession,
} from "../../api/placement";
import { formatClock, parseItem, wordCount } from "./placementItem";
import { useCountdown } from "./useCountdown";

interface ProductiveProps {
  session: PlacementSession;
  onSession: (session: PlacementSession) => void;
  onReload: () => void;
}

const ProductiveItemView: React.FC<{
  sessionId: string;
  item: PlacementProductiveItem;
  disabled: boolean;
  onSession: (session: PlacementSession) => void;
  onReload: () => void;
}> = ({ sessionId, item, disabled, onSession, onReload }) => {
  const { t } = useTranslation();
  const parsed = useMemo(() => parseItem(item.config), [item.config]);
  const [text, setText] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [key] = useState(() => crypto.randomUUID());
  const words = wordCount(text);

  const send = async (response: PlacementResponse) => {
    setPending(true);
    setError(null);
    try {
      onSession(
        await placementApi.answer(sessionId, item.activity_id, response, key),
      );
    } catch (err: unknown) {
      if (placementProblemCode(err) === "PLACEMENT_SESSION_EXPIRED") {
        onReload();
      } else {
        setError(t("placement.productive.answerFailed"));
      }
    } finally {
      setPending(false);
    }
  };

  const heading =
    item.skill === "writing"
      ? t("placement.productive.writingTitle")
      : t("placement.productive.speakingTitle");

  if (item.status !== "not_answered") {
    return (
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-border-subtle p-3">
        <span className="text-sm font-medium text-text">{heading}</span>
        {item.status === "graded" && item.band ? (
          <Badge variant="success">
            {t("placement.productive.itemBand", { band: item.band })}
          </Badge>
        ) : item.status === "failed" ? (
          <Badge variant="danger">{t("placement.productive.itemFailed")}</Badge>
        ) : (
          <Badge variant="secondary">
            {t("placement.productive.itemGrading")}
          </Badge>
        )}
      </div>
    );
  }

  const fieldId = `placement-writing-${item.activity_id}`;
  return (
    <div className="space-y-3 rounded-xl border border-border-subtle p-4">
      <h3 className="text-base font-semibold text-text">{heading}</h3>
      {item.skill === "writing" ? (
        <>
          <p className="text-base text-text">{parsed.prompt}</p>
          <label htmlFor={fieldId} className="sr-only">
            {t("placement.productive.writingLabel")}
          </label>
          <textarea
            id={fieldId}
            rows={6}
            value={text}
            onChange={(e) => setText(e.target.value)}
            disabled={disabled || pending}
            className="min-h-[160px] w-full rounded-xl border border-border bg-surface-base p-3 text-base text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          />
          <div className="flex flex-wrap justify-between gap-2 text-sm text-text-muted">
            <span>{t("placement.productive.wordCount", { words })}</span>
            <span>
              {t("placement.productive.wordTarget", {
                min: parsed.minWords || 60,
              })}
            </span>
          </div>
          <Button
            type="button"
            onClick={() => void send({ text_answer: text.trim() })}
            disabled={disabled || pending || words === 0}
            className="min-h-[44px] text-base"
          >
            {t("placement.productive.submitWriting")}
          </Button>
        </>
      ) : (
        <SpeakingRecorder
          taskType="respond"
          promptText={parsed.prompt}
          speakingTimeSeconds={parsed.speakingSeconds || 45}
          mode="exam"
          onRecordingComplete={(objectKey) =>
            void send({ audio_object_key: objectKey })
          }
        />
      )}
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
};

const ProductiveRunner: React.FC<ProductiveProps> = ({
  session,
  onSession,
  onReload,
}) => {
  const { t } = useTranslation();
  const answered = session.productive_items.filter(
    (item) => item.status !== "not_answered",
  ).length;
  const remaining = useCountdown(
    session.productive_remaining_seconds,
    `${session.id}:${answered}`,
  );

  return (
    <div className="space-y-4">
      <p className="font-mono text-sm text-text-muted">
        {t("placement.productive.timeLeft", { time: formatClock(remaining) })}
      </p>
      {session.productive_items.map((item) => (
        <ProductiveItemView
          key={item.activity_id}
          sessionId={session.id}
          item={item}
          disabled={remaining === 0}
          onSession={onSession}
          onReload={onReload}
        />
      ))}
      {remaining === 0 && (
        <Button
          type="button"
          variant="outline"
          onClick={onReload}
          className="min-h-[44px] text-base"
        >
          {t("placement.productive.timeUp")}
        </Button>
      )}
    </div>
  );
};

/**
 * The optional writing and speaking part, offered after the result. It can be
 * taken now or later; each answer is graded asynchronously and its band joins
 * the result when it is ready.
 */
export const PlacementProductive: React.FC<ProductiveProps> = ({
  session,
  onSession,
  onReload,
}) => {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const perSkill = session.result?.per_skill ?? {};
  const status = session.productive_status;

  const act = async (skip: boolean) => {
    setPending(true);
    setError(null);
    try {
      onSession(await placementApi.productive(session.id, skip));
    } catch {
      setError(t("placement.productive.failed"));
    } finally {
      setPending(false);
    }
  };

  return (
    <Card className="border-border bg-surface-card">
      <CardHeader className="space-y-1">
        <CardTitle className="text-lg font-bold text-text">
          {t("placement.productive.title")}
        </CardTitle>
        <CardDescription className="text-sm text-text-muted">
          {status === "offered"
            ? t("placement.productive.notMeasured")
            : status === "skipped"
              ? t("placement.productive.skipped")
              : status === "submitted"
                ? t("placement.productive.grading")
                : t("placement.productive.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <dl className="grid grid-cols-2 gap-3">
          {(["writing", "speaking"] as const).map((skill) => (
            <div
              key={skill}
              className="rounded-xl border border-border-subtle bg-surface-base p-3"
            >
              <dt className="text-xs text-text-muted">{t(`skills.${skill}`)}</dt>
              <dd className="text-lg font-bold text-text">
                {perSkill[skill]?.band ?? t("placement.result.notMeasured")}
              </dd>
            </div>
          ))}
        </dl>

        {(status === "offered" || status === "skipped") && (
          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              onClick={() => void act(false)}
              disabled={pending}
              className="min-h-[44px] text-base"
            >
              {t("placement.productive.takeNow")}
            </Button>
            {status === "offered" && (
              <Button
                type="button"
                variant="outline"
                onClick={() => void act(true)}
                disabled={pending}
                className="min-h-[44px] text-base"
              >
                {t("placement.productive.later")}
              </Button>
            )}
          </div>
        )}
        {status === "in_progress" && (
          <ProductiveRunner
            session={session}
            onSession={onSession}
            onReload={onReload}
          />
        )}
        {status === "submitted" && (
          <Button
            type="button"
            variant="outline"
            onClick={onReload}
            className="min-h-[44px] text-base"
          >
            {t("placement.productive.refresh")}
          </Button>
        )}
        {error && (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        )}
      </CardContent>
    </Card>
  );
};
