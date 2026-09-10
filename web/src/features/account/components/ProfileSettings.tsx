import React, { useMemo, useRef, useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Camera,
  Check,
  CheckCircle2,
  ChevronDown,
  Clock,
  Globe,
  Loader2,
  Mail,
  Search,
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
  countryOptions,
  timezoneOptions,
  getTimezoneInfo,
  type TimezoneInfo,
} from "@/lib/locales";

interface ProfileSettingsProps {
  initialProfile: UserProfile;
  onProfileUpdated?: (profile: UserProfile) => void;
}

/** One class string for both selects, so they cannot drift apart. */
const SELECT_CLASS =
  "flex h-11 min-h-[44px] w-full rounded-lg border border-border-subtle " +
  "bg-surface-card px-3 text-base text-text focus:outline-none " +
  "focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50";

export const ProfileSettings: React.FC<ProfileSettingsProps> = ({
  initialProfile,
  onProfileUpdated,
}) => {
  const { t, i18n } = useTranslation();
  const [profile, setProfile] = useState<UserProfile>(initialProfile);

  // Memoised: the country list is ~200 Intl lookups plus a locale-aware sort,
  // and this form re-renders on every keystroke.
  const countries = useMemo(
    () => countryOptions(i18n.language),
    [i18n.language],
  );
  const timezones = useMemo(
    () => timezoneOptions(profile.profile.timezone).map(getTimezoneInfo),
    [profile.profile.timezone],
  );
  const [isTzOpen, setIsTzOpen] = useState(false);
  const [tzSearch, setTzSearch] = useState("");
  const tzComboboxRef = useRef<HTMLDivElement>(null);

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

  const selectedTz = watch("timezone");
  const selectedTzInfo = useMemo<TimezoneInfo>(
    () =>
      timezones.find((tz) => tz.id === selectedTz) ??
      getTimezoneInfo(selectedTz || "UTC"),
    [timezones, selectedTz],
  );

  const filteredTimezones = useMemo(() => {
    const q = tzSearch.trim().toLowerCase();
    if (!q) return timezones;
    return timezones.filter((tz) => tz.searchTerms.includes(q));
  }, [timezones, tzSearch]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        tzComboboxRef.current &&
        !tzComboboxRef.current.contains(event.target as Node)
      ) {
        setIsTzOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsTzOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

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
            <select
              id="country"
              {...register("country")}
              aria-invalid={!!errors.country}
              className={SELECT_CLASS}
            >
              <option value="">{t("account.countryUnset")}</option>
              {countries.map((c) => (
                <option key={c.code} value={c.code}>
                  {c.name}
                </option>
              ))}
            </select>
            {errors.country && (
              <p className="text-xs text-danger-accent">
                {errors.country.message}
              </p>
            )}
          </div>

          {/* Timezone Combobox */}
          <div className="space-y-2" ref={tzComboboxRef}>
            <Label htmlFor="timezone" className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-text-muted" />
              {t("account.timezone")}
            </Label>
            <input type="hidden" {...register("timezone")} />
            <div className="relative">
              <button
                id="timezone"
                type="button"
                role="combobox"
                aria-haspopup="listbox"
                aria-expanded={isTzOpen}
                aria-controls="timezone-listbox"
                onClick={() => {
                  setIsTzOpen((prev) => !prev);
                  setTzSearch("");
                }}
                className={
                  "flex h-11 min-h-[44px] w-full items-center justify-between rounded-lg border border-border-subtle " +
                  "bg-surface-card px-3 text-base text-text focus:outline-none " +
                  "focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50"
                }
              >
                <span className="truncate">{selectedTzInfo.label}</span>
                <ChevronDown className="h-4 w-4 text-text-muted shrink-0 ml-2" />
              </button>

              {isTzOpen && (
                <div className="absolute z-50 mt-1 max-h-64 w-full overflow-hidden rounded-lg border border-border-subtle bg-surface-card shadow-xl flex flex-col">
                  <div className="p-2 border-b border-border-subtle bg-surface-card">
                    <div className="relative">
                      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-text-muted" />
                      <input
                        type="text"
                        value={tzSearch}
                        onChange={(e) => setTzSearch(e.target.value)}
                        placeholder={t(
                          "account.searchTimezone",
                          "Search timezone, city, country, or UTC offset...",
                        )}
                        aria-label={t(
                          "account.searchTimezone",
                          "Search timezone",
                        )}
                        className="w-full pl-9 pr-3 py-2 text-base rounded-md border border-border-subtle bg-surface-muted text-text focus:outline-none focus:ring-2 focus:ring-primary min-h-[44px]"
                        autoFocus
                      />
                    </div>
                  </div>
                  <ul
                    id="timezone-listbox"
                    role="listbox"
                    className="overflow-y-auto max-h-48 p-1 divide-y divide-border-subtle/20"
                  >
                    {filteredTimezones.length === 0 ? (
                      <li className="p-3 text-sm text-text-muted text-center">
                        {t("account.noTimezonesFound", "No timezones found")}
                      </li>
                    ) : (
                      filteredTimezones.map((tz) => (
                        <li
                          key={tz.id}
                          role="option"
                          aria-selected={tz.id === selectedTz}
                          onClick={() => {
                            setValue("timezone", tz.id, {
                              shouldDirty: true,
                              shouldValidate: true,
                            });
                            setIsTzOpen(false);
                            setTzSearch("");
                          }}
                          className={
                            "flex items-center justify-between px-3 py-2.5 min-h-[44px] text-base rounded-md cursor-pointer transition-colors hover:bg-surface-muted " +
                            (tz.id === selectedTz
                              ? "bg-primary/10 text-primary-accent font-medium"
                              : "text-text")
                          }
                        >
                          <span className="truncate">{tz.label}</span>
                          {tz.id === selectedTz && (
                            <Check className="h-4 w-4 text-primary-accent shrink-0 ml-2" />
                          )}
                        </li>
                      ))
                    )}
                  </ul>
                </div>
              )}
            </div>
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
