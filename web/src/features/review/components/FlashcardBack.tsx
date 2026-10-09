import React from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { ExampleSentences } from "@/components/ui/example-sentences";
import { PronounceButton } from "@/components/ui/pronounce-button";
import { shuffleExamplesForReview, type ExampleSentence } from "@/lib/examples";

export interface FlashcardBackProps {
  word: string;
  ipa?: string;
  definition: string;
  definitionVi?: string;
  exampleSentences?: ExampleSentence[];
  audioUrl?: string | null | undefined;
  /** The page crediting the recording; without it the recording does not play. */
  audioAttribution?: string | null | undefined;
  audioLicence?: string | null | undefined;
  partOfSpeech?: string;
  onReportSentence?: ((sentenceText: string) => void) | undefined;
  /**
   * The answer's heading. The session moves focus here on reveal: the reveal
   * button has just turned away, and focus left on a hidden face reads nothing.
   */
  answerRef?: React.Ref<HTMLHeadingElement> | undefined;
}

export const FlashcardBack: React.FC<FlashcardBackProps> = ({
  word,
  ipa,
  definition,
  definitionVi,
  exampleSentences = [],
  audioUrl,
  audioAttribution,
  audioLicence,
  partOfSpeech,
  onReportSentence,
  answerRef,
}) => {
  const { t, i18n } = useTranslation();

  // The learner's own language leads when they have chosen it, and the English
  // definition is never dropped — it is the thing being learned. `startsWith`
  // because a stored preference can be "vi-VN" as easily as "vi".
  //
  // Each is labelled. Unlabelled, a one-word Vietnamese gloss read as a heading
  // and learners reported that the card had no Vietnamese meaning at all.
  const prefersVietnamese = i18n.language.toLowerCase().startsWith("vi");
  const gloss = definitionVi?.trim() ? definitionVi : undefined;

  const meaningBlock = gloss && (
    <div className="space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        {t("review.meaningVi")}
      </p>
      <p className="text-xl font-bold text-text md:text-2xl">{gloss}</p>
    </div>
  );
  const definitionBlock = (
    <div className="space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        {t("review.definitionEn")}
      </p>
      <p className="text-base leading-relaxed text-text md:text-lg">
        {definition}
      </p>
    </div>
  );

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
    <div className="flex h-full min-h-[300px] w-full flex-col gap-5 rounded-[var(--radius-card)] border border-primary/30 bg-surface-card p-5 shadow-[var(--shadow-card)] sm:min-h-[340px] sm:p-8">
      <div className="flex flex-col items-center gap-2 border-b border-border-subtle pb-4 text-center">
        <div className="flex max-w-full flex-wrap items-center justify-center gap-2">
          <h2
            ref={answerRef}
            tabIndex={-1}
            className="max-w-full break-words text-3xl font-bold tracking-tight text-text focus:outline-none"
          >
            {word}
          </h2>
          {partOfSpeech && (
            <Badge variant="secondary" className="italic">
              {partOfSpeech}
            </Badge>
          )}
        </div>

        <div className="flex flex-wrap items-center justify-center gap-2">
          {ipa && (
            <p className="font-mono text-sm text-primary-accent">{ipa}</p>
          )}
          <PronounceButton
            text={word}
            audioUrl={audioUrl}
            audioAttribution={audioAttribution}
            audioLicence={audioLicence}
          />
        </div>
      </div>

      {/* The meaning in the language the learner is reading in comes first. */}
      <div className="space-y-4">
        {prefersVietnamese ? (
          <>
            {meaningBlock}
            {definitionBlock}
          </>
        ) : (
          <>
            {definitionBlock}
            {meaningBlock}
          </>
        )}
      </div>

      {/* Three of however many the word has, shuffled per card. */}
      <ExampleSentences
        sentences={displayedExamples}
        highlight={word}
        onReportSentence={onReportSentence}
      />
    </div>
  );
};
