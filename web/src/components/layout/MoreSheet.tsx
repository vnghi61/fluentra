import * as React from "react";
import { Link } from "@tanstack/react-router";
import {
  ChevronRight,
  FileText,
  Globe,
  GraduationCap,
  Lock,
  LogIn,
  Monitor,
  Moon,
  Palette,
  ShieldCheck,
  Sparkles,
  Sun,
  User,
  UserPlus,
  type LucideIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Locale } from "@/i18n";
import type { ThemeChoice } from "@/lib/theme";
import { revealOriginFrom, type RevealOrigin } from "@/lib/view-transition";
import { cn } from "@/lib/utils";

export interface MoreSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  signedIn: boolean;
  isAdmin: boolean;
  /** Theme and language switchers; the fallback when no theme setter is passed. */
  controls?: React.ReactNode;
  themeChoice?: ThemeChoice | undefined;
  onThemeChoice?:
    ((choice: ThemeChoice, origin?: RevealOrigin) => void) | undefined;
  locale?: Locale | undefined;
  onLocale?: ((locale: Locale) => void) | undefined;
  /** Where focus goes back to when the panel closes: the More tab. */
  returnFocusRef?: React.RefObject<HTMLElement | null> | undefined;
}

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
  Icon: LucideIcon;
}[];

/** The endonym: a language is offered in its own language, never translated. */
const localeNames: Record<Locale, string> = { en: "English", vi: "Tiếng Việt" };

const cardClass =
  "overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-xs";
const rowClass =
  "flex min-h-[64px] w-full items-center gap-3.5 px-4 py-3 text-left transition-colors hover:bg-surface-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary motion-reduce:transition-none";

function RowBody({
  Icon,
  title,
  subtitle,
}: {
  Icon: LucideIcon;
  title: string;
  subtitle?: string | undefined;
}): React.JSX.Element {
  return (
    <>
      <Icon className="h-5 w-5 shrink-0 text-text" aria-hidden="true" />
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-semibold text-text">{title}</span>
        {subtitle && (
          <span className="block truncate text-sm text-text-muted">
            {subtitle}
          </span>
        )}
      </span>
      <ChevronRight
        className="h-4 w-4 shrink-0 text-text-muted"
        aria-hidden="true"
      />
    </>
  );
}

/**
 * The mobile "More" screen.
 *
 * A full-height panel above the content and under the bottom bar, so the bar
 * stays usable and shows More as the current tab. Lazy: it is only fetched the
 * first time More is tapped.
 */
