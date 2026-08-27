import React from "react";
import { Check, Languages, Monitor, Moon, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";
import type { ThemeChoice } from "@/lib/theme";

export interface ThemeLanguageControlsProps {
  themeChoice: ThemeChoice;
  locale: Locale;
  onThemeChoice: (choice: ThemeChoice) => void;
  onLocale: (locale: Locale) => void;
}

const trigger =
  "flex items-center justify-center h-11 w-11 min-h-[44px] min-w-[44px] rounded-lg " +
  "text-text-muted hover:bg-surface-muted hover:text-text transition-colors cursor-pointer " +
  "outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 " +
  "focus-visible:ring-offset-surface-card";

/** Three options, because `system` is a real answer and not a missing one. */
const themeOptions = [
  { value: "light", labelKey: "theme.light", fallback: "Light", Icon: Sun },
  { value: "dark", labelKey: "theme.dark", fallback: "Dark", Icon: Moon },
  {
    value: "system",
    labelKey: "theme.system",
    fallback: "System",
    Icon: Monitor,
  },
] as const satisfies readonly {
  value: ThemeChoice;
  labelKey: string;
  fallback: string;
  Icon: typeof Sun;
}[];

/** `system` is the fallback because it is what an unset choice means. */
const systemOption = themeOptions[2];

/** The endonym: a language is offered in its own language, never translated. */
const localeNames: Record<Locale, string> = {
  en: "English",
  vi: "Tiếng Việt",
};

/**
 * The theme and language switchers.
 *
 * They live in `features/account` rather than in `components/layout` because
 * they read and write the learner's preferences, and a component may not reach
 * into a store or a feature. AppShell takes them as a slot, the way it already
 * takes `user` and `onLogout`.
 */
export const ThemeLanguageControls: React.FC<ThemeLanguageControlsProps> = ({
  themeChoice,
  locale,
  onThemeChoice,
  onLocale,
}) => {
  const { t } = useTranslation();
  const active =
    themeOptions.find((o) => o.value === themeChoice) ?? systemOption;
  const ActiveIcon = active.Icon;

  return (
    <div className="flex items-center gap-1">
      <DropdownMenu>
        {/*
          aria-label names the control and its current value, so a screen reader
          hears "Theme, Dark" rather than an unnamed button — the failure that
          took twenty-three E2E specs down when the account button lost its text.
        */}
        <DropdownMenuTrigger
          aria-label={`${t("theme.label", "Theme")}: ${t(active.labelKey, active.fallback)}`}
          className={trigger}
        >
          <ActiveIcon className="h-5 w-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>{t("theme.label", "Theme")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {themeOptions.map(({ value, labelKey, fallback, Icon }) => (
            <DropdownMenuItem key={value} onSelect={() => onThemeChoice(value)}>
              <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
              <span className="flex-1">{t(labelKey, fallback)}</span>
              {themeChoice === value && (
                <Check
                  className="h-4 w-4 shrink-0 text-primary-accent"
                  aria-hidden="true"
                />
              )}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={`${t("language.label", "Language")}: ${localeNames[locale]}`}
          className={trigger}
        >
          <Languages className="h-5 w-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>
            {t("language.label", "Language")}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          {SUPPORTED_LOCALES.map((value) => (
            <DropdownMenuItem key={value} onSelect={() => onLocale(value)}>
              {/* `lang` so a screen reader pronounces the endonym correctly. */}
              <span className="flex-1" lang={value}>
                {localeNames[value]}
              </span>
              {locale === value && (
                <Check
                  className="h-4 w-4 shrink-0 text-primary-accent"
                  aria-hidden="true"
                />
              )}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
