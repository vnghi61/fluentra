import React, { useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  Check,
  Clock,
  FileUp,
  PlayCircle,
  Upload,
  X,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Textarea } from "@/components/ui/textarea";

import {
  useSubmitUpload,
  useUploads,
  type VocabUpload,
} from "../api/uploadApi";
import type { WordSummary } from "../api/searchApi";
import {
  countWords,
  importWordFile,
  MAX_IMPORT_BYTES,
  MAX_UPLOAD_WORDS,
  splitAtLimit,
} from "../model/wordList";
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

export const UploadForm: React.FC<UploadFormProps> = ({ onSubmitted }) => {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const submit = useSubmitUpload();

  const wordCount = countWords(text);
  const canSubmit = wordCount > 0 && !submit.isPending;
  const overLimit = wordCount > MAX_UPLOAD_WORDS;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    // One upload takes MAX_UPLOAD_WORDS words and the server drops the rest
    // without saying so; the rest stays in the box for the next one.
    const { head, rest } = splitAtLimit(text);
    submit.mutate(head, {
      onSuccess: () => {
        setText(rest);
        setNotice(
          rest
            ? t("uploads.import.restLeft", {
                count: countWords(rest),
                defaultValue:
                  "{{count}} more words are still in the box. Add them when you are ready.",
              })
            : null,
        );
        onSubmitted?.();
      },
    });
  };

  /**
   * Reads a CSV, TSV or Anki export into the box, below what is there. It
   * does not submit: the learner sees the list and the count first.
   */
  const handleFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (file.size > MAX_IMPORT_BYTES) {
      setNotice(
        t(
          "uploads.import.tooLarge",
          "That file is over 1 MB. Export fewer cards, or split it.",
        ),
      );
      return;
    }
    const imported = importWordFile(await file.text());
    if (imported.rows === 0) {
      setNotice(
        t(
          "uploads.import.empty",
          "No words found in that file. Each row should start with a word.",
        ),
      );
      return;
    }
    setText((current) =>
      current.trim() === ""
        ? imported.text
        : `${current.replace(/\n*$/, "")}${NEWLINE}${imported.text}`,
    );
    setNotice(
      t("uploads.import.done", {
        count: imported.rows,
        name: file.name,
        defaultValue:
          "Read {{count}} rows from {{name}}. Check the list, then add them.",
      }),
    );
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
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <input
            ref={fileInput}
            id="vocab-import"
            type="file"
            accept=".csv,.tsv,.txt,text/csv,text/tab-separated-values,text/plain"
            className="sr-only"
            tabIndex={-1}
            onChange={(event) => void handleFile(event)}
          />
          <Button
            type="button"
            variant="outline"
            className="min-h-11 gap-2"
            onClick={() => fileInput.current?.click()}
          >
            <FileUp className="h-4 w-4" aria-hidden="true" />
            {t("uploads.import.button", "Import a file")}
          </Button>
          <span className="text-sm text-text-muted">
            {t(
              "uploads.import.hint",
              "CSV, TSV, or an Anki export (Notes in Plain Text): the word first, its meaning second.",
            )}
          </span>
        </div>
        {notice && (
          <p role="status" className="text-sm text-text">
            {notice}
          </p>
        )}
      </div>

      {submit.data && (
        <SubmitResult
          submitted={submit.data}
          onDismiss={() => submit.reset()}
        />
      )}

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
          {overLimit &&
            ` · ${t("uploads.import.overLimit", {
              count: MAX_UPLOAD_WORDS,
              defaultValue: "the first {{count}} go now, the rest stay here",
            })}`}
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

/**
 * What became of the paste just sent, where the learner is still looking.
 *
 * Known words come back already added, so this can say so at once; new words
 * are checked within seconds, and the counts follow the upload list's polling
 * rather than a timer of their own. The full list is one link away.
 */
const SubmitResult: React.FC<{
  submitted: VocabUpload;
  onDismiss: () => void;
}> = ({ submitted, onDismiss }) => {
  const { t } = useTranslation();
  const uploads = useUploads(true);
  const live =
    uploads.data?.pages
      .flatMap((page) => page.items)
      .find((upload) => upload.id === submitted.id) ?? submitted;

  const added = live.verified_count + (live.queued_count ?? 0);
  const attention = live.rejected_count + (live.failed_count ?? 0);
  const pending = live.pending_count;
  const done = live.item_count - pending;

  return (
    <div
      className="space-y-3 rounded-lg border border-border bg-surface-muted p-4"
      aria-live="polite"
    >
      <div className="flex items-start justify-between gap-3">
        <p className="font-semibold text-text">
          {pending > 0
            ? t("uploads.result.title", { count: live.item_count })
            : t("uploads.result.done", { count: added })}
        </p>
        <button
          type="button"
          onClick={onDismiss}
          className="-m-3 flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-text-muted hover:text-text"
          aria-label={t("uploads.result.dismiss")}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>

      <ul className="space-y-1.5 text-sm">
        {pending > 0 && added > 0 && (
          <li className="flex items-center gap-2 text-success-accent">
            <Check className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("uploads.result.addedNow", { count: added })}
          </li>
        )}
        {pending > 0 && (
          <li className="flex items-center gap-2 text-text-muted">
            <Clock className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("uploads.result.checking", { count: pending })}
          </li>
        )}
        {attention > 0 && (
          <li className="flex items-center gap-2 text-warning-accent">
            <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("uploads.result.notAdded", { count: attention })}
          </li>
        )}
      </ul>

      {pending > 0 && (
        <Progress
          value={done}
          max={live.item_count}
          aria-label={t("uploads.progress", { done, total: live.item_count })}
          className="h-1.5"
        />
      )}

      <div className="flex flex-col gap-2 sm:flex-row">
        {added > 0 && (
          <Button asChild size="sm">
            <Link to="/practice/review">
              <PlayCircle className="h-4 w-4" aria-hidden="true" />
              {t("uploads.result.review")}
            </Link>
          </Button>
        )}
        <Button asChild size="sm" variant="outline">
          <a href="#my-words-history">{t("uploads.result.viewList")}</a>
        </Button>
      </div>
    </div>
  );
};