export default function MoreSheet({
  open,
  onOpenChange,
  signedIn,
  isAdmin,
  controls,
  themeChoice = "system",
  onThemeChoice,
  locale,
  onLocale,
  returnFocusRef,
}: MoreSheetProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const headingRef = React.useRef<HTMLHeadingElement>(null);
  const headingId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    headingRef.current?.focus();
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onOpenChange(false);
    };
    document.addEventListener("keydown", onKey);
    const returnTo = returnFocusRef?.current;
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", onKey);
      returnTo?.focus();
    };
  }, [open, onOpenChange, returnFocusRef]);

  if (!open) return null;

  const close = () => onOpenChange(false);

  const destinations = [
    {
      to: "/exams",
      Icon: GraduationCap,
      title: t("nav.exams", "Exams"),
      subtitle: t(
        "nav.moreDescriptions.exams",
        "Practice mock exams and tests",
      ),
    },
    {
      to: "/studio",
      Icon: Sparkles,
      title: t("nav.studio", "Studio"),
      subtitle: t(
        "nav.moreDescriptions.studio",
        "Author and manage learning content",
      ),
    },
    ...(signedIn && isAdmin
      ? [
          {
            to: "/admin",
            Icon: ShieldCheck,
            title: t("nav.admin", "Admin"),
            subtitle: t(
              "nav.moreDescriptions.admin",
              "Platform administration and analytics",
            ),
          },
        ]
      : []),
  ];

  const settingsGroups = [
    [
      {
        // Learning preferences (goal, language, theme) now live in Profile.
        section: "profile",
        Icon: User,
        title: t("more.profile", "Profile and avatar"),
        subtitle: t("more.profileAndPrefsDesc"),
      },
      {
        section: "learning",
        Icon: GraduationCap,
        title: t("more.learning", "Learning profile"),
        subtitle: t("more.learningDesc", "Level, goals, learning path"),
      },
    ],
    [
      {
        section: "security",
        Icon: Lock,
        title: t("more.security", "Security and devices"),
        subtitle: t("more.securityDesc", "Password, signed-in devices"),
      },
      {
        section: "privacy",
        Icon: FileText,
        title: t("more.privacy", "Data and privacy"),
        subtitle: t("more.privacyDesc", "Export data, delete account"),
      },
    ],
  ];

  const otherLocale: Locale | undefined =
    locale === "vi" ? "en" : locale === "en" ? "vi" : undefined;

  return (
    <div
      role="dialog"
      aria-modal="true"
      // This screen locks the body itself (above). Opting out of the global
      // `html:has([aria-modal])` lock matters: with both <html> and <body>
      // clipped, the document shrinks to one viewport and the page behind
      // snaps to the top — the learner closed More, after a language switch
      // say, and found the page scrolled back to its start.
      data-scroll-lock="managed"
      aria-labelledby={headingId}
      className="fixed inset-x-0 top-0 bottom-[calc(4rem+env(safe-area-inset-bottom))] z-40 overflow-y-auto overscroll-contain bg-surface-muted md:hidden"
    >
      <div className="mx-auto w-full max-w-xl space-y-4 px-4 pt-[calc(1.25rem+env(safe-area-inset-top))] pb-6">
        <div>
          <h1
            ref={headingRef}
            id={headingId}
            tabIndex={-1}
            className="text-2xl font-bold tracking-tight text-text focus:outline-none"
          >
            {t("nav.more", "More")}
          </h1>
          <p className="mt-1 text-sm text-text-muted">
            {t("more.subtitle", "Manage your account and other options.")}
          </p>
        </div>

        {onThemeChoice ? (
          <section
            aria-labelledby={`${headingId}-appearance`}
            className={cn(cardClass, "p-4")}
          >
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
                <Palette className="h-5 w-5" aria-hidden="true" />
              </span>
              <span className="min-w-0">
                <span
                  id={`${headingId}-appearance`}
                  className="block text-sm font-semibold text-text"
                >
                  {t("more.appearance", "Appearance")}
                </span>
                <span className="block text-sm text-text-muted">
                  {t("more.appearanceDesc", "Customise how the app looks")}
                </span>
              </span>
            </div>

            <div
              role="radiogroup"
              aria-labelledby={`${headingId}-appearance`}
              className="mt-3 grid grid-cols-3 gap-2"
            >
              {themeOptions.map(({ value, labelKey, fallback, Icon }) => {
                const selected = themeChoice === value;
                return (
                  <label
                    key={value}
                    className={cn(
                      "flex min-h-[64px] cursor-pointer flex-col items-center justify-center gap-1 rounded-xl border px-1 py-2 text-center text-xs transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary motion-reduce:transition-none",
                      selected
                        ? "border-primary bg-primary/10 font-semibold text-primary-accent"
                        : "border-border-subtle text-text-muted hover:bg-surface-muted",
                    )}
                  >
                    <input
                      type="radio"
                      name={`${headingId}-theme`}
                      value={value}
                      checked={selected}
                      // React fires a radio's onChange from its click, so
                      // a tap carries the point the theme spreads from.
                      onChange={(event) =>
                        onThemeChoice(value, revealOriginFrom(event))
                      }
                      className="sr-only"
                    />
                    <Icon className="h-5 w-5" aria-hidden="true" />
                    {t(labelKey, fallback)}
                  </label>
                );
              })}
            </div>

            {themeChoice === "system" && (
              <div className="mt-3 flex gap-3 rounded-xl bg-primary/5 p-3">
                <Monitor
                  className="mt-0.5 h-4 w-4 shrink-0 text-primary-accent"
                  aria-hidden="true"
                />
                <span>
                  <span className="block text-sm font-semibold text-text">
                    {t("more.systemTitle", "Follows your device")}
                  </span>
                  <span className="block text-sm text-text-muted">
                    {t(
                      "more.systemDesc",
                      "The app switches automatically with your device setting.",
                    )}
                  </span>
                </span>
              </div>
            )}

            {locale && onLocale && otherLocale && (
              <button
                type="button"
                onClick={() => onLocale(otherLocale)}
                className={cn(
                  rowClass,
                  "-mx-4 mt-3 -mb-4 w-[calc(100%+2rem)] border-t border-border-subtle",
                )}
              >
                <RowBody
                  Icon={Globe}
                  title={t("more.language", "App language")}
                  subtitle={localeNames[locale]}
                />
              </button>
            )}
          </section>
        ) : controls ? (
          <section className={cn(cardClass, "p-4")}>
            <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-text-muted">
              {t("nav.appearanceAndLanguage", "Appearance & language")}
            </h2>
            <div className="flex flex-wrap items-center gap-2">{controls}</div>
          </section>
        ) : null}

        <nav
          aria-label={t("nav.more", "More")}
          className={cn(cardClass, "divide-y divide-border-subtle")}
        >
          {destinations.map(({ to, Icon, title, subtitle }) => (
            <Link key={to} to={to} onClick={close} className={rowClass}>
              <RowBody Icon={Icon} title={title} subtitle={subtitle} />
            </Link>
          ))}
        </nav>

        {signedIn ? (
          settingsGroups.map((group, index) => (
            <div
              key={index}
              className={cn(cardClass, "divide-y divide-border-subtle")}
            >
              {group.map(({ section, Icon, title, subtitle }) => (
                <Link
                  key={section}
                  to="/settings"
                  search={{ section }}
                  onClick={close}
                  className={rowClass}
                >
                  <RowBody Icon={Icon} title={title} subtitle={subtitle} />
                </Link>
              ))}
            </div>
          ))
        ) : (
          <div className={cn(cardClass, "divide-y divide-border-subtle")}>
            <Link to="/login" onClick={close} className={rowClass}>
              <RowBody Icon={LogIn} title={t("nav.signIn", "Sign in")} />
            </Link>
            <Link to="/register" onClick={close} className={rowClass}>
              <RowBody
                Icon={UserPlus}
                title={t("nav.createAccount", "Create account")}
              />
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
