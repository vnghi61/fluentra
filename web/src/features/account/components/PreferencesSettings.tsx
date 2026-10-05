import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { isLocale, setLocale } from "@/i18n";
import { applyThemeChoice } from "@/lib/theme";
import { usePreferencesStore } from "@/stores/preferencesStore";
import {
  Bell,
  Bot,
  CheckCircle2,
  Clock,
  Globe,
  Loader2,
  Moon,
  Sun,
  AlertCircle,
} from "lucide-react";
import { accountApi, type UserPreferences } from "../api/accountApi";
import {
  preferencesFormSchema,
  type PreferencesFormValues,
} from "../model/schemas";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  settingsAlert,
  settingsCard,
  settingsCardTitle,
} from "./settingsStyles";
import { DAILY_GOAL_MINUTES } from "@/lib/locales";

interface PreferencesSettingsProps {
  initialPreferences: UserPreferences;
  onPreferencesUpdated?: (preferences: UserPreferences) => void;
}

export const PreferencesSettings: React.FC<PreferencesSettingsProps> = ({
  initialPreferences,
  onPreferencesUpdated,
}) => {
  const { t } = useTranslation();
  const [preferences, setPreferences] =
    useState<UserPreferences>(initialPreferences);
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isDirty },
    reset,
  } = useForm<PreferencesFormValues>({
    resolver: (zodResolver as (schema: unknown) => never)(
      preferencesFormSchema,
    ),
    defaultValues: {
      locale: preferences.locale || "vi",
      theme: preferences.theme || "dark",
      daily_goal_minutes: preferences.daily_goal_minutes || 30,
      notification_channels: preferences.notification_channels || [
        "in_app",
        "email",
      ],
      quiet_hours_enabled: !!preferences.quiet_hours,
      quiet_hours_start: preferences.quiet_hours?.start || "22:00",
      quiet_hours_end: preferences.quiet_hours?.end || "07:00",
      ai_processing_opt_out: preferences.ai_processing_opt_out || false,
    },
  });

  const quietHoursEnabled = useWatch({ control, name: "quiet_hours_enabled" });
  const currentTheme = useWatch({ control, name: "theme" });
  const aiOptOut = useWatch({ control, name: "ai_processing_opt_out" });

  const onSubmit = async (values: PreferencesFormValues) => {
    setIsSaving(true);
    setStatusMessage(null);

    try {
      const updated = await accountApi.replacePreferences({
        locale: values.locale,
        theme: values.theme,
        daily_goal_minutes: values.daily_goal_minutes,
        notification_channels: values.notification_channels,
        quiet_hours: values.quiet_hours_enabled
          ? {
              start: values.quiet_hours_start,
              end: values.quiet_hours_end,
            }
          : null,
        ai_processing_opt_out: values.ai_processing_opt_out,
        // Not on this form, and a replacement stores an omitted level as null:
        // send back the one the daily practice card saved.
        practice_level:
          usePreferencesStore.getState().preferences?.practice_level ??
          preferences.practice_level ??
          null,
      });

      setPreferences(updated);
      // Apply what was just saved. Without these two lines the form wrote the
      // row and changed nothing on screen, which is how `applyTheme` and
      // `setLocale` both ended up written and never called.
      if (updated.theme) applyThemeChoice(updated.theme);
      if (isLocale(updated.locale)) setLocale(updated.locale);
      usePreferencesStore.getState().set(updated);
      reset({
        locale: updated.locale,
        theme: updated.theme,
        daily_goal_minutes: updated.daily_goal_minutes,
        notification_channels: updated.notification_channels,
        quiet_hours_enabled: !!updated.quiet_hours,
        quiet_hours_start: updated.quiet_hours?.start || "22:00",
        quiet_hours_end: updated.quiet_hours?.end || "07:00",
        ai_processing_opt_out: updated.ai_processing_opt_out,
      });
      setStatusMessage({
        type: "success",
        text: t(
          "account.preferencesUpdatedSuccessfully",
          "Preferences updated successfully.",
        ),
      });
      onPreferencesUpdated?.(updated);
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t(
                "account.failedToUpdatePreferences",
                "Failed to update preferences.",
              ),
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      {statusMessage && (
        <div role="status" className={settingsAlert(statusMessage.type)}>
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          ) : (
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      <form
        onSubmit={(e) => {
          void handleSubmit(onSubmit)(e);
        }}
        className="space-y-4"
      >
        {/* Learning Goal & Locale */}
        <section className={`${settingsCard} space-y-5`}>
          <h3 className={settingsCardTitle}>
            <Clock className="h-5 w-5 text-primary-accent" />
            {t("account.learningInterface", "Learning & Interface")}
          </h3>

          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            {/* Daily Goal */}
            <div className="space-y-2">
              <Label htmlFor="daily_goal_minutes">
                {t("account.dailyGoal", "Daily goal")}
              </Label>
              {/* A list, not a number box. The box accepted 7 and 113, whose
                  value over 10 and 120 is nil, and 1130 by a slipped keystroke,
                  which is a goal that can never be met and a streak that can
                  never be kept. */}
              <select
                id="daily_goal_minutes"
                {...register("daily_goal_minutes", { valueAsNumber: true })}
                aria-invalid={!!errors.daily_goal_minutes}
                className="flex h-11 min-h-[44px] w-full rounded-[10px] border border-border-subtle bg-surface-card px-3 text-base text-text focus:outline-none focus:ring-2 focus:ring-primary md:text-sm"
              >
                {DAILY_GOAL_MINUTES.map((minutes) => (
                  <option key={minutes} value={minutes}>
                    {t("account.goalMinutesShort", {
                      count: minutes,
                      defaultValue: `${minutes} min`,
                    })}
                  </option>
                ))}
              </select>
              {errors.daily_goal_minutes?.message && (
                <p className="text-xs text-danger-accent">
                  {t(errors.daily_goal_minutes.message)}
                </p>
              )}
            </div>

            {/* Language / Locale */}
            <div className="space-y-2">
              <Label htmlFor="locale" className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-text-muted" />
                {t("account.language", "Language")}
              </Label>
              <select
                id="locale"
                {...register("locale")}
                className="h-11 min-h-[44px] w-full rounded-[10px] border border-border-subtle bg-surface-card px-3 py-2 text-base text-text focus:outline-none focus:ring-2 focus:ring-primary md:text-sm"
              >
                <option value="vi">Tiếng Việt (Vietnamese)</option>
                <option value="en">{t("account.english", "English")}</option>
              </select>
            </div>

            {/* Theme */}
            <div className="space-y-2 sm:col-span-2">
              <Label className="flex items-center gap-2">
                <Sun className="h-4 w-4 text-text-muted" />
                {t("account.themePreference", "Theme Preference")}
              </Label>
              <div
                role="radiogroup"
                aria-label={t("account.themePreference", "Theme Preference")}
                className="grid grid-cols-3 gap-2"
              >
                {(["system", "dark", "light"] as const).map((mode) => (
                  <label
                    key={mode}
                    className={`flex min-h-[44px] min-w-0 cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-[10px] border px-2 text-sm font-medium transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary ${
                      currentTheme === mode
                        ? "border-primary bg-primary/10 text-primary-accent"
                        : "border-border-subtle bg-surface-card text-text-muted hover:bg-surface-muted"
                    }`}
                  >
                    <input
                      type="radio"
                      value={mode}
                      {...register("theme")}
                      className="sr-only"
                    />
                    {mode === "dark" ? (
                      <Moon className="h-4 w-4" />
                    ) : mode === "light" ? (
                      <Sun className="h-4 w-4" />
                    ) : (
                      <Globe className="h-4 w-4" />
                    )}
                    <span>
                      {mode === "system"
                        ? t("account.themeSystemShort", "System")
                        : mode === "dark"
                          ? t("theme.dark", "Dark")
                          : t("theme.light", "Light")}
                    </span>
                  </label>
                ))}
              </div>
            </div>
          </div>
        </section>

        {/* Notifications & Quiet Hours */}
        <section className={`${settingsCard} space-y-5`}>
          <h3 className={settingsCardTitle}>
            <Bell className="h-5 w-5 text-primary-accent" />
            {t(
              "account.notificationsQuietHours",
              "Notifications & Quiet Hours",
            )}
          </h3>

          <div className="space-y-4">
            <Label>
              {t("account.notificationChannels", "Notification Channels")}
            </Label>
            <div className="grid gap-1 sm:grid-cols-2">
              <Controller
                control={control}
                name="notification_channels"
                render={({ field }) => (
                  <>
                    {[
                      {
                        id: "in_app",
                        label: t(
                          "account.inAppNotifications",
                          "In-App Notifications",
                        ),
                      },
                      {
                        id: "email",
                        label: t(
                          "account.emailSummariesReminders",
                          "Email Summaries & Reminders",
                        ),
                      },
                      {
                        id: "push",
                        label: t(
                          "account.pushNotifications",
                          "Push Notifications",
                        ),
                      },
                    ].map((channel) => (
                      <label
                        key={channel.id}
                        className="flex min-h-[44px] cursor-pointer items-center gap-2.5 text-sm text-text"
                      >
                        <Checkbox
                          checked={field.value.includes(
                            channel.id as "in_app" | "email" | "push",
                          )}
                          onCheckedChange={(checked) => {
                            if (checked) {
                              field.onChange([...field.value, channel.id]);
                            } else {
                              field.onChange(
                                field.value.filter((v) => v !== channel.id),
                              );
                            }
                          }}
                        />
                        <span>{channel.label}</span>
                      </label>
                    ))}
                  </>
                )}
              />
            </div>
          </div>

          <div className="space-y-3 border-t border-border-subtle pt-4">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <Label
                  htmlFor="quiet_hours_enabled"
                  className="text-sm font-medium"
                >
                  {t("account.enableQuietHours", "Enable Quiet Hours")}
                </Label>
                <p className="text-xs text-text-muted">
                  {t(
                    "account.pauseNonCriticalNotificationsDuringSleepingHours",
                    "Pause non-critical notifications during sleeping hours",
                  )}
                </p>
              </div>
              <Controller
                control={control}
                name="quiet_hours_enabled"
                render={({ field }) => (
                  <Switch
                    id="quiet_hours_enabled"
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                )}
              />
            </div>

            {quietHoursEnabled && (
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="quiet_hours_start" className="text-xs">
                    {t("account.startTime", "Start Time")}
                  </Label>
                  <Input
                    id="quiet_hours_start"
                    type="time"
                    {...register("quiet_hours_start")}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="quiet_hours_end" className="text-xs">
                    {t("account.endTime", "End Time")}
                  </Label>
                  <Input
                    id="quiet_hours_end"
                    type="time"
                    {...register("quiet_hours_end")}
                  />
                </div>
              </div>
            )}
          </div>
        </section>

        {/* AI grading: the stored field is an opt-out, the switch shows "on" */}
        <section className={settingsCard}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <h3 className={settingsCardTitle}>
                <Bot className="h-5 w-5 text-primary-accent" aria-hidden />
                <label htmlFor="ai_processing_enabled">
                  {t("account.aiGradingProcessing", "AI Grading & Processing")}
                </label>
              </h3>
              <p className="max-w-xl text-sm leading-relaxed text-text-muted">
                {aiOptOut
                  ? t(
                      "account.aiGradingOffDesc",
                      "Off: AI-assisted grading and pronunciation feedback are not used. Deterministic exercises and progress tracking still work.",
                    )
                  : t(
                      "account.aiGradingOnDesc",
                      "On: AI grades open answers and gives pronunciation feedback. Turn it off to keep only deterministic exercises.",
                    )}
              </p>
            </div>
            <Controller
              control={control}
              name="ai_processing_opt_out"
              render={({ field }) => (
                <Switch
                  id="ai_processing_enabled"
                  checked={!field.value}
                  onCheckedChange={(on) => field.onChange(!on)}
                />
              )}
            />
          </div>
        </section>

        <div className="flex justify-end">
          <Button
            type="submit"
            disabled={isSaving || !isDirty}
            className="min-h-[44px] w-full rounded-xl sm:w-auto"
          >
            {isSaving ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t("account.saving", "Saving...")}
              </>
            ) : (
              t("account.savePreferences", "Save Preferences")
            )}
          </Button>
        </div>
      </form>
    </div>
  );
};
