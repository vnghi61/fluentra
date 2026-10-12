import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import {
  AlertCircle,
  ChevronDown,
  Download,
  ExternalLink,
  FileText,
  Loader2,
  RefreshCw,
  Sparkles,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { reachableStorageUrl } from "@/lib/storage-url";
import {
  resourceFamily,
  renditionsPending,
  type Resource,
  type ResourceRendition,
} from "../api/resourceApi";
import {
  familyLabelKey,
  formatBytes,
  statusLabelKey,
  statusTone,
} from "../model/resourceStatus";
import {
  useDeleteResource,
  useStartResourcePractice,
  useResourcePractice,
  useRevalidateResource,
} from "../hooks/useResources";

function findRendition(
  resource: Resource,
  kinds: string[],
): ResourceRendition | undefined {
  return resource.renditions?.find((r) => kinds.includes(r.kind) && r.url);
}

/** The visual or playable form of the original, when the pipeline has made one. */
function ResourcePreview({
  resource,
}: {
  resource: Resource;
}): React.JSX.Element | null {
  const { t } = useTranslation();
  const family =
    resource.kind === "url"
      ? "url"
      : resourceFamily(resource.detected_mime ?? "");

  if (resource.kind === "url") {
    return resource.source_url ? (
      <a
        href={resource.source_url}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex min-h-[44px] max-w-full items-center gap-2 text-sm font-medium text-primary-accent underline"
      >
        <ExternalLink className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span className="min-w-0 break-all">{resource.source_url}</span>
      </a>
    ) : null;
  }

  if (family === "image") {
    const image = findRendition(resource, ["display", "thumbnail"]);
    return image?.url ? (
      <img
        src={reachableStorageUrl(image.url)}
        alt={resource.title || resource.original_filename}
        className="max-h-96 w-full rounded-xl border border-border-subtle object-contain"
      />
    ) : null;
  }

  if (family === "audio") {
    const audio = findRendition(resource, ["audio_web"]);
    return audio?.url ? (
      <audio
        controls
        preload="none"
        src={reachableStorageUrl(audio.url)}
        className="w-full"
      >
        {t(
          "resources.audioUnsupported",
          "Your browser cannot play this audio.",
        )}
      </audio>
    ) : null;
  }

  if (family === "video") {
    const video = findRendition(resource, ["video_360p", "video_720p"]);
    const poster = findRendition(resource, ["poster", "thumbnail"]);
    return video?.url ? (
      <video
        controls
        playsInline
        preload="metadata"
        src={reachableStorageUrl(video.url)}
        {...(poster?.url && { poster: reachableStorageUrl(poster.url) })}
        className="max-h-96 w-full rounded-xl border border-border-subtle"
      >
        {t(
          "resources.videoUnsupported",
          "Your browser cannot play this video.",
        )}
      </video>
    ) : null;
  }

  // A document: the first page as an image, made by the pipeline.
  const preview = findRendition(resource, ["preview", "thumbnail"]);
  return preview?.url ? (
    <img
      src={reachableStorageUrl(preview.url)}
      alt={t("resources.previewAlt", "First page preview")}
      className="max-h-96 w-full rounded-xl border border-border-subtle object-contain"
    />
  ) : null;
}

export interface ResourceDetailProps {
  resource: Resource;
}

export function ResourceDetail({
  resource,
}: ResourceDetailProps): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const remove = useDeleteResource();
  const startPractice = useStartResourcePractice();
  const revalidate = useRevalidateResource();
  const [confirmOpen, setConfirmOpen] = useState(false);

  const family =
    resource.kind === "url"
      ? "url"
      : resourceFamily(resource.detected_mime ?? "");
  const processing =
    resource.status === "pending" ||
    resource.status === "uploaded" ||
    renditionsPending(resource);
  const extraction = resource.extraction;
  const classification = resource.classification;

  const handleDelete = () => {
    remove.mutate(resource.id, {
      onSuccess: () => void navigate({ to: "/my-resources" }),
    });
  };

  // Generation needs the text and at least one spine node, which is exactly
  // what the server checks; the button says so instead of failing a request.
  const canGeneratePractice =
    resource.status === "validated" &&
    resource.classification?.learning_suitable === true &&
    (resource.extraction?.char_count ?? 0) > 0 &&
    (resource.classification?.nodes?.length ?? 0) > 0;
  const practice = useResourcePractice(resource.id, canGeneratePractice);
  const practiceStatus = canGeneratePractice
    ? practice.data?.status
    : undefined;

  const handleStartPractice = () => {
    if (practiceStatus === "ready" || practiceStatus === "generating") {
      void navigate({
        to: "/practice/resource/$resourceId",
        params: { resourceId: resource.id },
      });
      return;
    }
    startPractice.mutate(resource.id, {
      onSuccess: () =>
        void navigate({
          to: "/practice/resource/$resourceId",
          params: { resourceId: resource.id },
        }),
    });
  };

  return (
    <div className="space-y-4">
      {/* Status line: every state says something, including why a rejection was one. */}
      {processing && (
        <div
          role="status"
          className="flex items-start gap-2 rounded-xl border border-border-subtle bg-surface-muted/50 p-3 text-base leading-relaxed md:text-sm text-text-muted"
        >
          <Loader2
            className="mt-0.5 h-5 w-5 shrink-0 animate-spin text-primary"
            aria-hidden="true"
          />
          <span>
            {t(
              "resources.processingNote",
              "We are preparing this file — preview, text and level. The pipeline runs in the background, so a few minutes is normal.",
            )}
          </span>
        </div>
      )}

      {resource.status === "rejected" && resource.failure_reason && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/5 p-3 text-base leading-relaxed md:text-sm text-danger-accent"
        >
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
          <span className="min-w-0 break-words">{resource.failure_reason}</span>
        </div>
      )}

      {resource.status === "failed" && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-warning/30 bg-warning/5 p-3 text-base leading-relaxed md:text-sm text-warning-accent"
        >
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
          <span className="min-w-0 break-words">
            {t(
              "resources.failedNote",
              "Something went wrong on our side while reading this file. Upload it again to retry.",
            )}
          </span>
        </div>
      )}

      <Card>
        <CardContent className="space-y-4 p-4">
          <ResourcePreview resource={resource} />

          <dl className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            <div className="flex items-center justify-between gap-2">
              <dt className="text-text-muted">
                {t("resources.fieldKind", "Type")}
              </dt>
              <dd className="font-medium text-text">
                {t(familyLabelKey(resource.detected_mime ?? ""), family)}
              </dd>
            </div>
            <div className="flex items-center justify-between gap-2">
              <dt className="text-text-muted">
                {t("resources.fieldSize", "Size")}
              </dt>
              <dd className="font-medium text-text">
                {formatBytes(resource.byte_size)}
              </dd>
            </div>
            <div className="flex items-center justify-between gap-2">
              <dt className="text-text-muted">
                {t("resources.fieldStatus", "Status")}
              </dt>
              <dd>
                <Badge variant={statusTone(resource.status)}>
                  {resource.status === "validated" &&
                  resource.processing_status === "failed"
                    ? t("resources.renderFailed", "Preview unavailable")
                    : processing
                      ? t("resources.statusProcessing", "Processing")
                      : t(statusLabelKey(resource.status), resource.status)}
                </Badge>
              </dd>
            </div>
            {resource.download_url && (
              <div className="flex items-center justify-between gap-2">
                <dt className="text-text-muted">
                  {t("resources.fieldOriginal", "Original")}
                </dt>
                <dd>
                  <a
                    href={reachableStorageUrl(resource.download_url)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex min-h-[44px] items-center gap-1.5 font-medium text-primary-accent underline"
                  >
                    <Download className="h-4 w-4" aria-hidden="true" />
                    {t("resources.openFile", "Open")}
                  </a>
                </dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      {classification &&
        (classification.cefr_estimate ||
          classification.skill ||
          (classification.nodes?.length ?? 0) > 0) && (
          <Card>
            <CardContent className="space-y-3 p-4">
              <h2 className="text-lg font-semibold leading-snug text-text">
                {t("resources.classificationTitle", "What we found in it")}
              </h2>
              <div className="flex flex-wrap items-center gap-2">
                {classification.cefr_estimate && (
                  <Badge variant="primary">
                    {t("resources.cefrEstimate", {
                      level: classification.cefr_estimate,
                      defaultValue: `Around ${classification.cefr_estimate}`,
                    })}
                  </Badge>
                )}
                {classification.skill && (
                  <Badge variant="secondary">{classification.skill}</Badge>
                )}
                {classification.nodes?.map((node) => (
                  <Badge key={node.code} variant="outline">
                    {node.label}
                  </Badge>
                ))}
              </div>
              <p className="text-sm text-text-muted">
                {t(
                  "resources.estimateNote",
                  "The level is an estimate, not a placement result.",
                )}
              </p>
            </CardContent>
          </Card>
        )}

      {extraction && (
        <details className="group rounded-xl border border-border-subtle bg-surface-card">
          <summary className="flex min-h-[44px] cursor-pointer list-none items-center justify-between gap-2 p-4 text-sm font-medium text-text">
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <FileText
                className="h-4 w-4 shrink-0 text-primary-accent"
                aria-hidden="true"
              />
              {extraction.source === "transcript"
                ? t("resources.transcriptTitle", "Transcript")
                : t("resources.extractedTitle", "Extracted text")}
              <span className="text-xs font-normal text-text-muted">
                {t("resources.charCount", {
                  chars: extraction.char_count,
                  defaultValue: `${extraction.char_count} characters`,
                })}
              </span>
            </span>
            <ChevronDown
              className="h-4 w-4 shrink-0 transition-transform group-open:rotate-180"
              aria-hidden="true"
            />
          </summary>
          <div className="border-t border-border-subtle p-4">
            <p className="max-w-prose whitespace-pre-wrap break-words text-base leading-relaxed text-text-muted">
              {extraction.excerpt}
            </p>
            {extraction.truncated && (
              <p className="mt-2 text-sm text-text-muted">
                {t(
                  "resources.truncatedNote",
                  "Only the beginning of the file is shown here.",
                )}
              </p>
            )}
          </div>
        </details>
      )}

      {/* Practice generated from this file (WO 21 Stage B). */}
      <Card>
        <CardContent className="space-y-2 p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0 space-y-0.5">
              <h2 className="flex items-center gap-2 text-lg font-semibold leading-snug text-text">
                <Sparkles
                  className="h-5 w-5 shrink-0 text-primary-accent"
                  aria-hidden="true"
                />
                {t("resources.practiceTitle", "Practice from this file")}
              </h2>
              <p className="text-base leading-relaxed text-text-muted md:text-sm">
                {t(
                  "resources.practiceDesc",
                  "Ten exercises built from its text. They are private to you and never enter the shared question bank.",
                )}
              </p>
            </div>
            <Button
              disabled={!canGeneratePractice}
              isLoading={startPractice.isPending}
              onClick={handleStartPractice}
              className="gap-2 sm:shrink-0"
            >
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              {practiceStatus === "ready"
                ? t("resources.practiceReadyBtn", "Practice now")
                : practiceStatus === "generating"
                  ? t(
                      "resources.practiceGeneratingBtn",
                      "View generation progress",
                    )
                  : t("resources.practiceBtn", "Create practice")}
            </Button>
          </div>
          {!canGeneratePractice && (
            <p className="text-base leading-relaxed text-text-muted md:text-sm">
              {classification?.learning_suitable === false
                ? t(
                    "resources.unsuitable",
                    "This file is not suitable for English practice. It is kept privately, but no exercises will be created from it.",
                  )
                : classification && classification.learning_suitable == null
                  ? t(
                      "resources.revalidationRequired",
                      "Check this document again to confirm it is suitable for English practice.",
                    )
                  : t(
                      "resources.practiceNotReady",
                      "Once the file has been read and classified, practice can be built from it.",
                    )}
            </p>
          )}
          {practiceStatus === "ready" && (
            <p role="status" className="text-sm text-success-accent">
              {t(
                "resources.practiceReady",
                "Exercises are ready. You can practise without generating them again.",
              )}
            </p>
          )}
          {resource.status === "validated" &&
            (resource.extraction?.char_count ?? 0) > 0 && (
              <Button
                variant="outline"
                isLoading={revalidate.isPending}
                onClick={() => revalidate.mutate(resource.id)}
                className="gap-2"
              >
                <RefreshCw className="h-4 w-4" aria-hidden="true" />
                {t("resources.revalidateBtn", "Check document again")}
              </Button>
            )}
          {revalidate.isSuccess && !classification && (
            <p role="status" className="text-sm text-text-muted">
              {t(
                "resources.revalidationQueued",
                "Content check queued. This page will update automatically.",
              )}
            </p>
          )}
          {revalidate.isError && (
            <p role="alert" className="text-sm text-danger-accent">
              {t(
                "resources.revalidationError",
                "Could not queue the content check. Try again.",
              )}
            </p>
          )}
          {startPractice.isError && (
            <p
              role="alert"
              className="flex items-start gap-2 text-sm text-danger-accent"
            >
              <AlertCircle
                className="mt-0.5 h-4 w-4 shrink-0"
                aria-hidden="true"
              />
              <span>
                {startPractice.error instanceof Error
                  ? startPractice.error.message
                  : t(
                      "resources.practiceError",
                      "Practice could not be created from this file.",
                    )}
              </span>
            </p>
          )}
        </CardContent>
      </Card>

      <div className="flex justify-end pt-2">
        <Button
          variant="outline"
          onClick={() => setConfirmOpen(true)}
          className="gap-2 text-danger-accent"
        >
          <Trash2 className="h-4 w-4" aria-hidden="true" />
          {t("resources.deleteBtn", "Delete")}
        </Button>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => {
          if (!open && remove.isPending) return;
          setConfirmOpen(open);
        }}
        title={t("resources.deleteTitle", "Delete this file?")}
        description={t(
          "resources.deleteDesc",
          "The file and everything we made from it are removed. This cannot be undone.",
        )}
        cancelLabel={t("common.cancel", "Cancel")}
        confirmLabel={t("resources.deleteConfirm", "Delete")}
        destructive
        isPending={remove.isPending}
        keepOpenOnConfirm
        onConfirm={handleDelete}
      />
    </div>
  );
}
