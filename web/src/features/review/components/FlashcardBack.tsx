import React from "react";
import { useTranslation } from "react-i18next";

import { ExampleSentences } from "@/components/ui/example-sentences";
import { shuffleExamplesForReview, type ExampleSentence } from "@/lib/examples";
import { PronounceButton } from "@/components/ui/pronounce-button";

export interface FlashcardBackProps {
  word: string;
  ipa?: string;
  definition: string;
  definitionVi?: string;
  exampleSentences?: ExampleSentence[];
  audioUrl?: string | null | undefined;
  partOfSpeech?: string;
  onReportSentence?: ((sentenceText: string) => void) | undefined;
}

export const FlashcardBack: React.FC<FlashcardBackProps> = ({
  word,
  ipa,
  definition,
  definitionVi,
  exampleSentences = [],
  audioUrl,
  partOfSpeech,
  onReportSentence,
}) => {
  const { i18n } = useTranslation();

  // The learner's own language leads when they have chosen it, and the English
  // definition is never dropped — it is the thing being learned. `startsWith`
  // because a stored preference can be "vi-VN" as easily as "vi".
  const prefersVietnamese = i18n.language.toLowerCase().startsWith("vi");
  const gloss = definitionVi?.trim() ? definitionVi : undefined;
  const lead = prefersVietnamese && gloss ? gloss : definition;
  const second = prefersVietnamese && gloss ? definition : gloss;

  // Shuffled once per card, not once per render: flipping the card back and
  // forth must not deal a new hand each time. Keyed on the word rather than on
  // the array, because the page above rebuilds `exampleSentences` on every
  // render and its identity would reshuffle mid-flip.
  const displayedExamples = React.useMemo(
    () => shuffleExamplesForReview(exampleSentences),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [word, exampleSentences.length],
  );

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

        {/* Three of however many the word has, shuffled per card. */}
        <ExampleSentences
          sentences={displayedExamples}
          highlight={word}
          onReportSentence={onReportSentence}
        />
      </div>
    </div>
  );
};
