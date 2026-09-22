import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  FileText,
  Image as ImageIcon,
  Link2,
  Music,
  Video,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { reachableStorageUrl } from "@/lib/storage-url";
import { cn } from "@/lib/utils";
import {
  RESOURCE_QUOTA_BYTES,
  RESOURCE_QUOTA_COUNT,
  resourceFamily,
  renditionsPending,
  type Resource,
} from "../api/resourceApi";
import {
  familyLabelKey,
  formatBytes,
  statusLabelKey,
  statusTone,
} from "../model/resourceStatus";
import { quotaUsage } from "../hooks/useResources";

function KindIcon({ family }: { family: string }): React.JSX.Element {
  const className = "h-5 w-5 text-primary-accent";
  switch (family) {
    case "image":
      return <ImageIcon className={className} aria-hidden="true" />;
    case "audio":
      return <Music className={className} aria-hidden="true" />;
    case "video":
      return <Video className={className} aria-hidden="true" />;
    default:
      return <FileText className={className} aria-hidden="true" />;
  }
}

/** The thumbnail when the pipeline has made one, the family's icon otherwise. */
function Thumbnail({ resource }: { resource: Resource }): React.JSX.Element {
  const thumbnail = resource.renditions?.find((r) => r.kind === "thumbnail");
  const display = resource.renditions?.find((r) => r.kind === "display");
  const url = thumbnail?.url ?? display?.url;
  if (url) {
    return (
      <img
        src={reachableStorageUrl(url)}
        alt=""
        className="h-12 w-12 shrink-0 rounded-lg border border-border-subtle object-cover"
      />
    );
  }
  return (
    <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border border-border-subtle bg-surface-muted">
      {resource.kind === "url" ? (
        <Link2 className="h-5 w-5 text-primary-accent" aria-hidden="true" />
      ) : (
        <KindIcon family={resourceFamily(resource.detected_mime ?? "")} />
      )}
    </span>
  );
}

export interface ResourceQuotaProps {
  resources: Resource[];
}

/** "12 of 50 files · 24 MB of 250 MB", with the fuller of the two as the bar. */
export function ResourceQuota({
  resources,
}: ResourceQuotaProps): React.JSX.Element {
  const { t } = useTranslation();
  const { count, bytes } = quotaUsage(resources);
  const countRatio = count / RESOURCE_QUOTA_COUNT;
  const byteRatio = bytes / RESOURCE_QUOTA_BYTES;
  const ratio = Math.max(countRatio, byteRatio);

  return (
    <div className="space-y-1.5">
      <Progress
        value={Math.round(ratio * 100)}
        max={100}
        variant={ratio >= 0.9 ? "danger" : ratio >= 0.7 ? "warning" : "primary"}
        aria-label={t("resources.quotaLabel", "Storage used")}
      />
      <p className="text-xs text-text-muted">
        {t("resources.quota", {
          files: count,
          filesMax: RESOURCE_QUOTA_COUNT,
          size: formatBytes(bytes),
          sizeMax: formatBytes(RESOURCE_QUOTA_BYTES),
          defaultValue: `${count} of ${RESOURCE_QUOTA_COUNT} files · ${formatBytes(bytes)} of ${formatBytes(RESOURCE_QUOTA_BYTES)}`,
        })}
      </p>
    </div>
  );
}

export interface ResourceListProps {
  resources: Resource[];
}

export function ResourceList({
  resources,
}: ResourceListProps): React.JSX.Element {
  const { t, i18n } = useTranslation();

  return (
    <ul className="space-y-2">
      {resources.map((resource) => {
        const family = resource.kind === "url" ? "url" : resourceFamily(resource.detected_mime ?? "");
        const processing =
          resource.status === "pending" ||
          resource.status === "uploaded" ||
          renditionsPending(resource);
        const label = processing
          ? t("resources.statusProcessing", "Processing")
          : t(statusLabelKey(resource.status), resource.status);
        const date = new Date(resource.created_at).toLocaleDateString(
          i18n.language.startsWith("vi") ? "vi-VN" : "en-US",
          { year: "numeric", month: "short", day: "numeric" },
        );

        return (
          <li key={resource.id}>
            <Link
              to="/my-resources/$resourceId"
              params={{ resourceId: resource.id }}
              className="flex min-h-[44px] items-center gap-3 rounded-xl border border-border-subtle bg-surface-card p-3 transition-colors hover:border-primary/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            >
              <Thumbnail resource={resource} />
              <span className="min-w-0 flex-1 space-y-0.5">
                <span className="block truncate text-sm font-medium text-text">
                  {resource.title || resource.original_filename}
                </span>
                <span className="block truncate text-xs text-text-muted">
                  {t(familyLabelKey(resource.detected_mime ?? ""), family)}
                  {" · "}
                  {formatBytes(resource.byte_size)}
                  {" · "}
                  {date}
                </span>
                {resource.status === "rejected" && resource.failure_reason && (
                  <span className="block truncate text-xs text-danger-accent">
                    {resource.failure_reason}
                  </span>
                )}
              </span>
              <Badge
                variant={statusTone(resource.status)}
                className={cn(processing && "animate-pulse")}
              >
                {label}
              </Badge>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
