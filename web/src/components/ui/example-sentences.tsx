import React, { useState } from "react";
import { ChevronDown, Languages } from "lucide-react";
import { useTranslation } from "react-i18next";

import { PronounceButton } from "@/components/ui/pronounce-button";
import type { ExampleSentence } from "@/lib/examples";
import { cn } from "@/lib/utils";

/**
 * The example sentences for a word, each one individually audible.
 *
 * A sense carries five or more sentences now, and five sentences shown at once
 * turn a flashcard into a wall of text on a phone — the card is 300px tall and
 * the recall verdict sits below it. So the first two lead and the rest collapse
 * behind a control that names how many there are. Nothing is hidden from a
 * learner who wants it; nothing is in the way of one who does not.
 *
 * Highlighting the target word matters more than it looks: the point of five
 * examples is seeing the same word behave differently, and that only reads if
 * the eye can find it in each line.
 *
 * # Why the translations start hidden
 *
 * Every sentence now carries its Vietnamese rendering, and showing both at once
 * would defeat the exercise: the eye goes to the line it can read, and the
 * English becomes decoration under it. One tap reveals every translation at
 * once, so a learner who is stuck is never more than a tap from the meaning —
 * and a learner who is not stuck never reads it by accident.
 */
export interface ExampleSentencesProps {
  sentences: ExampleSentence[];
  /** Bolded wherever it appears, so the word stands out across the examples. */
  highlight?: string | undefined;
  /** How many to show before collapsing the rest. */
  initialVisible?: number;
  className?: string;
}

/** Splits a sentence around each occurrence of the target word, case-insensitively. */
function highlightParts(sentence: string, word?: string): React.ReactNode {
  const target = word?.trim();
  if (!target) return sentence;

  // Escaped because a lemma is data: a word such as "C++" would otherwise be a
  // malformed pattern and throw while rendering the card.
  const escaped = target.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  // `\w*` on the tail so "achieve" also matches "achieved" and "achieves",
  // which is how the sentences actually use it.
  const pattern = new RegExp(`(${escaped}\\w*)`, "gi");

  // `split` with a capturing group puts the captured matches at the odd
  // indices, which is the whole test. Calling `pattern.test()` here instead
  // would be wrong: a `/g/` regex carries `lastIndex` between calls and
  // alternates its answer on identical input.
  return sentence.split(pattern).map((part, index) =>
    index % 2 === 1 ? (
      <strong key={index} className="font-bold text-text">
        {part}
      </strong>
    ) : (
      <React.Fragment key={index}>{part}</React.Fragment>
    ),
  );
}

export const ExampleSentences: React.FC<ExampleSentencesProps> = ({
  sentences,
  highlight,
  initialVisible = 2,
  className,
}) => {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const [translated, setTranslated] = useState(false);

  if (sentences.length === 0) return null;

  const visible = expanded ? sentences : sentences.slice(0, initialVisible);
  const hidden = sentences.length - visible.length;
  const hasTranslations = sentences.some((sentence) => sentence.translation);

  // The card behind this flips on click, so every control here stops the event.
  const contain = (run: () => void) => (event: React.MouseEvent) => {
    event.stopPropagation();
    run();
  };

  return (
    <div
      className={cn(
        "w-full space-y-2 border-t border-border-subtle pt-3 text-left",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-text-muted">
          {t("examples.heading", "Examples")}
        </p>
        {hasTranslations && (
          <button
            type="button"
            onClick={contain(() => setTranslated((current) => !current))}
            aria-pressed={translated}
            className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-semibold text-primary-accent hover:bg-surface-muted"
          >
            <Languages className="h-3.5 w-3.5" aria-hidden="true" />
            {translated
              ? t("examples.hideTranslation", "Hide meaning")
              : t("examples.showTranslation", "Show meaning")}
          </button>
        )}
      </div>

      <ul className="space-y-1.5">
        {visible.map((sentence) => (
          <li
            key={sentence.text}
            className="flex items-start justify-between gap-2 rounded-lg px-2 py-1 hover:bg-surface-muted/50"
          >
            <div className="space-y-0.5 pt-2.5">
              <p className="text-sm italic leading-relaxed text-text-muted">
                &ldquo;{highlightParts(sentence.text, highlight)}&rdquo;
              </p>
              {translated && sentence.translation && (
                <p className="text-sm leading-relaxed text-primary-accent">
                  {sentence.translation}
                </p>
              )}
            </div>
            <PronounceButton
              text={sentence.text}
              audioUrl={sentence.audioUrl}
              label={t("examples.listen", "Listen to this sentence")}
            />
          </li>
        ))}
      </ul>

      {hidden > 0 && (
        <button
          type="button"
          onClick={contain(() => setExpanded(true))}
          className="inline-flex items-center gap-1 px-2 text-xs font-semibold text-primary-accent hover:underline"
        >
          <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
          {/*
            The count is rendered beside the label rather than interpolated into
            it. `t(key, default)` returns the raw default when i18next has not
            been initialised — which is the case in unit tests — and a literal
            "{{count}}" then reaches the screen.
          */}
          {t("examples.showMore", "Show more")} ({hidden})
        </button>
      )}
    </div>
  );
};
