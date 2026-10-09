import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { Camera, CheckCircle2, Mail } from "lucide-react";

import type { UserProfile } from "../api/accountApi";
import { AvatarUploadModal } from "./AvatarUploadModal";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDate, settingsAlert, settingsCard } from "./settingsStyles";

interface ProfileHeroCardProps {
  profile: UserProfile;
  onAvatarUpdated: (avatarUrl: string | null) => void;
}

/**
 * Who is signed in: the avatar with its change action, the name, the email.
 * There is no remove-avatar operation in the API, so there is no Remove
 * button; a new upload replaces the old image (the server deletes it).
 */
export const ProfileHeroCard: React.FC<ProfileHeroCardProps> = ({
  profile,
  onAvatarUpdated,
}) => {
  const { t, i18n } = useTranslation();
  const [isAvatarModalOpen, setIsAvatarModalOpen] = useState(false);
  const [updated, setUpdated] = useState(false);
  const name = profile.profile.display_name;
  const joined = formatDate(profile.created_at, i18n.language);

  return (
    <section
      aria-labelledby="profile-hero-name"
      className={`${settingsCard} @container relative overflow-hidden`}
    >
      {/* A soft wash behind the avatar; tokens only, so both themes hold. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 h-24 bg-gradient-to-b from-primary/10 to-transparent"
      />
      <div className="relative flex flex-col items-center gap-4 text-center @md:flex-row @md:items-center @md:text-left">
        <div className="relative shrink-0">
          <div className="flex h-24 w-24 items-center justify-center overflow-hidden rounded-full border-4 border-surface-card bg-primary/10 text-3xl font-bold text-primary-accent shadow-[var(--shadow-card)] ring-2 ring-primary/25 sm:h-28 sm:w-28">
            {profile.profile.avatar_url ? (
              <img
                src={profile.profile.avatar_url}
                alt={name}
                className="h-full w-full object-cover"
              />
            ) : (
              <span aria-hidden="true">{name.charAt(0).toUpperCase()}</span>
            )}
          </div>
          <button
            type="button"
            onClick={() => setIsAvatarModalOpen(true)}
            aria-label={t("account.changeAvatar", "Change avatar")}
            className="absolute -bottom-1 -right-1 flex h-11 w-11 cursor-pointer items-center justify-center rounded-full border-4 border-surface-card bg-primary text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 motion-reduce:transition-none"
          >
            <Camera className="h-5 w-5" aria-hidden />
          </button>
        </div>

        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center justify-center gap-2 @md:justify-start">
            <h2
              id="profile-hero-name"
              className="min-w-0 break-words text-xl font-semibold leading-tight text-text md:text-2xl"
            >
              {name}
            </h2>
            <Badge variant="primary">
              {t("settings.profile.personalAccount", "Personal account")}
            </Badge>
          </div>
          <p className="flex min-w-0 items-center justify-center gap-1.5 text-sm text-text-muted @md:justify-start">
            <Mail className="h-4 w-4 shrink-0" aria-hidden />
            <span className="min-w-0 truncate">{profile.email}</span>
          </p>
          <p className="text-sm text-text-muted">
            {t("settings.profile.memberSince", {
              date: joined,
              defaultValue: `Member since ${joined}`,
            })}
          </p>
          <p className="text-sm text-text-muted">
            {t(
              "account.uploadCustomAvatar",
              "Upload a custom avatar. PNG, JPG or WebP up to 5 MB.",
            )}
          </p>
          <div className="flex flex-wrap justify-center gap-2 pt-1 @md:justify-start">
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsAvatarModalOpen(true)}
              className="w-full @xs:w-auto"
            >
              <Camera className="h-4 w-4" aria-hidden />
              {t("account.changePhoto", "Change Photo")}
            </Button>
          </div>
        </div>
      </div>

      {updated && (
        <div
          role="status"
          className={`${settingsAlert("success")} relative mt-4`}
        >
          <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden />
          <span>
            {t(
              "account.avatarUpdatedSuccessfully",
              "Avatar updated successfully.",
            )}
          </span>
        </div>
      )}

      <AvatarUploadModal
        isOpen={isAvatarModalOpen}
        onClose={() => setIsAvatarModalOpen(false)}
        onSuccess={(url) => {
          setUpdated(true);
          onAvatarUpdated(url ?? null);
        }}
      />
    </section>
  );
};
