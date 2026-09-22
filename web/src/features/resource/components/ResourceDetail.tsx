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
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
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
import { useDeleteResource } from "../hooks/useResources";

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
        className="inline-flex min-h-[44px] items-center gap-2 text-sm font-medium text-primary-accent underline"
      >
        <ExternalLink className="h-4 w-4" aria-hidden="true" />
        {resource.source_url}
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
        {t("resources.audioUnsupported", "Your browser cannot play this audio.")}
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
        {t("resources.videoUnsupported", "Your browser cannot play this video.")}
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

  return (
    <div className="space-y-4">
      {/* Status line: every state says something, including why a rejection was one. */}
      {processing && (
        <div
          role="status"
          className="flex items-start gap-2 rounded-xl border border-border-subtle bg-surface-muted/50 p-3 text-sm text-text-muted"
        >
          <Loader2
            className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary"
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
          className="flex items-start gap-2 rounded-xl border border-danger/30 bg-danger/5 p-3 text-sm text-danger-accent"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>{resource.failure_reason}</span>
        </div>
      )}

      {resource.status === "failed" && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-warning/30 bg-warning/5 p-3 text-sm text-warning-accent"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <span>
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
                  {processing
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
              <h2 className="text-sm font-semibold text-text">
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
              <p className="text-xs text-text-muted">
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
            <span className="flex items-center gap-2">
              <FileText className="h-4 w-4 text-primary-accent" aria-hidden="true" />
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
            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-text-muted">
              {extraction.excerpt}
            </p>
            {extraction.truncated && (
              <p className="mt-2 text-xs text-text-muted">
                {t(
                  "resources.truncatedNote",
                  "Only the beginning of the file is shown here.",
                )}
              </p>
            )}
          </div>
        </details>
      )}

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

      {confirmOpen && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="resource-delete-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm"
        >
          <div className="w-full max-w-sm space-y-4 rounded-2xl border border-border bg-surface-card p-6 shadow-2xl">
            <h2
              id="resource-delete-title"
              className="text-lg font-bold text-text"
            >
              {t("resources.deleteTitle", "Delete this file?")}
            </h2>
            <p className="text-sm text-text-muted">
              {t(
                "resources.deleteDesc",
                "The file and everything we made from it are removed. This cannot be undone.",
              )}
            </p>
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button
                variant="outline"
                disabled={remove.isPending}
                onClick={() => setConfirmOpen(false)}
              >
                {t("common.cancel", "Cancel")}
              </Button>
              <Button
                variant="destructive"
                isLoading={remove.isPending}
                onClick={handleDelete}
              >
                {t("resources.deleteConfirm", "Delete")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
