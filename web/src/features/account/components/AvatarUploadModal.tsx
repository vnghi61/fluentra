import React, { useState, useRef } from "react";
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
      // 1. Request presigned intent from API
      const intent = await accountApi.requestAvatarUploadIntent();

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
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <h2
            id="avatar-modal-title"
            className="text-lg font-semibold text-slate-100 flex items-center gap-2"
          >
            <Camera className="h-5 w-5 text-indigo-400" />
            Update Profile Avatar
          </h2>
          <button
            type="button"
            onClick={handleClose}
            disabled={isUploading}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <div className="flex flex-col items-center justify-center space-y-4">
          {previewUrl ? (
            <div className="relative h-36 w-36 overflow-hidden rounded-full border-4 border-indigo-500/30 bg-slate-800">
              <img
                src={previewUrl}
                alt="Avatar preview"
                className="h-full w-full object-cover"
              />
            </div>
          ) : (
            <div
              onClick={() => fileInputRef.current?.click()}
              className="flex h-36 w-36 cursor-pointer flex-col items-center justify-center rounded-full border-2 border-dashed border-slate-700 bg-slate-800/50 hover:border-indigo-500/50 hover:bg-slate-800 transition-all text-slate-400 hover:text-indigo-300"
            >
              <UploadCloud className="h-8 w-8 mb-1" />
              <span className="text-xs font-medium">Choose file</span>
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
            <p className="text-xs text-slate-400">
              JPEG, PNG, or WebP. Max size: 5 MB.
            </p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-slate-800 pt-4">
          <Button
            type="button"
            variant="ghost"
            onClick={handleClose}
            disabled={isUploading}
          >
            Cancel
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
                Uploading directly...
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
