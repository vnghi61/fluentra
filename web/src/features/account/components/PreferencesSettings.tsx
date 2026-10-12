import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { currentLocale, isLocale, setLocale, switchLocale } from "@/i18n";
import { applyThemeChoice, currentTheme, systemTheme } from "@/lib/theme";
import {
  revealOriginOf,
  withPageTransition,
  type RevealOrigin,
} from "@/lib/view-transition";
import { usePreferencesStore } from "@/stores/preferencesStore";
import {
  AlertCircle,
  CheckCircle2,
  Globe,
  Monitor,
  Moon,
  Palette,
  Sun,
  Target,
} from "lucide-react";
import {
  accountApi,
  replacementFor,
  type ReplacePreferencesRequest,
  type UserPreferences,
} from "../api/accountApi";
import {
  appearanceSchema,
  learningPreferencesSchema,
  type AppearanceValues,
  type LearningPreferencesValues,
} from "../model/schemas";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { SettingsCardHeader } from "./SettingsCard";
import { settingsAlert, settingsCard } from "./settingsStyles";
import { DAILY_GOAL_MINUTES } from "@/lib/locales";
import { NativeSelect } from "@/components/ui/native-select";

interface PreferencesSettingsProps {
  initialPreferences: UserPreferences;
  onPreferencesUpdated?: (preferences: UserPreferences) => void;
  /** Anchor for links to the former "Learning preferences" section. */
  id?: string;
}

type StatusMessage = { type: "success" | "error"; text: string } | null;

/** Sends `changes` on top of the freshest stored record. */
type SavePreferences = (
  changes: Partial<ReplacePreferencesRequest>,
) => Promise<UserPreferences>;

const PRACTICE_LEVELS = ["A2", "B1", "B2"] as const;

const StatusAlert: React.FC<{ message: StatusMessage }> = ({ message }) =>
  message ? (
    <div role="status" className={settingsAlert(message.type)}>
      {message.type === "success" ? (
        <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden />
      ) : (
        <AlertCircle className="h-5 w-5 shrink-0" aria-hidden />
      )}
      <span>{message.text}</span>
    </div>
  ) : null;

/**
 * Two cards, two forms: "Learning preferences" (daily goal, practice level)
 * and "Appearance" (display mode, language). Each sends only its own fields;
 * the rest of the record goes back as stored (`replacementFor`).
 */
export const PreferencesSettings: React.FC<PreferencesSettingsProps> = ({
  initialPreferences,
  onPreferencesUpdated,
  id,
}) => {
  const [preferences, setPreferences] =
    useState<UserPreferences>(initialPreferences);

  const save: SavePreferences = async (changes) => {
    // PUT replaces the whole record. The base is the freshest copy: the
    // header theme/language switchers and the daily practice card write to
    // the store after this page loaded, and their choices must survive a
    // save made here. Notification channels, quiet hours and the AI opt-out
    // have no control anywhere and go back exactly as stored.
    const latest = usePreferencesStore.getState().preferences ?? preferences;
    const updated = await accountApi.replacePreferences(
      replacementFor(latest, changes),
    );
    setPreferences(updated);
    usePreferencesStore.getState().set(updated);
    onPreferencesUpdated?.(updated);
    return updated;
  };

  return (
    <>
      <LearningPreferencesCard id={id} preferences={preferences} save={save} />
      <AppearanceCard preferences={preferences} save={save} />
    </>
  );
};

