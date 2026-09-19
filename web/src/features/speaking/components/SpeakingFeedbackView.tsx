import React, { useMemo, useState } from "react";
import {
  Activity,
  Award,
  ChevronDown,
  ChevronUp,
  Clock,
  Info,
  Mic,
  Sparkles,
  Volume2,
} from "lucide-react";
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
import { cn } from "@/lib/utils";
import type { SpeakingFeedback } from "../types";
import { computeWordDiff, type DiffToken } from "../utils/diff";

export interface SpeakingFeedbackViewProps {
  feedback: SpeakingFeedback;
  taskType?: "read_aloud" | "respond" | undefined;
  referenceText?: string | undefined;
  onContinue?: (() => void) | undefined;
  className?: string | undefined;
}

function getBandColor(band: number): string {
  if (band >= 7.0)
    return "text-emerald-500 bg-emerald-500/10 border-emerald-500/30";
  if (band >= 6.0) return "text-primary bg-primary/10 border-primary/30";
  if (band >= 5.0) return "text-amber-500 bg-amber-500/10 border-amber-500/30";
  return "text-rose-500 bg-rose-500/10 border-rose-500/30";
}

/**
 * The four criteria `speaking_grade.v1` asks the model for, plus the older
 * spellings that rows already in the table were written with.
 *
 * The names have to match the prompt exactly or the label falls through to the
 * title-cased raw name — which is English, whatever the learner's language.
 * `grammatical_range` did precisely that, and `task_response` was not listed at
 * all, so half the criteria on this screen were untranslated.
 *
 * There is no `pronunciation` entry on purpose. The prompt tells the model "Do
 * not guess pronunciation acoustics", and this screen says a few blocks down
 * that pronunciation was not assessed. Offering a label for it invites a
 * contradiction on the same page; an unexpected criterion falls through to the
 * default and is shown under its own name instead.
 */
const CRITERION_KEYS: Record<string, string> = {
  task_response: "speaking.criterionTaskResponse",
  fluency_coherence: "speaking.criterionFluency",
  fluency: "speaking.criterionFluency",
  lexical_resource: "speaking.criterionVocabulary",
  vocabulary: "speaking.criterionVocabulary",
  grammatical_range: "speaking.criterionGrammar",
  grammatical_range_accuracy: "speaking.criterionGrammar",
  grammar: "speaking.criterionGrammar",
};

const CRITERION_FALLBACKS: Record<string, string> = {
  "speaking.criterionTaskResponse": "Task Response",
  "speaking.criterionFluency": "Fluency & Coherence",
  "speaking.criterionVocabulary": "Lexical Resource",
  "speaking.criterionGrammar": "Grammatical Range & Accuracy",
};

function getCriterionTitleKey(name: string): string {
  return CRITERION_KEYS[name] ?? name;
}

