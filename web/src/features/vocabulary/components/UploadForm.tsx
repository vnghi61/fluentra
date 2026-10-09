import React, { useState } from "react";
import { Upload } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

import { useSubmitUpload } from "../api/uploadApi";
import type { WordSummary } from "../api/searchApi";
import { WordAutocomplete } from "./WordAutocomplete";

/**
 * Pasting your own vocabulary in.
 *
 * A textarea and nothing else. The parser takes tab, dash, colon, equals,
 * semicolon and pipe, strips bullets and numbering, and accepts a bare word
 * list — so the thing a learner already has in their clipboard works, and there
 * is no format to learn before using the feature.
 *
 * The word count updates as they type, because the number that matters is how
 * many words were found, not how many lines were pasted: duplicates and page
 * numbers are dropped, and seeing that happen live is less alarming than being
 * told afterwards that twelve of thirty lines were ignored.
 *
 * Above it, a single-word field that searches the dictionary as you type. It
 * writes into the same box rather than submitting on its own, so adding six
 * words one at a time is still one upload -- and the learner can see and edit
 * what they picked before it is sent.
 */
export interface UploadFormProps {
  onSubmitted?: () => void;
}

/** The line separator the box and the server's parser both work in. */
const NEWLINE = "\n";

/** Counts what the server's parser will find, using the same rules. */
function countWords(text: string): number {
  const seen = new Set<string>();
  for (const rawLine of text.split("\n")) {
    let line = rawLine.trim().replace(/^[-*•\t ]+/, "");
    line = line.replace(/^\d+[.):]\s*/, "");
    if (line === "") continue;

    let term = line;
    for (const separator of [
      "\t",
      " - ",
      " – ",
      " — ",
      " = ",
      ": ",
      " : ",
      "=",
      ";",
      "|",
    ]) {
      const index = line.indexOf(separator);
      if (index > 0) {
        term = line.slice(0, index);
        break;
      }
    }
    term = term.trim().replace(/[.,;:]+$/, "");
    if (term === "" || !/\p{L}/u.test(term)) continue;
    seen.add(term.toLowerCase());
  }
  return seen.size;
}

export const UploadForm: React.FC<UploadFormProps> = ({ onSubmitted }) => {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const submit = useSubmitUpload();

  const wordCount = countWords(text);
  const canSubmit = wordCount > 0 && !submit.isPending;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    submit.mutate(text, {
      onSuccess: () => {
        setText("");
        onSubmitted?.();
      },
    });
  };

  /**
   * Puts a chosen word into the box, on its own line and only once.
   *
   * The list is what gets submitted, so a word picked from the dropdown has to
   * end up there and nowhere else. Appending a duplicate would be dropped by
   * the parser anyway, but the count beside the button would jump and then not
   * change, which reads as a broken button.
   */
  const appendTerm = (term: string) => {
    const trimmed = term.trim();
    if (trimmed === "") return;

    const already = text
      .split(NEWLINE)
      .some((line) => line.trim().toLowerCase() === trimmed.toLowerCase());
    if (already) return;

    setText((current) =>
      current === "" || current.endsWith(NEWLINE)
        ? `${current}${trimmed}${NEWLINE}`
        : `${current}${NEWLINE}${trimmed}${NEWLINE}`,
    );
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <div className="space-y-2">
        <label
          htmlFor="vocab-single-add"
          className="block text-sm font-semibold text-text"
        >
          {t("uploads.searchLabel", "Look a word up")}
        </label>
        <WordAutocomplete
          id="vocab-single-add"
          onSelectWord={(word: WordSummary) => appendTerm(word.lemma)}
          onCustomSubmit={appendTerm}
          placeholder={t(
            "uploads.searchPlaceholder",
            "Type a word — we suggest from the dictionary",
          )}
        />
        <p className="text-sm text-text-muted">
          {t(
            "uploads.searchHint",
            "Pick a suggestion and it drops into the list below. A word we do not have yet can be added anyway.",
          )}
        </p>
      </div>

      <div className="space-y-2">
        <label
          htmlFor="vocab-upload"
          className="block text-sm font-semibold text-text"
        >
          {t("uploads.label", "Paste your words")}
        </label>
        <Textarea
          id="vocab-upload"
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={10}
          spellCheck={false}
          placeholder={t(
            "uploads.placeholder",
            "leisure - thời gian rảnh\nhabit: thói quen\njourney",
          )}
          // 16px at every width, not only on phones: a pasted list is read
          // line by line, and the monospace keeps the separators aligned.
          className="min-h-[200px] font-mono md:text-base"
        />
        <p className="text-sm text-text-muted">
          {t(
            "uploads.hint",
            "One word per line. Add its meaning after a dash, colon or tab if you want to — a plain list of words is fine.",
          )}
        </p>
      </div>

      {submit.isError && (
        <div
          role="alert"
          className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger-accent"
        >
          {submit.error instanceof Error
            ? submit.error.message
            : t("uploads.failed", "We could not save that. Please try again.")}
        </div>
      )}

      <div className="flex flex-col-reverse gap-3 border-t border-border-subtle pt-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-text-muted" aria-live="polite">
          {wordCount > 0
            ? `${wordCount} ${t("uploads.wordsFound", "words found")}`
            : t("uploads.nothingYet", "Nothing to add yet")}
        </p>
        <Button
          type="submit"
          size="lg"
          disabled={!canSubmit}
          isLoading={submit.isPending}
          className="w-full font-semibold sm:w-auto"
        >
          <Upload className="h-4 w-4" aria-hidden="true" />
          {t("uploads.submit", "Add these words")}
        </Button>
      </div>
    </form>
  );
};
