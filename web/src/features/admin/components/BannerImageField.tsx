import * as React from "react";
import { useTranslation } from "react-i18next";
import { ImagePlus, Loader2, RefreshCw, Trash2 } from "lucide-react";

import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import {
  BANNER_IMAGE,
  isBannerImageType,
  useUploadBannerImage,
} from "@/features/banners";
import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";

export interface BannerImageValue {
  key: string;
  url: string;
}

export interface BannerImageFieldProps {
  id: string;
  value: BannerImageValue | null;
  onChange: (value: BannerImageValue | null) => void;
  /** Tells the form an upload is in flight, so it is not saved half-done. */
  onBusyChange?: ((busy: boolean) => void) | undefined;
  disabled?: boolean | undefined;
  /** Replaces the generic size and format hint (a slide names its shape). */
  hint?: string | undefined;
  /**
   * The preview frame: `slide` matches the dashboard carousel's 3:1 crop so
   * the administrator sees what learners will; `wide` is 16:9.
   */
  aspect?: "wide" | "slide" | undefined;
}

/**
 * Picks, uploads and previews a banner image. The file goes straight to
 * storage and the server then checks, scales and re-encodes it; this field
 * only holds the resulting key until the form is saved.
 */
export function BannerImageField({
  id,
  value,
  onChange,
  onBusyChange,
  disabled = false,
  hint,
  aspect = "wide",
}: BannerImageFieldProps): React.JSX.Element {
  const { t } = useTranslation();
  const upload = useUploadBannerImage();
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [dragging, setDragging] = React.useState(false);
  const busy = upload.isPending;
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;

  const pick = () => inputRef.current?.click();

  const accept = (file: File | undefined) => {
    if (!file || busy || disabled) return;
    setError(null);
    if (!isBannerImageType(file.type)) {
      setError(t("adminBanners.image.typeError"));
      return;
    }
    if (file.size > BANNER_IMAGE.maxBytes) {
      setError(t("adminBanners.image.sizeError"));
      return;
    }
    onBusyChange?.(true);
    upload.mutate(file, {
      onSuccess: (image) =>
        onChange({ key: image.image_key, url: image.image_url }),
      onError: (err) =>
        setError(
          err instanceof ApiError && err.problem.status === 422
            ? t("adminBanners.image.rejected")
            : t("adminBanners.image.uploadFailed"),
        ),
      onSettled: () => onBusyChange?.(false),
    });
  };

  const onDrop = (event: React.DragEvent) => {
    event.preventDefault();
    setDragging(false);
    accept(event.dataTransfer.files[0]);
  };

  return (
    <div className="space-y-2">
      <input
        ref={inputRef}
        id={id}
        type="file"
        accept={BANNER_IMAGE.accept}
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
        disabled={disabled || busy}
        onChange={(event) => {
          accept(event.target.files?.[0]);
          // Choosing the same file again must fire change again.
          event.target.value = "";
        }}
      />

      {value ? (
        <div className="overflow-hidden rounded-xl border border-border-subtle bg-surface-card">
          <div
            className={cn(
              "relative bg-surface-muted",
              aspect === "slide" ? "aspect-[3/1]" : "aspect-[16/9]",
            )}
          >
            <img
              src={reachableStorageUrl(value.url)}
              alt={t("adminBanners.image.previewAlt")}
              decoding="async"
              className="absolute inset-0 h-full w-full object-cover"
            />
            {busy && (
              <UploadingOverlay label={t("adminBanners.image.uploading")} />
            )}
          </div>
          <div className="flex flex-wrap gap-2 p-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={pick}
              disabled={disabled || busy}
              aria-describedby={hintId}
            >
              <RefreshCw aria-hidden="true" className="h-4 w-4" />
              {t("adminBanners.image.replace")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setError(null);
                onChange(null);
              }}
              disabled={disabled || busy}
              className="text-danger-accent hover:text-danger-accent"
            >
              <Trash2 aria-hidden="true" className="h-4 w-4" />
              {t("adminBanners.image.remove")}
            </Button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={pick}
          disabled={disabled || busy}
          aria-describedby={error ? `${hintId} ${errorId}` : hintId}
          onDragOver={(event) => {
            event.preventDefault();
            if (!dragging) setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          className={cn(
            "relative flex min-h-32 w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed px-4 py-6 text-center transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none",
            dragging
              ? "border-primary bg-primary/5"
              : "border-border bg-surface-muted/40 hover:border-primary/60 hover:bg-primary/5",
            (disabled || busy) && "cursor-not-allowed opacity-70",
          )}
        >
          {busy ? (
            <Loader2
              aria-hidden="true"
              className="h-6 w-6 animate-spin text-primary-accent motion-reduce:animate-none"
            />
          ) : (
            <ImagePlus
              aria-hidden="true"
              className="h-6 w-6 text-primary-accent"
            />
          )}
          <span className="text-sm font-medium text-text">
            {busy
              ? t("adminBanners.image.uploading")
              : t("adminBanners.image.choose")}
          </span>
          <span className="text-sm text-text-muted">
            {t("adminBanners.image.dropHint")}
          </span>
        </button>
      )}

      <p id={hintId} className="text-sm text-text-muted">
        {hint ?? t("adminBanners.image.hint")}
      </p>
      {error && (
        <p
          id={errorId}
          role="alert"
          className="text-sm font-medium text-danger-accent"
        >
          {error}
        </p>
      )}
    </div>
  );
}

function UploadingOverlay({ label }: { label: string }): React.JSX.Element {
  return (
    <div className="absolute inset-0 flex items-center justify-center gap-2 bg-surface-card/70 text-sm font-medium text-text backdrop-blur-sm">
      <Loader2
        aria-hidden="true"
        className="h-5 w-5 animate-spin motion-reduce:animate-none"
      />
      <span role="status">{label}</span>
    </div>
  );
}
