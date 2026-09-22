import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CheckCircle2,
  Loader2,
  Plus,
  Trash2,
  Upload,
} from "lucide-react";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  pollMaterial,
  resourceApi,
  type MaterialUploadState,
} from "@/features/resource/api/resourceApi";
import {
  blankQuestion,
  type ActivityFields,
  type ActivityKind,
} from "../model/activityKinds";

const textareaClass =
  "w-full rounded-lg border border-border-subtle bg-surface-base px-3 py-2 text-base sm:text-xs text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary";

const labelClass = "block text-[11px] font-semibold text-text-muted mb-1";

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
          <textarea
            id={id}
            rows={opts.rows ?? 3}
            value={f[key]}
            placeholder={opts.placeholder}
            onChange={(e) => set(e.target.value)}
            className={textareaClass}
          />
        ) : (
          <Input
            id={id}
            value={f[key]}
            placeholder={opts.placeholder}
            onChange={(e) => set(e.target.value)}
            className="text-xs"
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
        <div key={i} className="flex items-center gap-1">
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
            className={cn("text-xs", correctIndex === i && "border-success")}
          />
        </div>
      ))}
      <p className="text-[11px] text-text-muted">{k("optionsHint")}</p>
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
        <div className="space-y-2.5">
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
        <div className="space-y-2.5">
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
        <div className="space-y-2.5">
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
        <div className="space-y-2.5">
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
        <div className="space-y-2.5">
          {text("prompt", k("instruction"))}
          {text("answer", k("spokenText"))}
        </div>
      );

    case "vocab_reorder":
      return (
        <div className="space-y-2.5">
          {text("prompt", k("instruction"))}
          {text("answer", k("reorderSentence"))}
          <p className="text-[11px] text-text-muted">{k("reorderHint")}</p>
        </div>
      );

    case "vocab_match":
      return (
        <div className="space-y-2.5">
          {text("prompt", k("instruction"))}
          <div className="space-y-1.5">
            <span className={labelClass}>{k("pairs")}</span>
            {f.pairs.map((pair, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input
                  value={pair.word}
                  placeholder={k("pairWord")}
                  onChange={(e) =>
                    onChange({
                      pairs: f.pairs.map((p, j) =>
                        j === i ? { ...p, word: e.target.value } : p,
                      ),
                    })
                  }
                  className="text-xs"
                />
                <Input
                  value={pair.meaning}
                  placeholder={k("pairMeaning")}
                  onChange={(e) =>
                    onChange({
                      pairs: f.pairs.map((p, j) =>
                        j === i ? { ...p, meaning: e.target.value } : p,
                      ),
                    })
                  }
                  className="text-xs"
                />
                <button
                  type="button"
                  onClick={() =>
                    onChange({ pairs: f.pairs.filter((_, j) => j !== i) })
                  }
                  disabled={f.pairs.length <= 2}
                  aria-label={k("removePair")}
                  className="p-1 text-text-muted hover:text-danger disabled:opacity-30"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() =>
                onChange({ pairs: [...f.pairs, { word: "", meaning: "" }] })
              }
              disabled={f.pairs.length >= 8}
              className="flex items-center gap-1 text-[11px] font-semibold text-primary-accent disabled:opacity-40"
            >
              <Plus className="h-3 w-3" /> {k("addPair")}
            </button>
          </div>
        </div>
      );

    case "grammar_sentence_transform":
      return (
        <div className="space-y-2.5">
          {text("prompt", k("transformPrompt"), { multiline: true, rows: 2 })}
          {text("answer", k("correctAnswer"))}
          {text("acceptable", k("acceptable"), { multiline: true, rows: 2 })}
          {explanationVi}
        </div>
      );

    case "reading_comprehension":
      return (
        <div className="space-y-2.5">
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
                className="space-y-1.5 rounded-md border border-border-subtle p-2"
              >
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-bold text-text">
                    {t("studio.activity.field.questionN", { n: qi + 1 })}
                  </span>
                  <button
                    type="button"
                    onClick={() =>
                      onChange({
                        questions: f.questions.filter((_, j) => j !== qi),
                      })
                    }
                    disabled={f.questions.length <= 4}
                    aria-label={k("removeQuestion")}
                    className="p-1 text-text-muted hover:text-danger disabled:opacity-30"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
                <Input
                  value={q.prompt}
                  placeholder={k("question")}
                  onChange={(e) => setQ({ prompt: e.target.value })}
                  className="text-xs"
                />
                {choices(
                  q.options,
                  q.correctIndex,
                  (options, correctIndex) => setQ({ options, correctIndex }),
                  `${idPrefix}-q${qi}`,
                )}
                <textarea
                  rows={2}
                  value={q.explanationVi}
                  placeholder={k("explanationVi")}
                  aria-label={k("explanationVi")}
                  onChange={(e) => setQ({ explanationVi: e.target.value })}
                  className={textareaClass}
                />
              </div>
            );
          })}
          <button
            type="button"
            onClick={() =>
              onChange({ questions: [...f.questions, blankQuestion()] })
            }
            disabled={f.questions.length >= 6}
            className="flex items-center gap-1 text-[11px] font-semibold text-primary-accent disabled:opacity-40"
          >
            <Plus className="h-3 w-3" /> {k("addQuestion")}
          </button>
        </div>
      );

    case "writing_prompt":
      return (
        <div className="space-y-2.5">
          {text("prompt", k("writingPrompt"), { multiline: true, rows: 3 })}
          {text("modelAnswer", k("modelAnswer"), { multiline: true, rows: 5 })}
          <div className="w-32">
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
              className="text-xs"
            />
          </div>
        </div>
      );

    case "speaking_task":
      return (
        <div className="space-y-2.5">
          <div className="flex gap-4 text-xs text-text">
            {(["read_aloud", "respond"] as const).map((type) => (
              <label key={type} className="flex items-center gap-1.5">
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
        <MaterialFields fields={f} onChange={onChange} idPrefix={idPrefix} />
      );
  }
}

const MATERIAL_ACCEPT: Record<"document" | "video", string> = {
  document:
    ".pdf,.doc,.docx,.ppt,.pptx,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.ms-powerpoint,application/vnd.openxmlformats-officedocument.presentationml.presentation",
  video: "video/mp4,video/webm",
};

/** The file picker, rights declaration and processing status for a material. */
function MaterialFields({
  fields: f,
  onChange,
  idPrefix,
}: {
  fields: ActivityFields;
  onChange: (patch: Partial<ActivityFields>) => void;
  idPrefix: string;
}): React.JSX.Element {
  const { t } = useTranslation();
  const [error, setError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const stopPolling = useRef<(() => void) | null>(null);

  const pollingFor = useRef<string | null>(null);

  useEffect(() => () => stopPolling.current?.(), []);

  const watch = (resourceId: string, kind: "document" | "video") => {
    stopPolling.current?.();
    pollingFor.current = resourceId;
    stopPolling.current = pollMaterial(
      resourceId,
      kind,
      (state: MaterialUploadState) => onChange({ materialStatus: state }),
    );
  };

  // A reopened draft reads back as "processing" and nothing had asked the
  // server since, so the material stayed there and the draft could never be
  // submitted. Ask again whenever a file is present that nobody is watching.
  useEffect(() => {
    if (!f.resourceId || pollingFor.current === f.resourceId) return;
    if (f.materialStatus === "ready" || f.materialStatus === "uploading")
      return;
    watch(f.resourceId, f.materialKind);
  });

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    setError(null);
    setIsUploading(true);
    onChange({ materialStatus: "uploading" });
    try {
      const intent = await resourceApi.createUploadIntent(file.name, file.type);
      await resourceApi.uploadDirect(intent, file);
      await resourceApi.confirmUpload(intent.id);
      onChange({
        resourceId: intent.id,
        materialStatus: "processing",
        materialTitle:
          f.materialTitle.trim() || file.name.replace(/\.[^.]+$/, ""),
      });
      watch(intent.id, f.materialKind);
    } catch (err) {
      onChange({ materialStatus: "failed" });
      setError(
        err instanceof Error
          ? err.message
          : t("studio.activity.material.uploadFailed", "Upload failed"),
      );
    } finally {
      setIsUploading(false);
    }
  };

  const status = f.materialStatus;
  const statusText: Record<MaterialUploadState, string> = {
    idle: t("studio.activity.material.statusIdle", "No file chosen yet"),
    uploading: t("studio.activity.material.statusUploading", "Uploading…"),
    processing:
      f.materialKind === "video"
        ? t(
            "studio.activity.material.statusProcessing",
            "Processing video… renditions are being prepared",
          )
        : t(
            "studio.activity.material.statusProcessingDocument",
            "Processing document… preparing its preview",
          ),
    ready: t("studio.activity.material.statusReady", "Ready to publish"),
    failed: t("studio.activity.material.statusFailed", "Processing failed"),
  };

  return (
    <div className="space-y-2.5">
      <div className="flex gap-4 text-xs text-text">
        {(["document", "video"] as const).map((kind) => (
          <label key={kind} className="flex items-center gap-1.5">
            <input
              type="radio"
              name={`${idPrefix}-material-kind`}
              checked={f.materialKind === kind}
              onChange={() => {
                onChange({ materialKind: kind });
                if (f.resourceId) watch(f.resourceId, kind);
              }}
              className="accent-primary"
            />
            {t(
              kind === "document"
                ? "studio.activity.material.document"
                : "studio.activity.material.video",
            )}
          </label>
        ))}
      </div>

      <div>
        <label htmlFor={`${idPrefix}-material-title`} className={labelClass}>
          {t("studio.activity.material.title", "Material title")}
        </label>
        <Input
          id={`${idPrefix}-material-title`}
          value={f.materialTitle}
          onChange={(e) => onChange({ materialTitle: e.target.value })}
          className="text-base sm:text-xs"
        />
      </div>

      <div>
        <label htmlFor={`${idPrefix}-material-desc`} className={labelClass}>
          {t("studio.activity.material.description", "Description (optional)")}
        </label>
        <textarea
          id={`${idPrefix}-material-desc`}
          rows={2}
          value={f.materialDescription}
          onChange={(e) => onChange({ materialDescription: e.target.value })}
          className={textareaClass}
        />
      </div>

      <div>
        <label htmlFor={`${idPrefix}-material-file`} className={labelClass}>
          {t("studio.activity.material.file", "File")}
        </label>
        <input
          id={`${idPrefix}-material-file`}
          type="file"
          accept={MATERIAL_ACCEPT[f.materialKind]}
          disabled={isUploading}
          onChange={(e) => void handleFile(e.target.files?.[0])}
          className="block w-full text-base sm:text-xs text-text-muted file:mr-3 file:rounded-md file:border-0 file:bg-primary file:px-3 file:py-2 file:text-base sm:file:text-xs file:font-semibold file:text-primary-foreground"
        />
        <p className="mt-1 flex items-center gap-1.5 text-[11px] text-text-muted">
          {status === "uploading" || status === "processing" ? (
            <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
          ) : status === "ready" ? (
            <CheckCircle2 className="h-3 w-3 text-success" aria-hidden="true" />
          ) : status === "failed" ? (
            <AlertCircle className="h-3 w-3 text-danger" aria-hidden="true" />
          ) : (
            <Upload className="h-3 w-3" aria-hidden="true" />
          )}
          {statusText[status]}
        </p>
      </div>

      <label className="flex items-start gap-2 text-[11px] text-text">
        <input
          type="checkbox"
          checked={f.rightsConfirmed}
          onChange={(e) => onChange({ rightsConfirmed: e.target.checked })}
          className="mt-0.5 accent-primary"
        />
        <span>
          {t(
            "studio.activity.material.rights",
            "I own or have the right to publish this material",
          )}
        </span>
      </label>

      {error && (
        <p className="flex items-center gap-1.5 text-[11px] text-danger-accent">
          <AlertCircle className="h-3 w-3 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}
