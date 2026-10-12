import React, { useState, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Camera, Loader2, UploadCloud, AlertCircle } from "lucide-react";
import { accountApi } from "../api/accountApi";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

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
      setError(
        t(
          "account.pleaseSelectAJpegPng",
          "Please select a JPEG, PNG, or WebP image.",
        ),
      );
      return;
    }

    if (file.size > MAX_BYTES) {
      setError(
        t("account.imageSizeExceedsThe5", "Image size exceeds the 5 MB limit."),
      );
      return;
    }

    setSelectedFile(file);
    const url = URL.createObjectURL(file);
    // createObjectURL only ever returns blob: URLs. Checking says so where the
    // value reaches <img src>, and a value that is not one is never rendered.
    if (!url.startsWith("blob:")) {
      URL.revokeObjectURL(url);
      return;
    }
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
          : t(
              "account.failedToUploadAvatarPlease",
              "Failed to upload avatar. Please try again.",
            ),
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
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !isUploading) handleClose();
      }}
    >
      <DialogContent className="max-w-md" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Camera
              aria-hidden="true"
              className="h-5 w-5 text-primary-accent"
            />
            {t("account.updateProfileAvatar", "Update Profile Avatar")}
          </DialogTitle>
        </DialogHeader>

        <DialogBody className="space-y-6">
          {error && (
            <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm leading-relaxed text-danger-accent">
              <AlertCircle
                aria-hidden="true"
                className="h-5 w-5 shrink-0 text-danger-accent"
              />
              <span>{error}</span>
            </div>
          )}

          <div className="flex flex-col items-center justify-center space-y-4">
            {previewUrl ? (
              <div className="relative h-36 w-36 overflow-hidden rounded-full border-4 border-primary/30 bg-surface-muted">
                <img
                  src={previewUrl}
                  alt={t("account.avatarPreview", "Avatar preview")}
                  className="h-full w-full object-cover"
                />
              </div>
            ) : (
              <div
                onClick={() => fileInputRef.current?.click()}
                className="flex h-36 w-36 cursor-pointer flex-col items-center justify-center rounded-full border-2 border-dashed border-border-subtle bg-surface-muted/50 hover:border-primary/50 hover:bg-surface-muted transition-all text-text-muted hover:text-primary-accent"
              >
                <UploadCloud aria-hidden="true" className="h-8 w-8 mb-1" />
                <span className="text-sm font-medium">
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
                {previewUrl
                  ? t("account.chooseDifferentImage", "Choose different image")
                  : t("account.browseComputer", "Browse computer")}
              </Button>
              <p className="text-sm text-text-muted">
                JPEG, PNG, or WebP. Max size: 5 MB.
              </p>
            </div>
          </div>
        </DialogBody>

        <DialogFooter>
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
                <Loader2
                  aria-hidden="true"
                  className="mr-2 h-4 w-4 animate-spin"
                />
                {t("account.uploadingDirectly", "Uploading directly...")}
              </>
            ) : (
              t("account.saveAvatar", "Save Avatar")
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