const LearningPreferencesCard: React.FC<{
  id: string | undefined;
  preferences: UserPreferences;
  save: SavePreferences;
}> = ({ id, preferences, save }) => {
  const { t } = useTranslation();
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<StatusMessage>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isDirty },
    reset,
  } = useForm<LearningPreferencesValues>({
    resolver: (zodResolver as (schema: unknown) => never)(
      learningPreferencesSchema,
    ),
    defaultValues: {
      daily_goal_minutes: preferences.daily_goal_minutes || 30,
      practice_level: preferences.practice_level ?? "",
    },
  });

  const onSubmit = async (values: LearningPreferencesValues) => {
    setIsSaving(true);
    setStatusMessage(null);
    try {
      const updated = await save({
        daily_goal_minutes: values.daily_goal_minutes,
        practice_level:
          values.practice_level === "" ? null : values.practice_level,
      });
      reset({
        daily_goal_minutes: updated.daily_goal_minutes,
        practice_level: updated.practice_level ?? "",
      });
      setStatusMessage({
        type: "success",
        text: t(
          "account.preferencesUpdatedSuccessfully",
          "Preferences updated successfully.",
        ),
      });
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
    <section
      id={id}
      className={`${settingsCard} flex scroll-mt-4 flex-col gap-5`}
    >
      <SettingsCardHeader
        icon={Target}
        title={t("settings.learningPrefs.title", "Learning preferences")}
        description={t(
          "settings.learningPrefs.desc",
          "Your daily goal and the level your daily practice is drawn from.",
        )}
      />
      <form
        onSubmit={(e) => {
          void handleSubmit(onSubmit)(e);
        }}
        className="flex flex-1 flex-col gap-5"
      >
        <div className="space-y-2">
          <Label htmlFor="daily_goal_minutes">
            {t("account.dailyGoal", "Daily goal")}
          </Label>
          {/* A list, not a number box. The box accepted 7 and 113, whose
              value over 10 and 120 is nil, and 1130 by a slipped keystroke,
              which is a goal that can never be met and a streak that can
              never be kept. */}
          <NativeSelect
            id="daily_goal_minutes"
            {...register("daily_goal_minutes", { valueAsNumber: true })}
            aria-invalid={!!errors.daily_goal_minutes}
          >
            {DAILY_GOAL_MINUTES.map((minutes) => (
              <option key={minutes} value={minutes}>
                {t("account.goalMinutesShort", {
                  count: minutes,
                  defaultValue: `${minutes} min`,
                })}
              </option>
            ))}
          </NativeSelect>
          {errors.daily_goal_minutes?.message && (
            <p className="text-sm text-danger-accent">
              {t(errors.daily_goal_minutes.message)}
            </p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="practice_level">
            {t("settings.learningPrefs.practiceLevel", "Daily practice level")}
          </Label>
          <NativeSelect id="practice_level" {...register("practice_level")}>
            <option value="">
              {t("settings.learningPrefs.levelUnset", "Not chosen yet")}
            </option>
            {PRACTICE_LEVELS.map((level) => (
              <option key={level} value={level}>
                {level}
              </option>
            ))}
          </NativeSelect>
          <p className="text-sm text-text-muted">
            {t(
              "settings.learningPrefs.practiceLevelHint",
              "Daily practice sets are drawn at A2, B1 or B2.",
            )}
          </p>
        </div>

        <div className="mt-auto flex justify-end border-t border-border-subtle pt-4">
          <Button
            type="submit"
            disabled={!isDirty}
            isLoading={isSaving}
            className="w-full sm:w-auto"
          >
            {isSaving
              ? t("account.saving", "Saving...")
              : t("account.savePreferences", "Save Preferences")}
          </Button>
        </div>
      </form>
      <StatusAlert message={statusMessage} />
    </section>
  );
};

const THEME_MODES = [
  { mode: "system", Icon: Monitor },
  { mode: "dark", Icon: Moon },
  { mode: "light", Icon: Sun },
] as const;

const AppearanceCard: React.FC<{
  preferences: UserPreferences;
  save: SavePreferences;
}> = ({ preferences, save }) => {
  const { t } = useTranslation();
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<StatusMessage>(null);

  const {
    register,
    handleSubmit,
    control,
    formState: { isDirty },
    reset,
  } = useForm<AppearanceValues>({
    resolver: (zodResolver as (schema: unknown) => never)(appearanceSchema),
    defaultValues: {
      locale: preferences.locale || "vi",
      theme: preferences.theme || "dark",
    },
  });

  const selectedTheme = useWatch({ control, name: "theme" });

  const onSubmit = async (
    values: AppearanceValues,
    event?: React.BaseSyntheticEvent,
  ) => {
    // The theme lands after the round trip, from the button that saved it.
    const submitter = (event?.nativeEvent as SubmitEvent | undefined)
      ?.submitter;
    const origin = revealOriginOf(submitter);
    setIsSaving(true);
    setStatusMessage(null);

    try {
      const updated = await save({
        locale: values.locale,
        theme: values.theme,
      });
      // Apply what was just saved. Without this the form wrote the row and
      // changed nothing on screen, which is how `applyTheme` and `setLocale`
      // both ended up written and never called.
      applySaved(updated, origin);
      reset({ locale: updated.locale, theme: updated.theme });
      setStatusMessage({
        type: "success",
        text: t(
          "account.preferencesUpdatedSuccessfully",
          "Preferences updated successfully.",
        ),
      });
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
    <section className={`${settingsCard} @container flex flex-col gap-5`}>
      <SettingsCardHeader
        icon={Palette}
        title={t("settings.appearance.title", "Appearance")}
        description={t(
          "settings.appearance.desc",
          "Choose how Fluentra looks and which language it uses.",
        )}
      />
      <form
        onSubmit={(e) => {
          void handleSubmit(onSubmit)(e);
        }}
        className="flex flex-1 flex-col gap-5"
      >
        <fieldset className="space-y-2">
          <legend className="mb-2 text-sm font-medium text-text">
            {t("settings.appearance.mode", "Display mode")}
          </legend>
          <div
            role="radiogroup"
            aria-label={t("account.themePreference", "Theme Preference")}
            className="grid grid-cols-3 gap-1 rounded-xl border border-border-subtle bg-surface-muted/60 p-1"
          >
            {THEME_MODES.map(({ mode, Icon }) => (
              <label
                key={mode}
                className={cn(
                  "flex min-h-[44px] min-w-0 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg px-1 text-sm font-medium transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary motion-reduce:transition-none @xs:flex-row @xs:gap-1.5",
                  selectedTheme === mode
                    ? "bg-surface-card text-primary-accent shadow-sm"
                    : "text-text-muted hover:text-text",
                )}
              >
                <input
                  type="radio"
                  value={mode}
                  {...register("theme")}
                  className="sr-only"
                />
                <Icon className="h-4 w-4 shrink-0" aria-hidden />
                <span className="truncate">
                  {mode === "system"
                    ? t("account.themeSystemShort", "System")
                    : mode === "dark"
                      ? t("theme.dark", "Dark")
                      : t("theme.light", "Light")}
                </span>
              </label>
            ))}
          </div>
        </fieldset>

        <div className="space-y-2">
          <Label htmlFor="locale" className="flex items-center gap-2">
            <Globe className="h-4 w-4 text-text-muted" aria-hidden />
            {t("settings.appearance.language", "Display language")}
          </Label>
          <NativeSelect id="locale" {...register("locale")}>
            <option value="vi" lang="vi">
              Tiếng Việt
            </option>
            <option value="en" lang="en">
              English
            </option>
          </NativeSelect>
        </div>

        <div className="mt-auto flex justify-end border-t border-border-subtle pt-4">
          <Button
            type="submit"
            disabled={!isDirty}
            isLoading={isSaving}
            className="w-full sm:w-auto"
          >
            {isSaving
              ? t("account.saving", "Saving...")
              : t("settings.appearance.save", "Save appearance")}
          </Button>
        </div>
      </form>
      {/* Below the Save button, not above the fields: a message appearing
          over the form pushed the select and the button down under the
          learner's finger, and with a language change in the same moment the
          whole card jumped. Nothing visible sits under it to move. */}
      <StatusAlert message={statusMessage} />
    </section>
  );
};

/**
 * Paints a saved theme and language the way the header switchers do: the
 * theme spreads from the Save button, a language change pulls focus. Both at
 * once share the theme's reveal — a second transition would cut the first off.
 */
function applySaved(
  updated: UserPreferences,
  origin: RevealOrigin | undefined,
): void {
  const theme = updated.theme;
  const locale = isLocale(updated.locale) ? updated.locale : undefined;
  const painted = theme === "system" ? systemTheme() : theme;
  if (theme && painted !== currentTheme()) {
    void withPageTransition(
      "theme",
      () => {
        applyThemeChoice(theme);
      },
      { origin },
    );
    if (locale) setLocale(locale);
    return;
  }
  if (theme) applyThemeChoice(theme);
  if (locale && locale !== currentLocale()) {
    void switchLocale(locale, (change) =>
      withPageTransition("locale", async () => {
        await change();
      }),
    );
  } else if (locale) {
    setLocale(locale);
  }
}
