import React, { useState } from "react";
import {
  Award,
  BookOpen,
  ChevronDown,
  ChevronUp,
  HelpCircle,
  MessageSquare,
  Sparkles,
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
import type { WritingFeedback } from "../types";

export interface WritingFeedbackViewProps {
  feedback: WritingFeedback;
  essayText?: string | undefined;
  sampleAnswer?: string | undefined;
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

function getCriterionTitleKey(name: string): string {
  switch (name) {
    case "task_response":
      return "writing.taskResponse";
    case "coherence_cohesion":
      return "writing.coherenceCohesion";
    case "lexical_resource":
      return "writing.lexicalResource";
    case "grammatical_range_accuracy":
      return "writing.grammaticalRange";
    default:
      return name;
  }
}

function getCriterionFallback(name: string): string {
  switch (name) {
    case "task_response":
      return "Task Response";
    case "coherence_cohesion":
      return "Coherence & Cohesion";
    case "lexical_resource":
      return "Lexical Resource";
    case "grammatical_range_accuracy":
      return "Grammatical Range & Accuracy";
    default:
      return name.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  }
}

export const WritingFeedbackView: React.FC<WritingFeedbackViewProps> = ({
  feedback,
  essayText = "",
  sampleAnswer,
  onContinue,
  className,
}) => {
  const { t, i18n } = useTranslation();
  const isVi = i18n.language.startsWith("vi");
  const [selectedAnnotationIdx, setSelectedAnnotationIdx] = useState<
    number | null
  >(null);
  const [showModelAnswer, setShowModelAnswer] = useState(false);

  const summary = isVi
    ? feedback.feedback_vi || feedback.feedback_en
    : feedback.feedback_en;

  const annotations = feedback.annotations ?? [];
  const selectedAnnotation =
    selectedAnnotationIdx !== null ? annotations[selectedAnnotationIdx] : null;

  const runes = Array.from(essayText);

  // Render highlighted essay text using rune slices
  const renderAnnotatedEssay = () => {
    if (!essayText) {
      if (annotations.length === 0) {
        return (
          <p className="text-sm text-text-muted italic">
            {t("writing.noAnnotations", "No specific annotations.")}
          </p>
        );
      }
      return (
        <div className="space-y-2">
          {annotations.map((ann, idx) => (
            <div
              key={idx}
              className="p-3 rounded-lg border border-border bg-surface-muted/50 text-sm"
            >
              <div className="font-semibold text-text mb-1">
                &ldquo;{ann.quoted_text}&rdquo;
              </div>
              <p className="text-text-muted">
                {isVi ? ann.comment_vi || ann.comment_en : ann.comment_en}
              </p>
            </div>
          ))}
        </div>
      );
    }

    if (annotations.length === 0) {
      return (
        <p className="whitespace-pre-wrap leading-relaxed text-text text-base">
          {essayText}
        </p>
      );
    }

    const valid = annotations
      .map((a, idx) => ({ ...a, originalIdx: idx }))
      .filter(
        (a) =>
          a.start_offset >= 0 &&
          a.end_offset <= runes.length &&
          a.start_offset < a.end_offset,
      )
      .sort((a, b) => a.start_offset - b.start_offset);

    const nonOverlapping: typeof valid = [];
    let lastEnd = 0;
    for (const ann of valid) {
      if (ann.start_offset >= lastEnd) {
        nonOverlapping.push(ann);
        lastEnd = ann.end_offset;
      }
    }

    const parts: React.ReactNode[] = [];
    let cursor = 0;

    nonOverlapping.forEach((ann) => {
      if (ann.start_offset > cursor) {
        parts.push(
          <span key={`plain-${cursor}`}>
            {runes.slice(cursor, ann.start_offset).join("")}
          </span>,
        );
      }

      const isSelected = selectedAnnotationIdx === ann.originalIdx;
      const comment = isVi ? ann.comment_vi || ann.comment_en : ann.comment_en;

      parts.push(
        <button
          key={`ann-${ann.originalIdx}`}
          type="button"
          onClick={() =>
            setSelectedAnnotationIdx(isSelected ? null : ann.originalIdx)
          }
          className={cn(
            "rounded px-1.5 py-0.5 transition-all text-left inline cursor-pointer font-medium",
            isSelected
              ? "bg-primary text-primary-foreground shadow-sm ring-2 ring-primary/50"
              : "bg-amber-400/20 text-text border-b-2 border-amber-500 hover:bg-amber-400/35",
          )}
          title={comment}
        >
          {runes.slice(ann.start_offset, ann.end_offset).join("")}
        </button>,
      );

      cursor = ann.end_offset;
    });

    if (cursor < runes.length) {
      parts.push(
        <span key={`plain-${cursor}`}>{runes.slice(cursor).join("")}</span>,
      );
    }

    return (
      <div className="whitespace-pre-wrap leading-relaxed text-text text-base">
        {parts}
      </div>
    );
  };

  return (
    <div className={cn("space-y-6 max-w-3xl mx-auto py-2", className)}>
      {/* Header with Band and Score */}
      <div className="rounded-2xl border border-border bg-surface-card p-6 shadow-sm">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              <span>{t("writing.feedbackTitle", "Writing Assessment")}</span>
            </div>
            <h2 className="text-2xl font-bold text-text">
              {t("writing.overallBand", "Overall Band")}:{" "}
              <span
                className={cn(
                  "px-2.5 py-0.5 rounded-lg border font-mono text-2xl inline-block ml-1",
                  getBandColor(feedback.overall_band),
                )}
              >
                {feedback.overall_band.toFixed(1)}
              </span>
            </h2>
            <p className="text-sm text-text-muted">
              {t("writing.score", "Score")}: {feedback.score} / 100
            </p>
          </div>

          {feedback.overall_band >= 6.5 ? (
            <div className="flex items-center gap-2 rounded-xl bg-emerald-500/10 border border-emerald-500/30 px-3 py-2 text-emerald-500 text-sm font-medium">
              <Award className="h-5 w-5 shrink-0" />
              <span>Competent User</span>
            </div>
          ) : (
            <div className="flex items-center gap-2 rounded-xl bg-primary/10 border border-primary/30 px-3 py-2 text-primary-accent text-sm font-medium">
              <HelpCircle className="h-5 w-5 shrink-0" />
              <span>Developing Skills</span>
            </div>
          )}
        </div>

        {/* Examiner Bilingual Summary */}
        {summary && (
          <div className="mt-5 pt-5 border-t border-border space-y-2">
            <div className="text-xs font-semibold uppercase tracking-wider text-text-muted">
              {t("writing.summary", "Examiner Summary")}
            </div>
            <p className="text-sm text-text leading-relaxed whitespace-pre-line bg-surface-muted/60 rounded-xl p-4 border border-border/60">
              {summary}
            </p>
          </div>
        )}
      </div>

      {/* 4 Criteria Breakdown */}
      {feedback.criteria && feedback.criteria.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold uppercase tracking-wider text-text-muted px-1">
            {t("writing.criteriaBands", "Criteria Breakdown")}
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {feedback.criteria.map((c) => {
              const comment = isVi
                ? c.comment_vi || c.comment_en
                : c.comment_en;
              return (
                <Card
                  key={c.name}
                  className="bg-surface-card border-border shadow-none hover:border-border/80 transition-colors"
                >
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between gap-2">
                      <CardTitle className="text-sm font-semibold text-text">
                        {t(
                          getCriterionTitleKey(c.name),
                          getCriterionFallback(c.name),
                        )}
                      </CardTitle>
                      <Badge
                        variant="outline"
                        className={cn("font-mono", getBandColor(c.band))}
                      >
                        Band {c.band.toFixed(1)}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <p className="text-xs text-text-muted leading-relaxed">
                      {comment}
                    </p>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      {/* Annotated Essay */}
      <Card className="bg-surface-card border-border shadow-none">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-base font-semibold text-text flex items-center gap-2">
              <MessageSquare className="h-4 w-4 text-primary" />
              <span>
                {t("writing.annotations", "Essay Corrections & Notes")}
              </span>
            </CardTitle>
            {annotations.length > 0 && (
              <span className="text-xs text-text-muted">
                {t(
                  "writing.annotationClickHint",
                  "Click highlighted text for notes",
                )}
              </span>
            )}
          </div>
          <CardDescription>
            {annotations.length > 0
              ? `${annotations.length} highlighted observation${annotations.length > 1 ? "s" : ""}`
              : t("writing.noAnnotations", "No specific corrections")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="p-4 rounded-xl border border-border bg-surface-muted/30">
            {renderAnnotatedEssay()}
          </div>

          {/* Active Annotation Inspector Card */}
          {selectedAnnotation && (
            <div className="p-4 rounded-xl border border-primary/40 bg-primary/5 space-y-1 animate-in fade-in slide-in-from-top-1 duration-150">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase text-primary tracking-wider">
                  &ldquo;{selectedAnnotation.quoted_text}&rdquo;
                </span>
                <Button
                  variant="ghost"
                  className="h-11 w-11 min-h-[44px] min-w-[44px] p-0 text-xs text-text-muted hover:text-text"
                  onClick={() => setSelectedAnnotationIdx(null)}
                  aria-label="Close annotation details"
                >
                  ✕
                </Button>
              </div>
              <p className="text-sm text-text">
                {isVi
                  ? selectedAnnotation.comment_vi ||
                    selectedAnnotation.comment_en
                  : selectedAnnotation.comment_en}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Model Answer (Collapsible) */}
      {sampleAnswer && (
        <Card className="bg-surface-card border-border shadow-none">
          <CardHeader
            className="cursor-pointer select-none py-3"
            onClick={() => setShowModelAnswer(!showModelAnswer)}
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-sm font-semibold text-text">
                <BookOpen className="h-4 w-4 text-primary" />
                <span>{t("writing.modelAnswer", "Model Answer")}</span>
              </div>
              <Button size="sm" variant="ghost" className="h-8 w-8 p-0">
                {showModelAnswer ? (
                  <ChevronUp className="h-4 w-4" />
                ) : (
                  <ChevronDown className="h-4 w-4" />
                )}
              </Button>
            </div>
          </CardHeader>
          {showModelAnswer && (
            <CardContent className="pt-0 pb-4">
              <div className="p-4 rounded-xl border border-border/80 bg-surface-muted/40 text-sm text-text leading-relaxed whitespace-pre-line">
                {sampleAnswer}
              </div>
            </CardContent>
          )}
        </Card>
      )}

      {/* Continue Action */}
      {onContinue && (
        <div className="flex justify-end pt-2">
          <Button onClick={onContinue} size="lg" className="px-8">
            {t("runner.continueBtn", "Continue")}
          </Button>
        </div>
      )}
    </div>
  );
};
