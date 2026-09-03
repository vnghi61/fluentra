import React, { useState } from "react";
import { Upload } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { useSubmitUpload } from "../api/uploadApi";

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
 */
export interface UploadFormProps {
  onSubmitted?: () => void;
}

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

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <label
          htmlFor="vocab-upload"
          className="block text-sm font-semibold text-text"
        >
          {t("uploads.label", "Paste your words")}
        </label>
        <textarea
          id="vocab-upload"
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={10}
          spellCheck={false}
          placeholder={t(
            "uploads.placeholder",
            "leisure - thời gian rảnh\nhabit: thói quen\njourney",
          )}
          className={cn(
            "w-full rounded-xl border-2 border-border bg-surface-card p-4",
            // 16px, not 14: iOS Safari zooms the viewport when a field below
            // that is focused, and a learner pasting a list on a phone would
            // land in a zoomed page they then have to pinch out of.
            "font-mono text-base leading-relaxed text-text",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
          )}
        />
        <p className="text-xs text-text-muted">
          {t(
            "uploads.hint",
            "One word per line. Add its meaning after a dash, colon or tab if you want to — a plain list of words is fine.",
          )}
        </p>
      </div>

      {submit.isError && (
        <div
          role="alert"
          className="rounded-xl border border-danger/30 bg-danger/10 p-3 text-sm text-danger-accent"
        >
          {submit.error instanceof Error
            ? submit.error.message
            : t("uploads.failed", "We could not save that. Please try again.")}
        </div>
      )}

      <div className="flex items-center justify-between gap-3">
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
          className="gap-2 font-bold min-h-[48px]"
        >
          <Upload className="h-4 w-4" aria-hidden="true" />
          {t("uploads.submit", "Add these words")}
        </Button>
      </div>
    </form>
  );
};
