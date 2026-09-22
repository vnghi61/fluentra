import React, { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, Loader2, UploadCloud } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  RESOURCE_ACCEPT,
  RESOURCE_MAX_BYTES,
} from "../api/resourceApi";
import { useUploadResource } from "../hooks/useResources";

export interface ResourceUploadProps {
  /** Called with the new resource's id once it is confirmed and queued. */
  onUploaded?: ((id: string) => void) | undefined;
}

/**
 * The one way a file enters the learner's library.
 *
 * The bytes go straight to the object store through the signed URL from the
 * intent; the API only ever sees the confirmation. Validation is queued at that
 * point, so the screen moves to the detail page rather than pretending to know
 * the verdict.
 */
export function ResourceUpload({
  onUploaded,
}: ResourceUploadProps): React.JSX.Element {
  const { t } = useTranslation();
  const upload = useUploadResource();
  const inputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [dragActive, setDragActive] = useState(false);

  const handleFile = (file: File | undefined) => {
    if (!file) return;
    setError(null);
    // Refused here rather than after a 50 MB upload: the server would reject
    // it, and the learner would have waited for the answer.
    if (file.size > RESOURCE_MAX_BYTES) {
      setError(t("resources.errorTooLarge", "Files must be 50 MB or smaller."));
      return;
    }
    upload.mutate(file, {
      onSuccess: (id) => onUploaded?.(id),
      onError: (err) => {
        setError(
          err instanceof Error
            ? err.message
            : t("resources.errorUpload", "The upload could not be completed."),
        );
      },
    });
  };

  return (
    <div className="space-y-2">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragActive(true);
        }}
        onDragLeave={() => setDragActive(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragActive(false);
          handleFile(e.dataTransfer.files?.[0]);
        }}
        className={
          "flex flex-col items-center gap-3 rounded-2xl border-2 border-dashed p-6 text-center transition-colors " +
          (dragActive
            ? "border-primary bg-primary/5"
            : "border-border-subtle bg-surface-muted/40")
        }
      >
        <input
          ref={inputRef}
          type="file"
          accept={RESOURCE_ACCEPT}
          className="sr-only"
          onChange={(e) => {
            handleFile(e.target.files?.[0]);
            // Reset so choosing the same file twice fires change again.
            e.target.value = "";
          }}
        />
        {upload.isPending ? (
          <Loader2
            className="h-8 w-8 animate-spin text-primary"
            aria-hidden="true"
          />
        ) : (
          <UploadCloud
            className="h-8 w-8 text-primary"
            aria-hidden="true"
          />
        )}
        <div className="space-y-1">
          <p className="text-sm font-medium text-text">
            {upload.isPending
              ? t("resources.uploading", "Uploading…")
              : t(
                  "resources.dropHint",
                  "Drag a file here, or choose one from your device.",
                )}
          </p>
          <p className="text-xs text-text-muted">
            {t(
              "resources.acceptHint",
              "PDF, Word, PowerPoint, images, audio or video · up to 50 MB",
            )}
          </p>
        </div>
        <Button
          type="button"
          variant="secondary"
          disabled={upload.isPending}
          onClick={() => inputRef.current?.click()}
          className="min-h-[44px] gap-2"
        >
          <UploadCloud className="h-4 w-4" aria-hidden="true" />
          {t("resources.chooseFile", "Choose a file")}
        </Button>
      </div>

      {error && (
        <p
          role="alert"
          className="flex items-start gap-2 text-sm text-danger-accent"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
    </div>
  );
}
