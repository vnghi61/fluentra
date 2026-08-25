import React, { useEffect, useState } from "react";
import { ArrowRight, RotateCw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ExerciseFlashcardProps {
  prompt: string;
  targetWord?: string;
  ipa?: string;
  definition?: string;
  exampleSentence?: string;
  isLoading?: boolean;
  onComplete: () => void;
}

export const ExerciseFlashcard: React.FC<ExerciseFlashcardProps> = ({
  prompt,
  targetWord,
  ipa,
  definition,
  exampleSentence,
  isLoading = false,
  onComplete,
}) => {
  const { t } = useTranslation();
  const [isFlipped, setIsFlipped] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === " " || e.key === "Enter") {
        e.preventDefault();
        if (!isFlipped) {
          setIsFlipped(true);
        } else {
          onComplete();
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isFlipped, onComplete]);

  return (
    <div className="space-y-8 max-w-xl mx-auto py-4">
      <div className="text-center space-y-1">
        <h2 className="text-xl md:text-2xl font-bold text-text">
          {prompt || t("runner.flipPrompt", "Study this key term")}
        </h2>
      </div>

      {/* Interactive Flashcard with Flip Animation */}
      <div
        role="button"
        tabIndex={0}
        aria-label="Flashcard"
        onClick={() => setIsFlipped((prev) => !prev)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setIsFlipped((prev) => !prev);
          }
        }}
        className={cn(
          "min-h-[280px] p-8 rounded-3xl border-2 cursor-pointer transition-all duration-300 flex flex-col items-center justify-center text-center shadow-lg select-none",
          isFlipped
            ? "border-primary/50 bg-gradient-to-br from-surface-card to-primary/10"
            : "border-border bg-surface-card hover:border-primary/40",
        )}
      >
        {!isFlipped ? (
          <div className="space-y-4 animate-in fade-in">
            <h3 className="text-3xl md:text-4xl font-extrabold text-text tracking-tight">
              {targetWord}
            </h3>
            {ipa && (
              <p className="font-mono text-sm text-primary-accent tracking-wide">
                {ipa}
              </p>
            )}
            <p className="text-xs text-text-muted mt-6 flex items-center justify-center gap-1.5 font-medium">
              <RotateCw className="h-3.5 w-3.5" aria-hidden="true" />
              {t("runner.flipPrompt", "Press Space or tap to flip card")}
            </p>
          </div>
        ) : (
          <div className="space-y-4 animate-in fade-in">
            <p className="text-xl md:text-2xl font-semibold text-text">
              {definition}
            </p>
            {exampleSentence && (
              <p className="text-sm text-text-muted italic border-t border-border-subtle pt-3 max-w-md">
                "{exampleSentence}"
              </p>
            )}
          </div>
        )}
      </div>

      {/* Action Bar */}
      <div className="pt-4 flex justify-end">
        <Button
          size="lg"
          disabled={isLoading}
          onClick={isFlipped ? onComplete : () => setIsFlipped(true)}
          className="w-full sm:w-auto min-w-[160px] font-bold gap-2"
        >
          {isFlipped ? t("runner.continueBtn", "Continue") : t("runner.flipCardBtn", "Flip Card")}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
};
