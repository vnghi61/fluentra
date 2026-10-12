import React, { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CheckCircle2,
  FileSpreadsheet,
  FileText,
  Image as ImageIcon,
  Loader2,
  Presentation,
  RefreshCw,
  ShieldCheck,
  UploadCloud,
  Video,
  X,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Textarea } from "@/components/ui/textarea";
import {
  checkMaterialFile,
  formatBytes,
  MATERIAL_ACCEPT,
  resourceApi,
  useMaterialStatus,
  type MaterialUploadState,
} from "@/features/resource";
import { cn } from "@/lib/utils";
import type { ActivityFields } from "../model/activityKinds";

interface Props {
  fields: ActivityFields;
  onChange: (patch: Partial<ActivityFields>) => void;
  idPrefix: string;
}

/** An icon from the file name, since a spreadsheet and a PDF are both "document". */
function FileKindIcon({
  name,
  kind,
}: {
  name: string;
  kind: ActivityFields["materialKind"];
}): React.JSX.Element {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  const Icon =
    kind === "video"
      ? Video
      : kind === "image"
        ? ImageIcon
        : ext === "xls" || ext === "xlsx"
          ? FileSpreadsheet
          : ext === "ppt" || ext === "pptx"
            ? Presentation
            : FileText;
  return <Icon className="h-5 w-5" />;
}

const STATUS_TONE: Record<MaterialUploadState, string> = {
  idle: "text-text-muted",
  uploading: "text-primary-accent",
  processing: "text-warning-accent",
  ready: "text-success-accent",
  failed: "text-danger-accent",
};

/**
 * A course material's file: a drop zone, the upload with its progress, the
 * processing state the server reports, and the title and rights declaration
 * Gate 1 requires. The file never passes through the API: it goes straight to
 * storage on a signed URL, and the server validates its bytes afterwards.
 */
