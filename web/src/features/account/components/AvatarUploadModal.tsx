import React, { useState, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Camera, Loader2, UploadCloud, X, AlertCircle } from "lucide-react";
import { accountApi } from "../api/accountApi";
import { Button } from "@/components/ui/button";

interface AvatarUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (newAvatarUrl: string | null | undefined) => void;
}

const MAX_BYTES = 5 * 1024 * 1024; // 5 MB
const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp"];

export const AvatarUploadModal: React.FC<AvatarUploadModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  if (!isOpen) return null;

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setError(null);
    const file = e.target.files?.[0];
    if (!file) return;

    if (!ALLOWED_TYPES.includes(file.type)) {
      setError("Please select a JPEG, PNG, or WebP image.");
      return;
    }

    if (file.size > MAX_BYTES) {
      setError("Image size exceeds the 5 MB limit.");
      return;
    }

    setSelectedFile(file);
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
  };

  const handleUpload = async () => {
    if (!selectedFile) return;

    setIsUploading(true);
    setError(null);

    try {
      // 1. Request presigned intent from API with matching content type
      const intent = await accountApi.requestAvatarUploadIntent(
        selectedFile.type,
      );

      // 2. Upload bytes directly to storage (MinIO/S3), completely bypassing API
      await accountApi.uploadAvatarDirect(intent, selectedFile);

      // 3. Confirm upload with API to trigger thumbnailing & profile update
      const updatedProfile = await accountApi.confirmAvatar({
        object_key: intent.object_key,
      });

      onSuccess(updatedProfile.profile.avatar_url);
      handleClose();
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to upload avatar. Please try again.",
      );
    } finally {
      setIsUploading(false);
    }
  };

  const handleClose = () => {
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl);
    }
    setSelectedFile(null);
    setPreviewUrl(null);
    setError(null);
    onClose();
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="avatar-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-border-subtle pb-4">
          <h2
            id="avatar-modal-title"
            className="text-lg font-semibold text-text flex items-center gap-2"
          >
            <Camera className="h-5 w-5 text-primary-accent" />
            {t("account.updateProfileAvatar", "Update Profile Avatar")}
          </h2>
          <button
            type="button"
            onClick={handleClose}
            disabled={isUploading}
            className="rounded-lg p-1 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3 text-xs text-danger-accent">
            <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <div className="flex flex-col items-center justify-center space-y-4">
          {previewUrl ? (
            <div className="relative h-36 w-36 overflow-hidden rounded-full border-4 border-primary/30 bg-surface-muted">
              <img
                src={previewUrl}
                alt="Avatar preview"
                className="h-full w-full object-cover"
              />
            </div>
          ) : (
            <div
              onClick={() => fileInputRef.current?.click()}
              className="flex h-36 w-36 cursor-pointer flex-col items-center justify-center rounded-full border-2 border-dashed border-border-subtle bg-surface-muted/50 hover:border-primary/50 hover:bg-surface-muted transition-all text-text-muted hover:text-primary-accent"
            >
              <UploadCloud className="h-8 w-8 mb-1" />
              <span className="text-xs font-medium">
                {t("account.chooseFile", "Choose file")}
              </span>
            </div>
          )}

          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={handleFileChange}
            className="hidden"
            id="avatar-file-input"
          />

          <div className="text-center space-y-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => fileInputRef.current?.click()}
              disabled={isUploading}
            >
              {previewUrl ? "Choose different image" : "Browse computer"}
            </Button>
            <p className="text-xs text-text-muted">
              JPEG, PNG, or WebP. Max size: 5 MB.
            </p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-border-subtle pt-4">
          <Button
            type="button"
            variant="ghost"
            onClick={handleClose}
            disabled={isUploading}
          >
            {t("account.cancel", "Cancel")}
          </Button>
          <Button
            type="button"
            onClick={() => {
              void handleUpload();
            }}
            disabled={!selectedFile || isUploading}
          >
            {isUploading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t("account.uploadingDirectly", "Uploading directly...")}
              </>
            ) : (
              "Save Avatar"
            )}
          </Button>
        </div>
      </div>
    </div>
  );
};
