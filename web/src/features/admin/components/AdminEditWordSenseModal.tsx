import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  Check,
  Edit3,
  Loader2,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import {
  adminApi,
  type ExampleSentence,
  type LearnerWordQueueItem,
} from "../api/adminApi";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

interface AdminEditWordSenseModalProps {
  item: LearnerWordQueueItem | null;
  onClose: () => void;
  onSuccess: () => void;
}

export const AdminEditWordSenseModal: React.FC<
  AdminEditWordSenseModalProps
> = ({ item, onClose, onSuccess }) => {
  const { t } = useTranslation();

  const [definition, setDefinition] = useState(item?.definition || "");
  const [definitionVi, setDefinitionVi] = useState(item?.definition_vi || "");
  const [topic, setTopic] = useState(item?.topic || "");
  const [examples, setExamples] = useState<ExampleSentence[]>(
    item?.examples && item.examples.length > 0
      ? [...item.examples]
      : [{ sentence: "", sentence_vi: "" }],
  );

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!item) return null;

  const handleAddExample = () => {
    setExamples((prev) => [...prev, { sentence: "", sentence_vi: "" }]);
  };

  const handleRemoveExample = (idx: number) => {
    setExamples((prev) => prev.filter((_, i) => i !== idx));
  };

  const handleExampleChange = (
    idx: number,
    field: "sentence" | "sentence_vi",
    val: string,
  ) => {
    setExamples((prev) =>
      prev.map((ex, i) => (i === idx ? { ...ex, [field]: val } : ex)),
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!item.word_sense_id) {
      setError("This item has no associated word sense to update.");
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const validExamples = examples.filter(
        (ex) => ex.sentence.trim().length > 0,
      );
      await adminApi.updateWordSense(item.word_sense_id, {
        definition: definition.trim(),
        definition_vi: definitionVi.trim() || null,
        topic: topic.trim() || null,
        examples: validExamples,
      });

      onSuccess();
      onClose();
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : t("admin.failedToUpdateSense"),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 overflow-y-auto">
      <div className="relative w-full max-w-2xl bg-card border border-border rounded-xl shadow-2xl overflow-hidden my-8 flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between gap-3 border-b border-border bg-muted/40 px-4 py-4 md:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <div className="shrink-0 rounded-lg bg-primary/10 p-2 text-primary">
              <Edit3 aria-hidden="true" className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 className="break-words text-lg font-semibold leading-tight text-foreground">
                Edit Sense:{" "}
                <span className="break-all font-mono text-primary">
                  {item.term}
                </span>
              </h2>
              <p className="mt-0.5 break-all text-xs text-muted-foreground">
                Sense ID: {item.word_sense_id || "None"}
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onClose}
            aria-label={t("common.close")}
            className="shrink-0 text-muted-foreground hover:text-foreground"
          >
            <X aria-hidden="true" className="h-5 w-5" />
          </Button>
        </div>

        {/* Form Body */}
        <form
          onSubmit={(e) => void handleSubmit(e)}
          className="flex-1 space-y-4 overflow-y-auto p-4 md:p-6"
        >
          {error && (
            <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-2 text-sm">
              <AlertCircle aria-hidden="true" className="h-5 w-5 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {/* English Definition */}
          <div>
            <label
              htmlFor="edit-sense-definition"
              className="mb-1 block text-sm font-medium text-muted-foreground"
            >
              English Definition <span className="text-destructive">*</span>
            </label>
            <Textarea
              id="edit-sense-definition"
              value={definition}
              onChange={(e) => setDefinition(e.target.value)}
              required
              rows={2}
              placeholder={t("admin.definitionPlaceholder")}
            />
          </div>

          {/* Vietnamese Gloss */}
          <div>
            <label
              htmlFor="edit-sense-definition-vi"
              className="mb-1 block text-sm font-medium text-muted-foreground"
            >
              Vietnamese Meaning / Gloss (definition_vi)
            </label>
            <input
              id="edit-sense-definition-vi"
              type="text"
              value={definitionVi}
              onChange={(e) => setDefinitionVi(e.target.value)}
              className="h-11 w-full rounded-md border border-input bg-background px-3 py-1 text-base shadow-sm placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
              placeholder={t("admin.definitionViPlaceholder")}
            />
          </div>

          {/* Topic */}
          <div>
            <label
              htmlFor="edit-sense-topic"
              className="mb-1 block text-sm font-medium text-muted-foreground"
            >
              {t("admin.topicDomain")}
            </label>
            <input
              id="edit-sense-topic"
              type="text"
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
              className="h-11 w-full rounded-md border border-input bg-background px-3 py-1 text-base shadow-sm placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
              placeholder={t("admin.topicPlaceholder")}
            />
          </div>

          {/* Examples */}
          <div className="space-y-3 pt-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm font-semibold text-foreground">
                {t("admin.exampleSentences")}
              </h3>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleAddExample}
              >
                <Plus aria-hidden="true" className="h-4 w-4" />
                {t("admin.addExample")}
              </Button>
            </div>

            {examples.map((ex, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg border border-border bg-muted/20 space-y-2 relative"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-semibold text-muted-foreground">
                    Example #{idx + 1}
                  </span>
                  {examples.length > 1 && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => handleRemoveExample(idx)}
                      aria-label={t("admin.removeExample")}
                      className="text-muted-foreground hover:text-destructive"
                    >
                      <Trash2 aria-hidden="true" className="h-4 w-4" />
                    </Button>
                  )}
                </div>
                <input
                  type="text"
                  value={ex.sentence}
                  onChange={(e) =>
                    handleExampleChange(idx, "sentence", e.target.value)
                  }
                  placeholder={t("admin.exampleEnglish")}
                  aria-label={t("admin.exampleEnglish")}
                  className="h-11 w-full rounded border border-input bg-background px-2.5 text-base placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
                />
                <input
                  type="text"
                  value={ex.sentence_vi || ""}
                  onChange={(e) =>
                    handleExampleChange(idx, "sentence_vi", e.target.value)
                  }
                  placeholder={t("admin.exampleVietnamese")}
                  aria-label={t("admin.exampleVietnamese")}
                  className="h-11 w-full rounded border border-input bg-background px-2.5 text-base placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-ring md:text-sm"
                />
              </div>
            ))}
          </div>

          {/* Footer */}
          <div className="flex flex-wrap items-center justify-end gap-2 border-t border-border pt-4">
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              disabled={isSubmitting}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <Loader2
                  aria-hidden="true"
                  className="h-4 w-4 animate-spin motion-reduce:animate-none"
                />
              ) : (
                <Check aria-hidden="true" className="h-4 w-4" />
              )}
              {t("common.saveChanges")}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
