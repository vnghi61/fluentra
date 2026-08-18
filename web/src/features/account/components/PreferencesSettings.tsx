import React, { useState } from "react";
import { useForm, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
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

interface PreferencesSettingsProps {
  initialPreferences: UserPreferences;
  onPreferencesUpdated?: (preferences: UserPreferences) => void;
}

export const PreferencesSettings: React.FC<PreferencesSettingsProps> = ({
  initialPreferences,
  onPreferencesUpdated,
}) => {
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
      });

      setPreferences(updated);
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
        text: "Preferences updated successfully.",
      });
      onPreferencesUpdated?.(updated);
    } catch (err: unknown) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error ? err.message : "Failed to update preferences.",
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      {statusMessage && (
        <div
          role="status"
          className={`flex items-start gap-2.5 rounded-lg p-3.5 text-xs ${
            statusMessage.type === "success"
              ? "border border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
              : "border border-rose-500/30 bg-rose-500/10 text-rose-300"
          }`}
        >
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-400 mt-0.5" />
          ) : (
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      <form
        onSubmit={(e) => {
          void handleSubmit(onSubmit)(e);
        }}
        className="space-y-8"
      >
        {/* Learning Goal & Locale */}
        <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-6 space-y-6">
          <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
            <Clock className="h-5 w-5 text-indigo-400" />
            Learning & Interface
          </h3>

          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            {/* Daily Goal */}
            <div className="space-y-2">
              <Label htmlFor="daily_goal_minutes">
                Daily Study Goal (minutes)
              </Label>
              <Input
                id="daily_goal_minutes"
                type="number"
                min={5}
                max={180}
                {...register("daily_goal_minutes", { valueAsNumber: true })}
                aria-invalid={!!errors.daily_goal_minutes}
              />
              {errors.daily_goal_minutes && (
                <p className="text-xs text-rose-400">
                  {errors.daily_goal_minutes.message}
                </p>
              )}
            </div>

            {/* Language / Locale */}
            <div className="space-y-2">
              <Label htmlFor="locale" className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-slate-400" />
                Language
              </Label>
              <select
                id="locale"
                {...register("locale")}
                className="w-full h-11 rounded-lg border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              >
                <option value="vi">Tiếng Việt (Vietnamese)</option>
                <option value="en">English</option>
              </select>
            </div>

            {/* Theme */}
            <div className="space-y-2 sm:col-span-2">
              <Label className="flex items-center gap-2">
                <Sun className="h-4 w-4 text-slate-400" />
                Theme Preference
              </Label>
              <div className="grid grid-cols-3 gap-3">
                {(["system", "dark", "light"] as const).map((t) => (
                  <label
                    key={t}
                    className={`flex items-center justify-center gap-2 rounded-lg border p-3 text-sm font-medium cursor-pointer transition-colors ${
                      currentTheme === t
                        ? "border-indigo-500 bg-indigo-500/10 text-indigo-300"
                        : "border-slate-800 bg-slate-900/40 text-slate-400 hover:bg-slate-800"
                    }`}
                  >
                    <input
                      type="radio"
                      value={t}
                      {...register("theme")}
                      className="hidden"
                    />
                    {t === "dark" ? (
                      <Moon className="h-4 w-4" />
                    ) : t === "light" ? (
                      <Sun className="h-4 w-4" />
                    ) : (
                      <Globe className="h-4 w-4" />
                    )}
                    <span className="capitalize">{t}</span>
                  </label>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Notifications & Quiet Hours */}
        <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-6 space-y-6">
          <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
            <Bell className="h-5 w-5 text-indigo-400" />
            Notifications & Quiet Hours
          </h3>

          <div className="space-y-4">
            <Label>Notification Channels</Label>
            <div className="flex flex-wrap gap-4">
              <Controller
                control={control}
                name="notification_channels"
                render={({ field }) => (
                  <>
                    {[
                      { id: "in_app", label: "In-App Notifications" },
                      { id: "email", label: "Email Summaries & Reminders" },
                      { id: "push", label: "Push Notifications" },
                    ].map((channel) => (
                      <label
                        key={channel.id}
                        className="flex items-center gap-2.5 cursor-pointer text-sm text-slate-300"
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

          <div className="border-t border-slate-800 pt-4 space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <Label
                  htmlFor="quiet_hours_enabled"
                  className="text-sm font-medium"
                >
                  Enable Quiet Hours
                </Label>
                <p className="text-xs text-slate-400">
                  Pause non-critical notifications during sleeping hours
                </p>
              </div>
              <Controller
                control={control}
                name="quiet_hours_enabled"
                render={({ field }) => (
                  <Checkbox
                    id="quiet_hours_enabled"
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                )}
              />
            </div>

            {quietHoursEnabled && (
              <div className="grid grid-cols-2 gap-4 pt-2">
                <div className="space-y-1.5">
                  <Label htmlFor="quiet_hours_start" className="text-xs">
                    Start Time
                  </Label>
                  <Input
                    id="quiet_hours_start"
                    type="time"
                    {...register("quiet_hours_start")}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="quiet_hours_end" className="text-xs">
                    End Time
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
        </div>

        {/* AI Processing Opt-Out */}
        <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-6 space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <Bot className="h-5 w-5 text-indigo-400" />
                AI Grading & Processing
              </h3>
              <p className="text-xs text-slate-400 leading-relaxed max-w-xl">
                Opt out of AI-assisted grading and pronunciation feedback.
                Deterministic exercises and progress tracking remain fully
                functional.
              </p>
            </div>
            <Controller
              control={control}
              name="ai_processing_opt_out"
              render={({ field }) => (
                <Checkbox
                  id="ai_processing_opt_out"
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  className="mt-1"
                />
              )}
            />
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
              "Save Preferences"
            )}
          </Button>
        </div>
      </form>
    </div>
  );
};
