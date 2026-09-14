import React, { useMemo } from "react";
import { useTranslation } from "react-i18next";

import { ListeningPlayer } from "@/features/exam/components/ListeningPlayer";
import { cn } from "@/lib/utils";
import type { PlacementItem } from "../../api/placement";
import {
  hasQuestions,
  parseItem,
  type ChoiceOption,
  type ItemAnswer,
} from "./placementItem";

export interface PlacementItemViewProps {
  item: PlacementItem;
  sessionId: string;
  answer: ItemAnswer;
  onChange: (answer: ItemAnswer) => void;
}

const OptionList: React.FC<{
  options: ChoiceOption[];
  selected: string | null;
  onSelect: (id: string) => void;
}> = ({ options, selected, onSelect }) => (
  <div className="grid gap-2">
    {options.map((option) => (
      <button
        key={option.id}
        type="button"
        aria-pressed={selected === option.id}
        onClick={() => onSelect(option.id)}
        className={cn(
          "flex min-h-[44px] w-full items-center gap-3 rounded-xl border p-3 text-left text-base text-text transition-colors",
          selected === option.id
            ? "border-primary bg-primary/10"
            : "border-border-subtle bg-card hover:border-border",
        )}
      >
        <span
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-full border text-sm font-bold",
            selected === option.id
              ? "border-primary bg-primary text-primary-fg"
              : "border-border text-text-muted",
          )}
          aria-hidden="true"
        >
          {option.id}
        </span>
        <span>{option.text}</span>
      </button>
    ))}
  </div>
);

/**
 * One placement item, rendered from its redacted body: a single choice, or a
 * passage or clip with its questions. The body carries no answer to leak.
 */
export const PlacementItemView: React.FC<PlacementItemViewProps> = ({
  item,
  sessionId,
  answer,
  onChange,
}) => {
  const { t } = useTranslation();
  const parsed = useMemo(() => parseItem(item.config), [item.config]);

  if (!hasQuestions(item)) {
    return (
      <fieldset className="space-y-4">
        <legend className="mb-4 text-lg font-semibold leading-snug text-text">
          {parsed.prompt}
        </legend>
        <OptionList
          options={parsed.options}
          selected={answer.selected}
          onSelect={(id) => onChange({ ...answer, selected: id })}
        />
      </fieldset>
    );
  }

  return (
    <div className="space-y-6">
      {item.skill === "listening" ? (
        <ListeningPlayer
          versionId={item.content_version_id}
          sittingId={sessionId}
          contextType="placement"
          title={parsed.title || undefined}
        />
      ) : (
        <article className="max-h-80 overflow-y-auto rounded-xl border border-border-subtle bg-surface-muted/60 p-4 text-base leading-relaxed text-text">
          {parsed.title && (
            <h2 className="mb-2 text-base font-semibold">{parsed.title}</h2>
          )}
          <p className="whitespace-pre-line">{parsed.passage}</p>
        </article>
      )}
      {parsed.questions.map((question, index) => (
        <fieldset key={question.id} className="space-y-3">
          <legend className="mb-3 text-base font-semibold text-text">
            {t("placement.item.question", { number: index + 1 })}{" "}
            {question.prompt}
          </legend>
          <OptionList
            options={question.options}
            selected={answer.answers[question.id] ?? null}
            onSelect={(id) =>
              onChange({
                ...answer,
                answers: { ...answer.answers, [question.id]: id },
              })
            }
          />
        </fieldset>
      ))}
    </div>
  );
};
