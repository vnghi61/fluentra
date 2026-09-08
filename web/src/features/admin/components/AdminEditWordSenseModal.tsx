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
        <div className="flex items-center justify-between px-6 py-4 border-b border-border bg-muted/40">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10 text-primary">
              <Edit3 className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-foreground">
                Edit Sense:{" "}
                <span className="text-primary font-mono">{item.term}</span>
              </h2>
              <p className="text-xs text-muted-foreground mt-0.5">
                Sense ID: {item.word_sense_id || "None"}
              </p>
            </div>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
          >
            <X className="w-5 h-5" />
          </Button>
        </div>

        {/* Form Body */}
        <form
          onSubmit={(e) => void handleSubmit(e)}
          className="p-6 overflow-y-auto space-y-4 flex-1"
        >
          {error && (
            <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-2 text-xs">
              <AlertCircle className="w-4 h-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {/* English Definition */}
          <div>
            <label className="text-xs font-medium text-muted-foreground block mb-1">
              English Definition <span className="text-destructive">*</span>
            </label>
            <textarea
              value={definition}
              onChange={(e) => setDefinition(e.target.value)}
              required
              rows={2}
              className="w-full text-base rounded-md border border-input bg-background p-2.5 shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
              placeholder={t("admin.definitionPlaceholder")}
            />
          </div>

          {/* Vietnamese Gloss */}
          <div>
            <label className="text-xs font-medium text-muted-foreground block mb-1">
              Vietnamese Meaning / Gloss (definition_vi)
            </label>
            <input
              type="text"
              value={definitionVi}
              onChange={(e) => setDefinitionVi(e.target.value)}
              className="w-full h-11 rounded-md border border-input bg-background px-3 py-1 text-base shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
              placeholder={t("admin.definitionViPlaceholder")}
            />
          </div>

          {/* Topic */}
          <div>
            <label className="text-xs font-medium text-muted-foreground block mb-1">
              {t("admin.topicDomain")}
            </label>
            <input
              type="text"
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
              className="w-full h-11 rounded-md border border-input bg-background px-3 py-1 text-base shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
              placeholder={t("admin.topicPlaceholder")}
            />
          </div>

          {/* Examples */}
          <div className="space-y-3 pt-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-foreground">
                {t("admin.exampleSentences")}
              </label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleAddExample}
                className="min-h-11 px-4 text-xs"
              >
                <Plus className="w-3.5 h-3.5 mr-1" />
                {t("admin.addExample")}
              </Button>
            </div>

            {examples.map((ex, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg border border-border bg-muted/20 space-y-2 relative"
              >
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-semibold text-muted-foreground">
                    Example #{idx + 1}
                  </span>
                  {examples.length > 1 && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRemoveExample(idx)}
                      className="min-h-11 min-w-11 p-0 text-muted-foreground hover:text-destructive"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
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
                  className="w-full h-11 rounded border border-input bg-background px-2.5 text-base focus:outline-none focus:ring-1 focus:ring-ring"
                />
                <input
                  type="text"
                  value={ex.sentence_vi || ""}
                  onChange={(e) =>
                    handleExampleChange(idx, "sentence_vi", e.target.value)
                  }
                  placeholder={t("admin.exampleVietnamese")}
                  className="w-full h-11 rounded border border-input bg-background px-2.5 text-base focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>
            ))}
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-2 pt-4 border-t border-border">
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
                <Loader2 className="w-4 h-4 animate-spin mr-1.5" />
              ) : (
                <Check className="w-4 h-4 mr-1.5" />
              )}
              {t("common.saveChanges")}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
