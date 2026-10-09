import * as React from "react";
import { Check, Languages, Monitor, Moon, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";

import { SheetDialog } from "@/components/ui/sheet";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useViewport } from "@/hooks/useViewport";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";
import { cn } from "@/lib/utils";
import type { ThemeChoice } from "@/lib/theme";
import { revealOriginFrom, type RevealOrigin } from "@/lib/view-transition";

export interface ThemeLanguageControlsProps {
  themeChoice: ThemeChoice;
  locale: Locale;
  /** `origin` is where the choice was made; the new theme spreads from it. */
  onThemeChoice: (choice: ThemeChoice, origin?: RevealOrigin) => void;
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
 * Below 768 px (md breakpoint), triggers open accessible bottom sheets with
 * full touch targets. At 768 px and up, they use Radix dropdowns.
 */
export const ThemeLanguageControls: React.FC<ThemeLanguageControlsProps> = ({
  themeChoice,
  locale,
  onThemeChoice,
  onLocale,
}) => {
  const { t } = useTranslation();
  const { width } = useViewport();
  const isMobile = width < 768;

  const [themeSheetOpen, setThemeSheetOpen] = React.useState(false);
  const [langSheetOpen, setLangSheetOpen] = React.useState(false);

  const active =
    themeOptions.find((o) => o.value === themeChoice) ?? systemOption;
  const ActiveIcon = active.Icon;
  // The trigger's icon turns into the new one when the learner changes the
  // theme, in step with the reveal; not on the first paint.
  const [initialChoice] = React.useState(themeChoice);
  const iconClass = cn(
    "h-5 w-5",
    themeChoice !== initialChoice &&
      "motion-safe:animate-in motion-safe:spin-in-90 motion-safe:zoom-in-50 motion-safe:fade-in motion-safe:duration-500",
  );

  const themeAriaLabel = `${t("theme.label", "Theme")}: ${t(active.labelKey, active.fallback)}`;
  const langAriaLabel = `${t("language.label", "Language")}: ${localeNames[locale]}`;

  if (isMobile) {
    return (
      <div className="flex items-center gap-2">
        {/* Mobile Theme Trigger */}
        <button
          type="button"
          aria-haspopup="dialog"
          aria-expanded={themeSheetOpen}
          aria-label={themeAriaLabel}
          onClick={() => setThemeSheetOpen(true)}
          className={trigger}
        >
          <ActiveIcon
            key={active.value}
            className={iconClass}
            aria-hidden="true"
          />
        </button>

        {/* Mobile Language Trigger */}
        <button
          type="button"
          aria-haspopup="dialog"
          aria-expanded={langSheetOpen}
          aria-label={langAriaLabel}
          onClick={() => setLangSheetOpen(true)}
          className={trigger}
        >
          <Languages className="h-5 w-5" aria-hidden="true" />
        </button>

        {/* Mobile Theme Bottom Sheet */}
        <SheetDialog
          open={themeSheetOpen}
          onOpenChange={setThemeSheetOpen}
          title={t("theme.label", "Theme")}
        >
          <div className="flex flex-col gap-1 pb-4" role="radiogroup">
            {themeOptions.map(({ value, labelKey, fallback, Icon }) => {
              const isSelected = themeChoice === value;
              return (
                <button
                  key={value}
                  type="button"
                  role="menuitemradio"
                  aria-checked={isSelected}
                  onClick={(event) => {
                    onThemeChoice(value, revealOriginFrom(event));
                    setThemeSheetOpen(false);
                  }}
                  className={cn(
                    "flex min-h-[48px] w-full items-center gap-3.5 rounded-xl px-3 py-2.5 transition-colors cursor-pointer text-left",
                    isSelected
                      ? "bg-surface-muted font-medium text-text"
                      : "hover:bg-surface-muted text-text-muted hover:text-text",
                  )}
                >
                  <div
                    className={cn(
                      "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl",
                      isSelected
                        ? "bg-primary/10 text-primary-accent"
                        : "bg-surface-muted text-text-muted",
                    )}
                  >
                    <Icon className="h-5 w-5" aria-hidden="true" />
                  </div>
                  <span className="flex-1 text-sm font-medium">
                    {t(labelKey, fallback)}
                  </span>
                  {isSelected && (
                    <Check
                      className="h-5 w-5 shrink-0 text-primary-accent"
                      aria-hidden="true"
                    />
                  )}
                </button>
              );
            })}
          </div>
        </SheetDialog>

        {/* Mobile Language Bottom Sheet */}
        <SheetDialog
          open={langSheetOpen}
          onOpenChange={setLangSheetOpen}
          title={t("language.label", "Language")}
        >
          <div className="flex flex-col gap-1 pb-4" role="radiogroup">
            {SUPPORTED_LOCALES.map((value) => {
              const isSelected = locale === value;
              return (
                <button
                  key={value}
                  type="button"
                  role="menuitemradio"
                  aria-checked={isSelected}
                  onClick={() => {
                    onLocale(value);
                    setLangSheetOpen(false);
                  }}
                  className={cn(
                    "flex min-h-[48px] w-full items-center gap-3.5 rounded-xl px-3 py-2.5 transition-colors cursor-pointer text-left",
                    isSelected
                      ? "bg-surface-muted font-medium text-text"
                      : "hover:bg-surface-muted text-text-muted hover:text-text",
                  )}
                >
                  <div
                    className={cn(
                      "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl",
                      isSelected
                        ? "bg-primary/10 text-primary-accent"
                        : "bg-surface-muted text-text-muted",
                    )}
                  >
                    <Languages className="h-5 w-5" aria-hidden="true" />
                  </div>
                  <span className="flex-1 text-sm font-medium" lang={value}>
                    {localeNames[value]}
                  </span>
                  {isSelected && (
                    <Check
                      className="h-5 w-5 shrink-0 text-primary-accent"
                      aria-hidden="true"
                    />
                  )}
                </button>
              );
            })}
          </div>
        </SheetDialog>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <DropdownMenu>
        {/*
          aria-label names the control and its current value, so a screen reader
          hears "Theme, Dark" rather than an unnamed button — the failure that
          took twenty-three E2E specs down when the account button lost its text.
        */}
        <DropdownMenuTrigger aria-label={themeAriaLabel} className={trigger}>
          <ActiveIcon
            key={active.value}
            className={iconClass}
            aria-hidden="true"
          />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>{t("theme.label", "Theme")}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {themeOptions.map(({ value, labelKey, fallback, Icon }) => {
            const isSelected = themeChoice === value;
            return (
              <DropdownMenuItem
                key={value}
                onSelect={(event) =>
                  onThemeChoice(value, revealOriginFrom(event))
                }
                className="flex items-center gap-2.5 rounded-xl"
              >
                <div
                  className={cn(
                    "flex h-6 w-6 shrink-0 items-center justify-center rounded-lg",
                    isSelected
                      ? "bg-primary/10 text-primary-accent"
                      : "text-text-muted",
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                </div>
                <span className="flex-1">{t(labelKey, fallback)}</span>
                {isSelected && (
                  <Check
                    className="h-4 w-4 shrink-0 text-primary-accent"
                    aria-hidden="true"
                  />
                )}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger aria-label={langAriaLabel} className={trigger}>
          <Languages className="h-5 w-5" aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>
            {t("language.label", "Language")}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          {SUPPORTED_LOCALES.map((value) => {
            const isSelected = locale === value;
            return (
              <DropdownMenuItem
                key={value}
                onSelect={() => onLocale(value)}
                className="flex items-center gap-2.5 rounded-xl"
              >
                {/* `lang` so a screen reader pronounces the endonym correctly. */}
                <span className="flex-1" lang={value}>
                  {localeNames[value]}
                </span>
                {isSelected && (
                  <Check
                    className="h-4 w-4 shrink-0 text-primary-accent"
                    aria-hidden="true"
                  />
                )}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
