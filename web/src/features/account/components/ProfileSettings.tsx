import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Camera,
  CheckCircle2,
  Clock,
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
import {
  SearchableSelect,
  type SearchableSelectOption,
} from "@/components/ui/searchable-select";
import {
  countryOptions,
  timezoneOptions,
  getTimezoneInfo,
} from "@/lib/locales";

interface ProfileSettingsProps {
  initialProfile: UserProfile;
  onProfileUpdated?: (profile: UserProfile) => void;
}

export const ProfileSettings: React.FC<ProfileSettingsProps> = ({
  initialProfile,
  onProfileUpdated,
}) => {
  const { t, i18n } = useTranslation();
  const [profile, setProfile] = useState<UserProfile>(initialProfile);

  // Memoised: the country list is ~250 Intl lookups plus a locale-aware sort,
  // and this form re-renders on every keystroke.
  //
  // A searchable list, like the time zone, not a native select: ~250 countries
  // is too many to scroll, and a native select cannot be searched by the
  // English name or the code. The first entry clears the field.
  const countries = useMemo<SearchableSelectOption[]>(
    () => [
      { value: "", label: t("account.countryUnset") },
      ...countryOptions(i18n.language).map((c) => ({
        value: c.code,
        label: c.name,
        searchTerms: c.searchTerms,
      })),
    ],
    [i18n.language, t],
  );
  const timezones = useMemo<SearchableSelectOption[]>(
    () =>
      timezoneOptions(profile.profile.timezone)
        .map(getTimezoneInfo)
        .map((tz) => ({
          value: tz.id,
          label: tz.label,
          searchTerms: tz.searchTerms,
        })),
    [profile.profile.timezone],
  );

  const [isAvatarModalOpen, setIsAvatarModalOpen] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
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

  const selectedCountry = watch("country") ?? "";
  const selectedTz = watch("timezone");

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
        text: t(
          "account.profileUpdatedSuccessfully",
          "Profile updated successfully.",
        ),
      });
      onProfileUpdated?.(updated);
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t("account.failedToUpdateProfile", "Failed to update profile."),
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
    setStatusMessage({
      type: "success",
      text: t(
        "account.avatarUpdatedSuccessfully",
        "Avatar updated successfully.",
      ),
    });
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
            title={t("account.changeAvatar", "Change avatar")}
          >
            <Camera className="h-6 w-6" />
          </button>
        </div>

        <div className="space-y-2 text-center sm:text-left">
          <h3 className="text-base font-semibold text-text">
            {profile.profile.display_name}
          </h3>
          <p className="text-xs text-text-muted">
            {t(
              "account.uploadCustomAvatar",
              "Upload a custom avatar. PNG, JPG or WebP up to 5 MB.",
            )}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setIsAvatarModalOpen(true)}
            className="mt-2"
          >
            <Camera className="mr-2 h-4 w-4" />
            {t("account.changePhoto", "Change Photo")}
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
              {t("account.emailAddress", "Email Address")}
            </Label>
            <div className="flex items-center justify-between rounded-lg border border-border-subtle bg-surface-card/40 px-3.5 py-2.5 text-sm text-text-muted">
              <span>{profile.email}</span>
              {profile.email_verified_at ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-success/10 px-2 py-0.5 text-xs font-medium text-success-accent border border-success/20">
                  <CheckCircle2 className="h-3 w-3" />
                  {t("account.verified", "Verified")}
                </span>
              ) : (
                <span className="rounded-full bg-warning/10 px-2 py-0.5 text-xs font-medium text-warning-accent border border-warning/20">
                  {t("account.unverified", "Unverified")}
                </span>
              )}
            </div>
          </div>

          {/* Display Name */}
          <div className="space-y-2">
            <Label htmlFor="display_name" className="flex items-center gap-2">
              <User className="h-4 w-4 text-text-muted" />
              {t("account.displayName", "Display Name")}
            </Label>
            <Input
              id="display_name"
              {...register("display_name")}
              placeholder={t("account.yourName", "Your name")}
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
              {t("account.country")}
            </Label>
            {/* A list, not a text box. The server stores an ISO alpha-2 code and
                validated only the length, so "US" and "XX" were equally
                acceptable and "Viet Nam" was rejected for being too long. */}
            <input type="hidden" {...register("country")} />
            <SearchableSelect
              id="country"
              value={selectedCountry}
              options={countries}
              onChange={(code) =>
                setValue("country", code, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
              placeholder={t("account.countryUnset", "Not set")}
              searchPlaceholder={t(
                "account.searchCountry",
                "Search country or country code...",
              )}
              emptyText={t("account.noCountriesFound", "No countries found")}
              invalid={!!errors.country}
            />
            {errors.country && (
              <p className="text-xs text-danger-accent">
                {errors.country.message}
              </p>
            )}
          </div>

          {/* Timezone */}
          <div className="space-y-2">
            <Label htmlFor="timezone" className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-text-muted" />
              {t("account.timezone")}
            </Label>
            <input type="hidden" {...register("timezone")} />
            <SearchableSelect
              id="timezone"
              value={selectedTz}
              options={timezones}
              onChange={(zone) =>
                setValue("timezone", zone, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
              // A zone the browser cannot enumerate still names itself.
              placeholder={getTimezoneInfo(selectedTz || "UTC").label}
              searchPlaceholder={t(
                "account.searchTimezone",
                "Search timezone, city, country, or UTC offset...",
              )}
              emptyText={t("account.noTimezonesFound", "No timezones found")}
              invalid={!!errors.timezone}
            />
            {errors.timezone && (
              <p className="text-xs text-danger-accent">
                {errors.timezone.message}
              </p>
            )}
          </div>

          {/* Date of Birth */}
          <div className="space-y-2">
            <Label htmlFor="date_of_birth">
              {t("account.dateOfBirth", "Date of Birth")}
            </Label>
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
                {t("account.saving", "Saving...")}
              </>
            ) : (
              t("account.saveChanges", "Save Changes")
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
