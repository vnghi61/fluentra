import React, { useRef, useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  ExternalLink,
  FileText,
  Video,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { reachableStorageUrl } from "@/lib/storage-url";

export interface MaterialSources {
  poster_url?: string | undefined;
  video?: { url: string; height: number }[] | undefined;
  document?:
    | {
        url: string;
        preview_url?: string | undefined;
        page_count?: number | undefined;
      }
    | undefined;
}

export interface ExerciseMaterialProps {
  materialKind?: "document" | "video" | undefined;
  title?: string | undefined;
  description?: string | undefined;
  sources?: MaterialSources | undefined;
  isLoading?: boolean;
  isSubmitted: boolean;
  isCorrect?: boolean | null | undefined;
  /**
   * Refetches the lesson. A signed URL expires; a learner who leaves the tab
   * open past its TTL gets a dead video, and one refetch issues a fresh one
   * (WO 20 Stage D trap 1).
   */
  onRefetchLesson?: (() => void) | undefined;
  onSubmit: (done: boolean) => void;
  onContinue: () => void;
}

/**
 * A non-graded course material: a document to read or a video to watch. Opening
 * it is the whole task; "mark as done" completes the activity.
 */
export const ExerciseMaterial: React.FC<ExerciseMaterialProps> = ({
  materialKind,
  title,
  description,
  sources,
  isLoading = false,
  isSubmitted,
  onRefetchLesson,
  onSubmit,
  onContinue,
}) => {
  const { t } = useTranslation();
  // Refetch at most once per mount: a video that fails because its URL expired
  // gets one fresh URL, and a genuinely broken one does not loop.
  const refetchedRef = useRef(false);

  const video = sources?.video ?? [];
  const document = sources?.document;
  const isVideo = materialKind === "video" || video.length > 0;

  // One `src` at a time, not <source> children: when a <source> fails, the
  // error fires on that <source> and never on the <video>, so an expired URL
  // would go unnoticed. 720p first, then 360p, then one refetch.
  const [sourceIndex, setSourceIndex] = useState(0);
  const firstUrl = video[0]?.url;
  const [urlsFor, setUrlsFor] = useState(firstUrl);
  if (firstUrl !== urlsFor) {
    // Fresh URLs arrived (the refetch): start again from the best rendition.
    setUrlsFor(firstUrl);
    setSourceIndex(0);
  }
  const currentSource = video[sourceIndex];

  const handleVideoError = () => {
    if (sourceIndex + 1 < video.length) {
      setSourceIndex(sourceIndex + 1);
      return;
    }
    if (refetchedRef.current || !onRefetchLesson) return;
    refetchedRef.current = true;
    onRefetchLesson();
  };

  return (
    <div className="space-y-6 max-w-2xl mx-auto py-4">
      <div className="space-y-1">
        <div className="flex items-center gap-2 text-primary-accent">
          {isVideo ? (
            <Video className="h-4 w-4" aria-hidden="true" />
          ) : (
            <FileText className="h-4 w-4" aria-hidden="true" />
          )}
          <span className="text-[11px] font-bold uppercase tracking-wider">
            {t(
              isVideo
                ? "runner.material.videoLabel"
                : "runner.material.documentLabel",
            )}
          </span>
        </div>
        <h2 className="text-xl md:text-2xl font-bold text-text">
          {title || t("runner.material.untitled", "Course material")}
        </h2>
        {description && (
          <p className="text-sm text-text-muted">{description}</p>
        )}
      </div>

      {isVideo ? (
        <video
          controls
          playsInline
          preload="metadata"
          poster={
            sources?.poster_url
              ? reachableStorageUrl(sources.poster_url)
              : undefined
          }
          src={
            currentSource ? reachableStorageUrl(currentSource.url) : undefined
          }
          onError={handleVideoError}
          className="w-full rounded-xl border border-border-subtle bg-black"
        />
      ) : document ? (
        <div className="space-y-4">
          {document.preview_url && (
            <img
              src={reachableStorageUrl(document.preview_url)}
              alt={title ?? ""}
              className="w-full max-h-96 rounded-xl border border-border-subtle object-contain bg-surface-muted/40"
            />
          )}
          <div className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
            {typeof document.page_count === "number" && (
              <span>
                {t("runner.material.pages", "{{count}} pages", {
                  count: document.page_count,
                })}
              </span>
            )}
          </div>
          <a
            href={reachableStorageUrl(document.url)}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-border-subtle px-4 py-2 text-sm font-semibold text-primary-accent hover:bg-surface-muted"
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            {t("runner.material.openDocument", "Open document")}
          </a>
        </div>
      ) : (
        <p className="rounded-xl border border-border-subtle bg-surface-muted/40 p-4 text-sm text-text-muted">
          {t(
            "runner.material.unavailable",
            "This material is not available right now.",
          )}
        </p>
      )}

      {isSubmitted ? (
        <div
          role="status"
          className="rounded-xl border border-success bg-success/10 p-4 text-sm font-medium text-success-accent flex items-center gap-2"
        >
          <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
          {t("runner.material.done", "Marked as done.")}
        </div>
      ) : (
        <p className="text-xs text-text-muted">
          {t(
            "runner.material.hint",
            "Open the material, then mark it as done to complete this chapter.",
          )}
        </p>
      )}

      <div className="flex justify-end pt-2">
        {isSubmitted ? (
          <Button
            size="lg"
            disabled={isLoading}
            onClick={onContinue}
            className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
          >
            {t("runner.continueBtn", "Continue")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        ) : (
          <Button
            size="lg"
            disabled={isLoading || (!document && !isVideo)}
            onClick={() => onSubmit(true)}
            className="w-full sm:w-auto min-w-[160px] min-h-[44px] font-bold gap-2"
          >
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
            {t("runner.material.markDone", "Đã xem xong")}
          </Button>
        )}
      </div>
    </div>
  );
};