function getCriterionFallback(name: string): string {
  const key = CRITERION_KEYS[name];
  if (key !== undefined) return CRITERION_FALLBACKS[key] ?? name;
  return name.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

export const SpeakingFeedbackView: React.FC<SpeakingFeedbackViewProps> = ({
  feedback,
  taskType,
  referenceText,
  onContinue,
  className,
}) => {
  const { t, i18n } = useTranslation();
  const isVi = i18n.language.startsWith("vi");
  const [showCriteriaDetail, setShowCriteriaDetail] = useState(true);

  const isReadAloud =
    taskType === "read_aloud" ||
    (feedback.read_aloud_accuracy !== null &&
      feedback.read_aloud_accuracy !== undefined) ||
    Boolean(referenceText);

  const diffTokens: DiffToken[] = useMemo(() => {
    if (!isReadAloud || !referenceText) return [];
    return computeWordDiff(referenceText, feedback.transcript || "");
  }, [isReadAloud, referenceText, feedback.transcript]);

  const summary = isVi
    ? feedback.feedback_vi || feedback.feedback_en
    : feedback.feedback_en;

  const isPurged = Boolean(feedback.recording_deleted_at);

  return (
    <div className={cn("space-y-6 max-w-3xl mx-auto py-2", className)}>
      {/* Header card with overall bands or accuracy */}
      <Card className="border-border bg-surface-card shadow-sm overflow-hidden">
        <CardHeader className="p-5 sm:p-6 bg-surface-muted/30 border-b border-border">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="space-y-1">
              <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
                <Sparkles className="h-4 w-4" aria-hidden="true" />
                <span>
                  {isReadAloud
                    ? t("speaking.readAloudAssessment", "Read Aloud Evaluation")
                    : t("speaking.assessmentTitle", "Speaking Evaluation")}
                </span>
              </div>
              <CardTitle className="text-xl sm:text-2xl font-bold text-text">
                {isReadAloud
                  ? t("speaking.readAloudTitle", "Read Aloud")
                  : t("speaking.respondTitle", "Spoken Response")}
              </CardTitle>
              <CardDescription className="text-xs sm:text-sm text-text-muted">
                {t(
                  "speaking.evaluationSubtitle",
                  "Automated speech recognition and language analysis",
                )}
              </CardDescription>
            </div>

            <div className="flex items-center gap-3">
              {feedback.read_aloud_accuracy !== null &&
                feedback.read_aloud_accuracy !== undefined && (
                  <div className="text-center px-3.5 py-2 rounded-xl bg-primary/10 border border-primary/20">
                    <div className="text-xs text-text-muted font-medium">
                      {t("speaking.accuracy", "Accuracy")}
                    </div>
                    <div className="text-xl sm:text-2xl font-mono font-extrabold text-primary">
                      {Math.round(feedback.read_aloud_accuracy)}%
                    </div>
                  </div>
                )}
            </div>
          </div>
        </CardHeader>

        <CardContent className="p-5 sm:p-6 space-y-6">
          {/* Summary feedback message */}
          {summary && (
            <div className="p-4 rounded-xl border border-primary/20 bg-primary/5 space-y-1.5">
              <div className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-primary">
                <Mic className="h-3.5 w-3.5" aria-hidden="true" />
                <span>{t("speaking.overallFeedback", "Overall Feedback")}</span>
              </div>
              <p className="text-sm text-text leading-relaxed whitespace-pre-line">
                {summary}
              </p>
            </div>
          )}

          {/* Audio Player Section */}
          <div className="p-4 rounded-xl border border-border bg-surface-muted/30 space-y-3">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
                <Volume2 className="h-4 w-4 text-primary" aria-hidden="true" />
                <span>{t("speaking.yourRecording", "Your Recording")}</span>
              </div>
              {isPurged && (
                <Badge variant="outline" className="text-xs text-text-muted">
                  {t("speaking.audioPurgedBadge", "Audio purged (90d)")}
                </Badge>
              )}
            </div>

            {isPurged ? (
              <p className="text-xs text-text-muted leading-relaxed italic">
                {t(
                  "speaking.audioPurgedNotice",
                  "Audio recording was removed per 90-day privacy retention policy. Your evaluation metrics and transcript are permanent.",
                )}
              </p>
            ) : feedback.audio_url ? (
              <div className="w-full">
                <audio
                  controls
                  className="w-full h-10 rounded-lg"
                  src={feedback.audio_url}
                  preload="metadata"
                >
                  <track kind="captions" />
                  {t(
                    "speaking.audioUnsupported",
                    "Your browser does not support the audio element.",
                  )}
                </audio>
              </div>
            ) : (
              <p className="text-xs text-text-muted leading-relaxed">
                {t(
                  "speaking.audioPreserved",
                  "Audio playback available for 90 days after submission.",
                )}
              </p>
            )}
          </div>

          {/* Block 1: Transcript & Alignment Diff */}
          <div className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm font-semibold uppercase tracking-wider text-text-muted">
                {isReadAloud && referenceText
                  ? t("speaking.transcriptDiffTitle", "Transcript & Word Alignment")
                  : t("speaking.transcriptTitle", "Speech Transcript")}
              </h3>
              {isReadAloud && referenceText && (
                <div className="flex items-center gap-2 text-xs text-text-muted">
                  <span className="inline-flex items-center gap-1">
                    <span className="h-2 w-2 rounded-full bg-emerald-500" />
                    <span>{t("speaking.legendMatched", "Matched")}</span>
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <span className="h-2 w-2 rounded-full bg-rose-500" />
                    <span>{t("speaking.legendMissed", "Missed")}</span>
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <span className="h-2 w-2 rounded-full bg-amber-500" />
                    <span>{t("speaking.legendExtra", "Extra")}</span>
                  </span>
                </div>
              )}
            </div>

            <div className="p-4 rounded-xl border border-border bg-surface-muted/40 text-sm leading-loose">
              {isReadAloud && diffTokens.length > 0 ? (
                <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1">
                  {diffTokens.map((token, idx) => {
                    if (token.type === "match") {
                      return (
                        <span key={idx} className="text-text font-normal">
                          {token.text}
                        </span>
                      );
                    }
                    if (token.type === "omission") {
                      return (
                        <span
                          key={idx}
                          title={t("speaking.missedWordTooltip", "Missed word")}
                          className="line-through text-rose-500 bg-rose-500/10 px-1 py-0.5 rounded font-medium"
                        >
                          {token.text}
                        </span>
                      );
                    }
                    if (token.type === "addition") {
                      return (
                        <span
                          key={idx}
                          title={t("speaking.addedWordTooltip", "Extra word")}
                          className="text-amber-500 bg-amber-500/10 px-1 py-0.5 rounded font-medium underline decoration-amber-500/50"
                        >
                          {token.text}
                        </span>
                      );
                    }
                    if (token.type === "substitution") {
                      return (
                        <span
                          key={idx}
                          title={t(
                            "speaking.substitutionTooltip",
                            "Expected '{{expected}}', heard '{{received}}'",
                            {
                              expected: token.expected,
                              received: token.received,
                            },
                          )}
                          className="inline-flex items-center gap-1 bg-primary/10 border border-primary/20 px-1.5 py-0.5 rounded text-xs"
                        >
                          <span className="line-through text-text-muted">
                            {token.expected}
                          </span>
                          <span className="text-primary font-semibold">
                            {token.received}
                          </span>
                        </span>
                      );
                    }
                    return null;
                  })}
                </div>
              ) : (
                <p className="text-text whitespace-pre-line leading-relaxed">
                  {feedback.transcript || (
                    <span className="text-text-muted italic">
                      {t("speaking.noTranscript", "No transcript recorded.")}
                    </span>
                  )}
                </p>
              )}
            </div>
          </div>

          {/* Block 3: The Numbers (Metrics) */}
          <div className="space-y-3">
            <h3 className="text-sm font-semibold uppercase tracking-wider text-text-muted">
              {t("speaking.metricsTitle", "Pacing & Delivery Measurements")}
            </h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {/* Words Per Minute */}
              <div className="p-4 rounded-xl border border-border bg-surface-card space-y-1.5">
                <div className="flex items-center justify-between text-xs text-text-muted">
                  <span className="flex items-center gap-1 font-medium">
                    <Clock className="h-3.5 w-3.5 text-primary" />
                    {t("speaking.speakingRate", "Speaking Rate")}
                  </span>
                  <Badge variant="secondary" className="font-mono text-xs">
                    {feedback.words_per_minute ?? 0}{" "}
                    {t("speaking.wpmUnit", "WPM")}
                  </Badge>
                </div>
                <p className="text-xs text-text-muted leading-relaxed">
                  {t(
                    "speaking.wpmGuidance",
                    "A comfortable natural English speaking pace is typically 110–150 words per minute.",
                  )}
                </p>
              </div>

              {/* Read Aloud Accuracy */}
              {feedback.read_aloud_accuracy !== null &&
                feedback.read_aloud_accuracy !== undefined && (
                  <div className="p-4 rounded-xl border border-border bg-surface-card space-y-1.5">
                    <div className="flex items-center justify-between text-xs text-text-muted">
                      <span className="flex items-center gap-1 font-medium">
                        <Activity className="h-3.5 w-3.5 text-emerald-500" />
                        {t("speaking.accuracyMetric", "Text Fidelity")}
                      </span>
                      <Badge
                        variant="outline"
                        className="font-mono text-xs text-emerald-500 border-emerald-500/30"
                      >
                        {Math.round(feedback.read_aloud_accuracy)}%
                      </Badge>
                    </div>
                    <p className="text-xs text-text-muted leading-relaxed">
                      {t(
                        "speaking.accuracyGuidance",
                        "Percentage of reference words recognized by automatic transcription.",
                      )}
                    </p>
                  </div>
                )}
            </div>
          </div>

          {/* Block 2: Criteria bands */}
          {feedback.criteria && feedback.criteria.length > 0 && (
            <div className="space-y-3">
              <button
                type="button"
                onClick={() => setShowCriteriaDetail((prev) => !prev)}
                className="flex items-center justify-between w-full text-left text-sm font-semibold uppercase tracking-wider text-text-muted hover:text-text transition-colors"
              >
                <div className="flex items-center gap-1.5">
                  <Award className="h-4 w-4 text-primary" />
                  <span>
                    {t("speaking.criteriaBandsTitle", "Criteria Assessment")}
                  </span>
                </div>
                {showCriteriaDetail ? (
                  <ChevronUp className="h-4 w-4" />
                ) : (
                  <ChevronDown className="h-4 w-4" />
                )}
              </button>

              {showCriteriaDetail && (
                <div className="grid grid-cols-1 gap-3">
                  {feedback.criteria.map((crit) => {
                    const comment = isVi
                      ? crit.comment_vi || crit.comment_en
                      : crit.comment_en;
                    return (
                      <div
                        key={crit.name}
                        className="p-4 rounded-xl border border-border bg-surface-card space-y-2"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-sm font-semibold text-text">
                            {t(
                              getCriterionTitleKey(crit.name),
                              getCriterionFallback(crit.name),
                            )}
                          </span>
                          <span
                            className={cn(
                              "font-mono text-xs font-bold px-2.5 py-0.5 rounded-full border",
                              getBandColor(crit.band),
                            )}
                          >
                            {t("speaking.bandLabel", "Band {{band}}", {
                              band: crit.band.toFixed(1),
                            })}
                          </span>
                        </div>
                        {comment && (
                          <p className="text-xs sm:text-sm text-text-muted leading-relaxed">
                            {comment}
                          </p>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}

          {/* Block 4: Plain statement that pronunciation was not assessed (BR-SPEAKING-09) */}
          <div className="p-4 rounded-xl border border-border/80 bg-surface-muted/60 space-y-1.5 flex items-start gap-3">
            <Info className="h-4 w-4 text-text-muted shrink-0 mt-0.5" aria-hidden="true" />
            <div className="space-y-1 text-xs text-text-muted leading-relaxed">
              <p className="font-semibold text-text">
                {t(
                  "speaking.pronunciationDisclaimerTitle",
                  "Note on Pronunciation Assessment",
                )}
              </p>
              <p>
                {t(
                  "speaking.pronunciationDisclaimerDesc",
                  "Pronunciation, intonation, and vowel clarity are not directly measured. Scores reflect automated speech recognition transcript alignment and language model evaluation.",
                )}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Action button if continuation handler is provided */}
      {onContinue && (
        <div className="flex justify-end pt-2">
          <Button onClick={onContinue} className="min-w-[120px]">
            {t("common.continue", "Continue")}
          </Button>
        </div>
      )}
    </div>
  );
};

export default SpeakingFeedbackView;
