import React, { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Camera,
  CheckCircle2,
  Globe,
  Loader2,
  Mail,
  User,
  AlertCircle,
} from "lucide-react";
import { accountApi, type UserProfile } from "../api/accountApi";
import { profileFormSchema, type ProfileFormValues } from "../model/schemas";
import { AvatarUploadModal } from "./AvatarUploadModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ProfileSettingsProps {
  initialProfile: UserProfile;
  onProfileUpdated?: (profile: UserProfile) => void;
}

export const ProfileSettings: React.FC<ProfileSettingsProps> = ({
  initialProfile,
  onProfileUpdated,
}) => {
  const [profile, setProfile] = useState<UserProfile>(initialProfile);
  const [isAvatarModalOpen, setIsAvatarModalOpen] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isDirty },
    reset,
  } = useForm<ProfileFormValues>({
    resolver: (zodResolver as (schema: unknown) => never)(profileFormSchema),
    defaultValues: {
      display_name: profile.profile.display_name || "",
      country: profile.profile.country || "",
      timezone:
        profile.profile.timezone ||
        Intl.DateTimeFormat().resolvedOptions().timeZone ||
        "UTC",
      date_of_birth: profile.profile.date_of_birth || "",
    },
  });

  const onSubmit = async (values: ProfileFormValues) => {
    setIsSaving(true);
    setStatusMessage(null);

    try {
      const payload: {
        display_name?: string;
        country?: string;
        timezone?: string;
        date_of_birth?: string;
      } = {
        display_name: values.display_name,
        timezone: values.timezone,
      };
      if (values.country) {
        payload.country = values.country.toUpperCase();
      }
      if (values.date_of_birth) {
        payload.date_of_birth = values.date_of_birth;
      }

      const updated = await accountApi.updateProfile(payload);

      setProfile(updated);
      reset({
        display_name: updated.profile.display_name,
        country: updated.profile.country || "",
        timezone: updated.profile.timezone,
        date_of_birth: updated.profile.date_of_birth || "",
      });
      setStatusMessage({
        type: "success",
        text: "Profile updated successfully.",
      });
      onProfileUpdated?.(updated);
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text: err instanceof Error ? err.message : "Failed to update profile.",
      });
    } finally {
      setIsSaving(false);
    }
  };

  const handleAvatarUploaded = (newAvatarUrl: string | null | undefined) => {
    const updated: UserProfile = {
      ...profile,
      profile: {
        ...profile.profile,
        avatar_url: newAvatarUrl ?? null,
      },
    };
    setProfile(updated);
    setStatusMessage({ type: "success", text: "Avatar updated successfully." });
    onProfileUpdated?.(updated);
  };

  return (
    <div className="space-y-8">
      {/* Avatar Section */}
      <div className="flex flex-col sm:flex-row items-center gap-6 rounded-xl border border-border-subtle bg-surface-card/60 p-6">
        <div className="relative group">
          <div className="h-24 w-24 overflow-hidden rounded-full border-2 border-primary/40 bg-surface-muted flex items-center justify-center text-2xl font-bold text-primary-accent">
            {profile.profile.avatar_url ? (
              <img
                src={profile.profile.avatar_url}
                alt={profile.profile.display_name}
                className="h-full w-full object-cover"
              />
            ) : (
              <span>
                {profile.profile.display_name.charAt(0).toUpperCase()}
              </span>
            )}
          </div>
          <button
            type="button"
            onClick={() => setIsAvatarModalOpen(true)}
            className="absolute inset-0 flex items-center justify-center rounded-full bg-overlay/60 opacity-0 group-hover:opacity-100 transition-opacity text-white"
            title="Change avatar"
          >
            <Camera className="h-6 w-6" />
          </button>
        </div>

        <div className="space-y-2 text-center sm:text-left">
          <h3 className="text-base font-semibold text-text">
            {profile.profile.display_name}
          </h3>
          <p className="text-xs text-text-muted">
            Upload a custom avatar. PNG, JPG or WebP up to 5 MB.
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setIsAvatarModalOpen(true)}
            className="mt-2"
          >
            <Camera className="mr-2 h-4 w-4" />
            Change Photo
          </Button>
        </div>
      </div>

      {statusMessage && (
        <div
          role="status"
          className={`flex items-start gap-2.5 rounded-lg p-3.5 text-xs ${
            statusMessage.type === "success"
              ? "border border-success/30 bg-success/10 text-success-accent"
              : "border border-danger/30 bg-danger/10 text-danger-accent"
          }`}
        >
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="h-4 w-4 shrink-0 text-success-accent mt-0.5" />
          ) : (
            <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Profile Form */}
      <form
        onSubmit={(e) => {
          void handleSubmit(onSubmit)(e);
        }}
        className="space-y-6"
      >
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
          {/* Email (Read-only) */}
          <div className="space-y-2 sm:col-span-2">
            <Label htmlFor="email" className="flex items-center gap-2">
              <Mail className="h-4 w-4 text-text-muted" />
              Email Address
            </Label>
            <div className="flex items-center justify-between rounded-lg border border-border-subtle bg-surface-card/40 px-3.5 py-2.5 text-sm text-text-muted">
              <span>{profile.email}</span>
              {profile.email_verified_at ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-success/10 px-2 py-0.5 text-xs font-medium text-success-accent border border-success/20">
                  <CheckCircle2 className="h-3 w-3" />
                  Verified
                </span>
              ) : (
                <span className="rounded-full bg-warning/10 px-2 py-0.5 text-xs font-medium text-warning-accent border border-warning/20">
                  Unverified
                </span>
              )}
            </div>
          </div>

          {/* Display Name */}
          <div className="space-y-2">
            <Label htmlFor="display_name" className="flex items-center gap-2">
              <User className="h-4 w-4 text-text-muted" />
              Display Name
            </Label>
            <Input
              id="display_name"
              {...register("display_name")}
              placeholder="Your name"
              aria-invalid={!!errors.display_name}
            />
            {errors.display_name && (
              <p className="text-xs text-danger-accent">
                {errors.display_name.message}
              </p>
            )}
          </div>

          {/* Country */}
          <div className="space-y-2">
            <Label htmlFor="country" className="flex items-center gap-2">
              <Globe className="h-4 w-4 text-text-muted" />
              Country (ISO 2-letter code)
            </Label>
            <Input
              id="country"
              {...register("country")}
              placeholder="e.g. VN, US, JP"
              maxLength={2}
              aria-invalid={!!errors.country}
            />
            {errors.country && (
              <p className="text-xs text-danger-accent">{errors.country.message}</p>
            )}
          </div>

          {/* Timezone */}
          <div className="space-y-2">
            <Label htmlFor="timezone">Timezone</Label>
            <Input
              id="timezone"
              {...register("timezone")}
              placeholder="e.g. Asia/Ho_Chi_Minh"
              aria-invalid={!!errors.timezone}
            />
            {errors.timezone && (
              <p className="text-xs text-danger-accent">{errors.timezone.message}</p>
            )}
          </div>

          {/* Date of Birth */}
          <div className="space-y-2">
            <Label htmlFor="date_of_birth">Date of Birth</Label>
            <Input
              id="date_of_birth"
              type="date"
              {...register("date_of_birth")}
              aria-invalid={!!errors.date_of_birth}
            />
            {errors.date_of_birth && (
              <p className="text-xs text-danger-accent">
                {errors.date_of_birth.message}
              </p>
            )}
          </div>
        </div>

        <div className="flex justify-end pt-4">
          <Button type="submit" disabled={isSaving || !isDirty}>
            {isSaving ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Saving...
              </>
            ) : (
              "Save Changes"
            )}
          </Button>
        </div>
      </form>

      <AvatarUploadModal
        isOpen={isAvatarModalOpen}
        onClose={() => setIsAvatarModalOpen(false)}
        onSuccess={handleAvatarUploaded}
      />
    </div>
  );
};
