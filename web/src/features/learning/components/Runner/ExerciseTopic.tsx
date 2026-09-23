import React from "react";
import { ArrowRight, BookOpen, CheckCircle2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  FoundationTopicBody,
  type FoundationTopicBodyShape,
} from "@/features/learning/components/Foundation/FoundationTopicBody";

export interface ExerciseTopicProps {
  title?: string | undefined;
  body: FoundationTopicBodyShape;
  isLoading?: boolean;
  isSubmitted: boolean;
  /** Marks the topic read; the runner submits `{"done": true}`. */
  onSubmit: () => void;
  onContinue: () => void;
}

/**
 * A foundation topic as a lesson step (WO 22 Stage F). It teaches and nothing
 * else — ungraded, weight 0 — so the whole task is reading it and moving on.
 * The explanation is rendered by the same component the topic page uses.
 */
export const ExerciseTopic: React.FC<ExerciseTopicProps> = ({
  title,
  body,
  isLoading = false,
  isSubmitted,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      <div className="space-y-1">
        <div className="flex items-center gap-2 text-primary-accent">
          <BookOpen className="h-4 w-4" aria-hidden="true" />
          <span className="text-[11px] font-bold uppercase tracking-wider">
            {t("runner.topic.label", "Foundation topic")}
          </span>
        </div>
        {title && (
          <h2 className="text-xl md:text-2xl font-bold text-text">{title}</h2>
        )}
      </div>

      <FoundationTopicBody body={body} />

      <div className="flex justify-end">
        {isSubmitted ? (
          <Button
            size="lg"
            onClick={onContinue}
            className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
          >
            {t("runner.continueBtn", "Tiếp tục")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        ) : (
          <Button
            size="lg"
            disabled={isLoading}
            isLoading={isLoading}
            onClick={onSubmit}
            className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
          >
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
            {t("runner.topic.continue", "Tiếp tục")}
          </Button>
        )}
      </div>
    </div>
  );
};
