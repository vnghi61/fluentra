import React from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { MaterialUploader } from "./MaterialUploader";
import {
  blankQuestion,
  type ActivityFields,
  type ActivityKind,
} from "../model/activityKinds";

const labelClass = "mb-1.5 block text-sm font-semibold text-text";
const hintClass = "text-sm text-text-muted";

type TextKey = {
  [K in keyof ActivityFields]: ActivityFields[K] extends string ? K : never;
}[keyof ActivityFields];

interface Props {
  kind: ActivityKind;
  fields: ActivityFields;
  onChange: (patch: Partial<ActivityFields>) => void;
  /** Makes every input id unique on a page holding many activities. */
  idPrefix: string;
}

/** The inputs for one activity, which depend entirely on its kind. */
export function ActivityFieldsEditor({
  kind,
  fields: f,
  onChange,
  idPrefix,
}: Props): React.JSX.Element {
  const { t } = useTranslation();
  const k = (key: string) => t(`studio.activity.field.${key}`);

  const text = (
    key: TextKey,
    label: string,
    opts: { multiline?: boolean; rows?: number; placeholder?: string } = {},
  ) => {
    const id = `${idPrefix}-${key}`;
    const set = (value: string) => onChange({ [key]: value });
    return (
      <div>
        <label htmlFor={id} className={labelClass}>
          {label}
        </label>
        {opts.multiline ? (
          <Textarea
            id={id}
            rows={opts.rows ?? 3}
            value={f[key]}
            placeholder={opts.placeholder}
            onChange={(e) => set(e.target.value)}
          />
        ) : (
          <Input
            id={id}
            value={f[key]}
            placeholder={opts.placeholder}
            onChange={(e) => set(e.target.value)}
          />
        )}
      </div>
    );
  };

  const choices = (
    options: readonly string[],
    correctIndex: number,
    set: (
      options: [string, string, string, string],
      correctIndex: number,
    ) => void,
    name: string,
  ) => (
    <fieldset className="space-y-1.5">
      <legend className={labelClass}>{k("options")}</legend>
      {options.map((option, i) => (
        <div key={i} className="flex items-center gap-2">
          <label className="flex h-11 w-11 shrink-0 cursor-pointer items-center justify-center">
            <input
              type="radio"
              name={name}
              checked={correctIndex === i}
              onChange={() =>
                set([...options] as [string, string, string, string], i)
              }
              aria-label={t("studio.activity.field.markCorrect", { n: i + 1 })}
              className="accent-primary"
            />
          </label>
          <Input
            value={option}
            placeholder={t("studio.activity.field.optionN", { n: i + 1 })}
            onChange={(e) => {
              const next = [...options] as [string, string, string, string];
              next[i] = e.target.value;
              set(next, correctIndex);
            }}
            className={cn("min-w-0", correctIndex === i && "border-success")}
          />
        </div>
      ))}
      <p className={hintClass}>{k("optionsHint")}</p>
    </fieldset>
  );

  const explanationVi = text("explanationVi", k("explanationVi"), {
    multiline: true,
    rows: 2,
  });

  switch (kind) {
    case "vocab_multiple_choice":
    case "grammar_tense_choice":
      return (
        <div className="space-y-3">
          {text("prompt", k("question"))}
          {choices(
            f.options,
            f.correctIndex,
            (options, correctIndex) => onChange({ options, correctIndex }),
            `${idPrefix}-correct`,
          )}
          {explanationVi}
        </div>
      );

    case "vocab_context_choice":
      return (
        <div className="space-y-3">
          {text("sentence", k("contextSentence"))}
          {text("prompt", k("question"))}
          {choices(
            f.options,
            f.correctIndex,
            (options, correctIndex) => onChange({ options, correctIndex }),
            `${idPrefix}-correct`,
          )}
          {explanationVi}
        </div>
      );

    case "vocab_gap_fill":
      return (
        <div className="space-y-3">
          {text("prompt", k("instruction"))}
          <div className="grid gap-2 sm:grid-cols-3">
            {text("sentenceBefore", k("sentenceBefore"))}
            {text("answer", k("blankAnswer"))}
            {text("sentenceAfter", k("sentenceAfter"))}
          </div>
          {explanationVi}
        </div>
      );

    case "vocab_flashcard":
      return (
        <div className="space-y-3">
          <div className="grid gap-2 sm:grid-cols-2">
            {text("answer", k("word"))}
            {text("ipa", k("ipa"), { placeholder: "/ˈwɜːd/" })}
          </div>
          {text("definition", k("definition"))}
          {text("definitionVi", k("definitionVi"))}
          {text("example", k("example"))}
        </div>
      );

    case "vocab_listen_type":
      return (
        <div className="space-y-3">
          {text("prompt", k("instruction"))}
          {text("answer", k("spokenText"))}
        </div>
      );

    case "vocab_reorder":
      return (
        <div className="space-y-3">
          {text("prompt", k("instruction"))}
          {text("answer", k("reorderSentence"))}
          <p className={hintClass}>{k("reorderHint")}</p>
        </div>
      );

    case "vocab_match":
      return (
        <div className="space-y-3">
          {text("prompt", k("instruction"))}
          <div className="space-y-1.5">
            <span className={labelClass}>{k("pairs")}</span>
            {f.pairs.map((pair, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  className="min-w-0"
                  value={pair.word}
                  placeholder={k("pairWord")}
                  onChange={(e) =>
                    onChange({
                      pairs: f.pairs.map((p, j) =>
                        j === i ? { ...p, word: e.target.value } : p,
                      ),
                    })
                  }
                />
                <Input
                  className="min-w-0"
                  value={pair.meaning}
                  placeholder={k("pairMeaning")}
                  onChange={(e) =>
                    onChange({
                      pairs: f.pairs.map((p, j) =>
                        j === i ? { ...p, meaning: e.target.value } : p,
                      ),
                    })
                  }
                />
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() =>
                    onChange({ pairs: f.pairs.filter((_, j) => j !== i) })
                  }
                  disabled={f.pairs.length <= 2}
                  aria-label={k("removePair")}
                  className="shrink-0 text-text-muted hover:text-danger"
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </Button>
              </div>
            ))}
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                onChange({ pairs: [...f.pairs, { word: "", meaning: "" }] })
              }
              disabled={f.pairs.length >= 8}
              className="-ml-3"
            >
              <Plus className="h-4 w-4" aria-hidden="true" /> {k("addPair")}
            </Button>
          </div>
        </div>
      );

    case "grammar_sentence_transform":
      return (
        <div className="space-y-3">
          {text("prompt", k("transformPrompt"), { multiline: true, rows: 2 })}
          {text("answer", k("correctAnswer"))}
          {text("acceptable", k("acceptable"), { multiline: true, rows: 2 })}
          {explanationVi}
        </div>
      );

    case "reading_comprehension":
      return (
        <div className="space-y-3">
          {text("passageTitle", k("passageTitle"))}
          {text("passage", k("passage"), { multiline: true, rows: 6 })}
          {f.questions.map((q, qi) => {
            const setQ = (patch: Partial<typeof q>) =>
              onChange({
                questions: f.questions.map((x, j) =>
                  j === qi ? { ...x, ...patch } : x,
                ),
              });
            return (
              <div
                key={qi}
                className="space-y-2 rounded-xl border border-border-subtle p-3"
              >
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold text-text">
                    {t("studio.activity.field.questionN", { n: qi + 1 })}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() =>
                      onChange({
                        questions: f.questions.filter((_, j) => j !== qi),
                      })
                    }
                    disabled={f.questions.length <= 4}
                    aria-label={k("removeQuestion")}
                    className="text-text-muted hover:text-danger"
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </div>
                <Input
                  value={q.prompt}
                  placeholder={k("question")}
                  onChange={(e) => setQ({ prompt: e.target.value })}
                />
                {choices(
                  q.options,
                  q.correctIndex,
                  (options, correctIndex) => setQ({ options, correctIndex }),
                  `${idPrefix}-q${qi}`,
                )}
                <Textarea
                  rows={2}
                  value={q.explanationVi}
                  placeholder={k("explanationVi")}
                  aria-label={k("explanationVi")}
                  onChange={(e) => setQ({ explanationVi: e.target.value })}
                />
              </div>
            );
          })}
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onChange({ questions: [...f.questions, blankQuestion()] })
            }
            disabled={f.questions.length >= 6}
            className="-ml-3"
          >
            <Plus className="h-4 w-4" aria-hidden="true" /> {k("addQuestion")}
          </Button>
        </div>
      );

    case "writing_prompt":
      return (
        <div className="space-y-3">
          {text("prompt", k("writingPrompt"), { multiline: true, rows: 3 })}
          {text("modelAnswer", k("modelAnswer"), { multiline: true, rows: 5 })}
          <div className="w-36">
            <label htmlFor={`${idPrefix}-minWords`} className={labelClass}>
              {k("minWords")}
            </label>
            <Input
              id={`${idPrefix}-minWords`}
              type="number"
              min={20}
              max={400}
              value={f.minWords}
              onChange={(e) => onChange({ minWords: Number(e.target.value) })}
            />
          </div>
        </div>
      );

    case "speaking_task":
      return (
        <div className="space-y-3">
          <div className="flex flex-wrap gap-x-5 text-sm text-text">
            {(["read_aloud", "respond"] as const).map((type) => (
              <label
                key={type}
                className="flex min-h-11 cursor-pointer items-center gap-2"
              >
                <input
                  type="radio"
                  name={`${idPrefix}-task`}
                  checked={f.taskType === type}
                  onChange={() => onChange({ taskType: type })}
                  className="accent-primary"
                />
                {k(type === "read_aloud" ? "readAloud" : "respond")}
              </label>
            ))}
          </div>
          {text(
            "prompt",
            k(f.taskType === "read_aloud" ? "instruction" : "respondPrompt"),
            {
              multiline: f.taskType === "respond",
              rows: 2,
            },
          )}
          {f.taskType === "read_aloud" &&
            text("referenceText", k("referenceText"), {
              multiline: true,
              rows: 3,
            })}
        </div>
      );

    case "lesson_material":
      return (
        <MaterialUploader fields={f} onChange={onChange} idPrefix={idPrefix} />
      );
  }
}
