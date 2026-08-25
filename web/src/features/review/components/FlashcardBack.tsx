import React from "react";

export interface FlashcardBackProps {
  word: string;
  ipa?: string;
  definition: string;
  definitionVi?: string;
  exampleSentence?: string;
  partOfSpeech?: string;
}

export const FlashcardBack: React.FC<FlashcardBackProps> = ({
  word,
  ipa,
  definition,
  definitionVi,
  exampleSentence,
  partOfSpeech,
}) => {
  return (
    <div className="w-full min-h-[300px] p-8 rounded-3xl border-2 border-primary/40 bg-gradient-to-br from-surface-card to-primary/5 transition-all flex flex-col items-center justify-center text-center shadow-lg">
      <div className="space-y-4 max-w-lg mx-auto w-full">
        {/* Header word + pos */}
        <div className="flex flex-wrap items-center justify-center gap-2">
          <h2 className="text-3xl font-extrabold text-text tracking-tight">
            {word}
          </h2>
          {partOfSpeech && (
            <span className="italic text-xs text-text-muted font-medium bg-surface-muted px-2 py-0.5 rounded-md">
              {partOfSpeech}
            </span>
          )}
        </div>

        {ipa && (
          <p className="font-mono text-sm text-primary-accent">{ipa}</p>
        )}

        {/* English Definition */}
        <p className="text-lg md:text-xl font-medium text-text leading-relaxed pt-1">
          {definition}
        </p>

        {/* Vietnamese Translation / Sense */}
        {definitionVi && (
          <p className="text-base text-primary-accent font-semibold">
            {definitionVi}
          </p>
        )}

        {/* Example Sentence */}
        {exampleSentence && (
          <div className="pt-3 border-t border-border-subtle">
            <p className="text-sm text-text-muted italic">
              "{exampleSentence}"
            </p>
          </div>
        )}
      </div>
    </div>
  );
};