export function MaterialUploader({
  fields: f,
  onChange,
  idPrefix,
}: Props): React.JSX.Element {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const hintId = useId();
  const [error, setError] = useState<string | null>(null);
  const [progress, setProgress] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);

  // While nothing is uploading, the server is the source of truth: a reopened
  // draft asks again rather than trusting the state it was saved with.
  const live = useMaterialStatus(
    progress === null && f.resourceId ? f.resourceId : undefined,
    f.materialKind,
  );
  useEffect(() => {
    if (live && live !== f.materialStatus) onChange({ materialStatus: live });
  }, [live, f.materialStatus, onChange]);
  useEffect(() => () => abortRef.current?.abort(), []);

  const status: MaterialUploadState =
    progress !== null ? "uploading" : (live ?? f.materialStatus);

  const upload = async (file: File | undefined) => {
    if (!file) return;
    setError(null);
    const check = checkMaterialFile(file);
    if (!check.ok) {
      setError(
        check.reason === "size"
          ? t("studio.material.errSize", {
              max: formatBytes(check.maxBytes ?? 0),
            })
          : check.reason === "empty"
            ? t("studio.material.errEmpty")
            : t("studio.material.errType"),
      );
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;
    setProgress(0);
    onChange({
      materialStatus: "uploading",
      materialKind: check.kind,
      materialFileName: file.name,
      materialFileSize: file.size,
    });
    try {
      const intent = await resourceApi.createUploadIntent(
        file.name,
        check.mime,
      );
      await resourceApi.uploadDirect(intent, file, {
        contentType: check.mime,
        onProgress: setProgress,
        signal: controller.signal,
      });
      await resourceApi.confirmUpload(intent.id);
      onChange({
        resourceId: intent.id,
        materialStatus: "processing",
        materialTitle:
          f.materialTitle.trim() || file.name.replace(/\.[^.]+$/, ""),
      });
    } catch (err) {
      const aborted = err instanceof DOMException && err.name === "AbortError";
      onChange({
        materialStatus: f.resourceId ? f.materialStatus : "idle",
        ...(f.resourceId ? {} : { materialFileName: "", materialFileSize: 0 }),
      });
      if (!aborted) {
        setError(
          err instanceof Error && err.message
            ? err.message
            : t("studio.activity.material.uploadFailed"),
        );
      }
    } finally {
      abortRef.current = null;
      setProgress(null);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  const removeFile = () => {
    abortRef.current?.abort();
    onChange({
      resourceId: "",
      materialStatus: "idle",
      materialFileName: "",
      materialFileSize: 0,
    });
  };

  const hasFile = Boolean(f.resourceId) || progress !== null;
  const statusText: Record<MaterialUploadState, string> = {
    idle: t("studio.activity.material.statusIdle"),
    uploading: t("studio.material.uploadingPct", {
      pct: Math.round((progress ?? 0) * 100),
    }),
    processing:
      f.materialKind === "video"
        ? t("studio.activity.material.statusProcessing")
        : f.materialKind === "image"
          ? t("studio.material.statusProcessingImage")
          : t("studio.activity.material.statusProcessingDocument"),
    ready: t("studio.activity.material.statusReady"),
    failed: t("studio.activity.material.statusFailed"),
  };

  const fileInput = (
    <input
      ref={inputRef}
      id={`${idPrefix}-material-file`}
      type="file"
      accept={MATERIAL_ACCEPT}
      className="sr-only"
      aria-describedby={hintId}
      onChange={(e) => void upload(e.target.files?.[0])}
    />
  );

  return (
    <div className="space-y-4">
      {hasFile ? (
        <div className="rounded-xl border border-border-subtle bg-surface-muted/50 p-3">
          <div className="flex items-start gap-3">
            <span
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary-accent"
              aria-hidden="true"
            >
              <FileKindIcon name={f.materialFileName} kind={f.materialKind} />
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-text">
                {f.materialFileName ||
                  f.materialTitle ||
                  t("studio.material.file")}
              </p>
              <p
                className={cn(
                  "mt-0.5 flex items-center gap-1.5 text-sm",
                  STATUS_TONE[status],
                )}
                role="status"
                aria-live="polite"
              >
                {status === "uploading" || status === "processing" ? (
                  <Loader2
                    className="h-4 w-4 shrink-0 animate-spin"
                    aria-hidden="true"
                  />
                ) : status === "ready" ? (
                  <CheckCircle2
                    className="h-4 w-4 shrink-0"
                    aria-hidden="true"
                  />
                ) : status === "failed" ? (
                  <AlertCircle
                    className="h-4 w-4 shrink-0"
                    aria-hidden="true"
                  />
                ) : null}
                <span>
                  {statusText[status]}
                  {f.materialFileSize > 0 &&
                    ` · ${formatBytes(f.materialFileSize)}`}
                </span>
              </p>
            </div>
            <div className="flex shrink-0 items-center">
              {progress === null && (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => inputRef.current?.click()}
                  aria-label={t("studio.material.replace")}
                  title={t("studio.material.replace")}
                >
                  <RefreshCw className="h-4 w-4" aria-hidden="true" />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon"
                onClick={removeFile}
                aria-label={
                  progress !== null
                    ? t("studio.material.cancelUpload")
                    : t("studio.material.removeFile")
                }
                title={
                  progress !== null
                    ? t("studio.material.cancelUpload")
                    : t("studio.material.removeFile")
                }
                className="text-text-muted hover:text-danger"
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
          {progress !== null && (
            <Progress
              value={Math.round(progress * 100)}
              className="mt-3 h-1.5"
              aria-label={t("studio.material.uploadProgress")}
            />
          )}
          {fileInput}
        </div>
      ) : (
        <label
          htmlFor={`${idPrefix}-material-file`}
          onDragOver={(e) => {
            e.preventDefault();
            setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragging(false);
            void upload(e.dataTransfer.files[0]);
          }}
          className={cn(
            "flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center transition-colors focus-within:ring-2 focus-within:ring-primary focus-within:ring-offset-2 focus-within:ring-offset-surface",
            dragging
              ? "border-primary bg-primary/5"
              : "border-border hover:border-primary/60 hover:bg-surface-muted/60",
          )}
        >
          <span
            className="flex h-11 w-11 items-center justify-center rounded-full bg-primary/10 text-primary-accent"
            aria-hidden="true"
          >
            <UploadCloud className="h-5 w-5" />
          </span>
          <span className="text-sm font-semibold text-text">
            {t("studio.material.dropTitle")}
          </span>
          <span id={hintId} className="max-w-sm text-sm text-text-muted">
            {t("studio.material.dropHint")}
          </span>
          {fileInput}
        </label>
      )}

      {error && (
        <p
          role="alert"
          className="flex items-start gap-1.5 text-sm font-medium text-danger-accent"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5 sm:col-span-2">
          <label
            htmlFor={`${idPrefix}-material-title`}
            className="block text-sm font-semibold text-text"
          >
            {t("studio.activity.material.title")}
          </label>
          <Input
            id={`${idPrefix}-material-title`}
            value={f.materialTitle}
            maxLength={160}
            onChange={(e) => onChange({ materialTitle: e.target.value })}
          />
        </div>
        <div className="space-y-1.5 sm:col-span-2">
          <label
            htmlFor={`${idPrefix}-material-desc`}
            className="block text-sm font-semibold text-text"
          >
            {t("studio.activity.material.description")}
          </label>
          <Textarea
            id={`${idPrefix}-material-desc`}
            rows={2}
            maxLength={1000}
            value={f.materialDescription}
            placeholder={t("studio.material.descPlaceholder")}
            onChange={(e) => onChange({ materialDescription: e.target.value })}
          />
        </div>
      </div>

      <div className="flex items-start gap-1 rounded-lg bg-surface-muted/60 pr-3">
        <Checkbox
          id={`${idPrefix}-material-rights`}
          checked={f.rightsConfirmed}
          onCheckedChange={(checked) => onChange({ rightsConfirmed: checked })}
        />
        <label
          htmlFor={`${idPrefix}-material-rights`}
          className="min-w-0 cursor-pointer py-3 text-base leading-relaxed text-text"
        >
          {t("studio.activity.material.rights")}
          <span className="mt-1 flex items-start gap-1.5 text-sm text-text-muted">
            <ShieldCheck
              className="mt-0.5 h-4 w-4 shrink-0"
              aria-hidden="true"
            />
            {t("studio.material.securityNote")}
          </span>
        </label>
      </div>
    </div>
  );
}
