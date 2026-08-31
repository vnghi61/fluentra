import React from "react";
import { useTranslation } from "react-i18next";

import { ExampleSentences } from "@/components/ui/example-sentences";
import type { ExampleSentence } from "@/lib/examples";
import { PronounceButton } from "@/components/ui/pronounce-button";

export interface FlashcardBackProps {
  word: string;
  ipa?: string;
  definition: string;
  definitionVi?: string;
  exampleSentences?: ExampleSentence[];
  audioUrl?: string | null | undefined;
  partOfSpeech?: string;
}

export const FlashcardBack: React.FC<FlashcardBackProps> = ({
  word,
  ipa,
  definition,
  definitionVi,
  exampleSentences = [],
  audioUrl,
  partOfSpeech,
}) => {
  const { i18n } = useTranslation();

  // The learner's own language leads when they have chosen it, and the English
  // definition is never dropped — it is the thing being learned. `startsWith`
  // because a stored preference can be "vi-VN" as easily as "vi".
  const prefersVietnamese = i18n.language.toLowerCase().startsWith("vi");
  const gloss = definitionVi?.trim() ? definitionVi : undefined;
  const lead = prefersVietnamese && gloss ? gloss : definition;
  const second = prefersVietnamese && gloss ? definition : gloss;

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

        <div className="flex items-center justify-center gap-2">
          {ipa && (
            <p className="font-mono text-sm text-primary-accent">{ipa}</p>
          )}
          <PronounceButton text={word} audioUrl={audioUrl} />
        </div>

        {/* The definition in the language the learner is reading in. */}
        <p className="text-lg md:text-xl font-medium text-text leading-relaxed pt-1">
          {lead}
        </p>

        {second && (
          <p className="text-base text-primary-accent font-semibold">
            {second}
          </p>
        )}

        {/* Every authored example, each one audible on its own. */}
        <ExampleSentences sentences={exampleSentences} highlight={word} />
      </div>
    </div>
  );
};
