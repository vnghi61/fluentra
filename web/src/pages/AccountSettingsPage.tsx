import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  ChevronRight,
  FileText,
  GraduationCap,
  ShieldCheck,
  User,
  type LucideIcon,
} from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  accountApi,
  type UserPreferences,
  type UserProfile,
  ProfileHeroCard,
  ProfileCompletenessCard,
  ProfileSettings,
  LearningProfileSettings,
  PreferencesSettings,
  SecuritySettings,
  DataPrivacySettings,
} from "@/features/account";
import { authApi } from "@/features/auth";
import { useAuthStore } from "@/stores/authStore";

type TabKey = "profile" | "learning" | "security" | "privacy";

const VALID_TABS: readonly TabKey[] = [
  "profile",
  "learning",
  "security",
  "privacy",
];

/**
 * Sections that were folded into another tab. Old links (the More sheet,
 * bookmarks, e-mails) keep landing on the tab that now holds the content.
 */
const TAB_ALIASES: Readonly<Record<string, TabKey>> = {
  preferences: "profile",
};

/** Where an aliased section's content now lives on its new tab. */
const ALIAS_ANCHORS: Readonly<Record<string, string>> = {
  preferences: "learning-preferences",
};

function isTabKey(key: unknown): key is TabKey {
  return (
    typeof key === "string" && (VALID_TABS as readonly string[]).includes(key)
  );
}

function resolveTab(key: string | undefined): TabKey | undefined {
  if (key === undefined) return undefined;
  if (isTabKey(key)) return key;
  return TAB_ALIASES[key];
}

interface TabDef {
  key: TabKey;
  icon: LucideIcon;
  label: string;
  short: string;
  description: string;
}

/** The query key the shell reads the learner's name and avatar from. */
const ME_KEY = ["account", "me"] as const;

export function AccountSettingsPage(): React.JSX.Element {
  const { t } = useTranslation();
  const search: Record<string, unknown> = useSearch({ strict: false });
  const rawSection =
    typeof search["section"] === "string"
      ? search["section"].trim()
      : undefined;
  const activeTab: TabKey = resolveTab(rawSection) ?? "profile";

  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [preferences, setPreferences] = useState<UserPreferences | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);

  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const clearAuth = useAuthStore((s) => s.clearAuth);
  const navRef = useRef<HTMLElement>(null);

  const handleSelectTab = (key: TabKey) => {
    void navigate({
      to: "/settings",
      search: { section: key },
    });
  };

  // A rename or a new avatar shows in the account menu at once: it reads
  // the same cached profile.
  const handleProfileUpdated = (updated: UserProfile) => {
    setProfile(updated);
    queryClient.setQueryData(ME_KEY, updated);
  };

  // Loaded once per visit. This effect used to depend on `t`, whose identity
  // changes with the language, so switching vi/en (here or from the header)
  // threw the page back to its skeleton and refetched: the content collapsed,
  // the window scrolled to the top, and the forms remounted with their state
  // gone — the "jerk" on a language change. Nothing here needs `t`; the
  // failure screen translates its own text at render time.
  useEffect(() => {
    let cancelled = false;
    async function loadAccountData() {
      setIsLoading(true);
      setLoadFailed(false);
      try {
        const [meData, prefData] = await Promise.all([
          accountApi.getMe(),
          accountApi.getPreferences(),
        ]);
        if (cancelled) return;
        setProfile(meData);
        setPreferences(prefData);
      } catch {
        if (!cancelled) setLoadFailed(true);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    void loadAccountData();
    return () => {
      cancelled = true;
    };
  }, []);

  // A link to a folded section opens its new tab scrolled to the content.
  const aliasAnchor =
    rawSection !== undefined ? ALIAS_ANCHORS[rawSection] : undefined;
  useEffect(() => {
    if (isLoading || !aliasAnchor) return;
    document.getElementById(aliasAnchor)?.scrollIntoView?.({ block: "start" });
  }, [isLoading, aliasAnchor]);

  // Below `lg` the sections are a horizontal strip that may scroll; keep the
  // active one in view. The strip's own scrollLeft only, never the window.
  useLayoutEffect(() => {
    const nav = navRef.current;
    if (!nav || nav.scrollWidth <= nav.clientWidth) return;
    const active = nav.querySelector<HTMLElement>('[aria-current="page"]');
    if (!active) return;
    nav.scrollLeft =
      active.offsetLeft - (nav.clientWidth - active.offsetWidth) / 2;
  }, [activeTab, isLoading]);

  const handleLoggedOut = async () => {
    try {
      await authApi.logout();
    } finally {
      clearAuth();
      void navigate({ to: "/login" });
    }
  };

  const tabs: TabDef[] = [
    {
      key: "profile",
      icon: User,
      label: t("page.profilePreferences", "Profile & Preferences"),
      short: t("settings.short.profile", "Profile"),
      description: t(
        "settings.desc.profile",
        "Manage your personal details, profile photo and how Fluentra looks for you.",
      ),
    },
    {
      key: "learning",
      icon: GraduationCap,
      label: t("page.learningProfile", "Learning Profile"),
      short: t("settings.short.learning", "Learning"),
      description: t(
        "settings.desc.learning",
        "Your goal, target level and weekly time shape your weekly study plan.",
      ),
    },
    {
      key: "security",
      icon: ShieldCheck,
      label: t("page.securityDevices", "Security & Devices"),
      short: t("settings.short.security", "Security"),
      description: t(
        "settings.desc.security",
        "Protect your account, manage the devices you are signed in on and control access.",
      ),
    },
    {
      key: "privacy",
      icon: FileText,
      label: t("page.dataPrivacy", "Data & Privacy"),
      short: t("settings.short.privacy", "Privacy"),
      description: t(
        "settings.desc.privacy",
        "Download a copy of your data or ask for your account to be deleted.",
      ),
    },
  ];
  const currentTab = tabs.find((tab) => tab.key === activeTab) ?? tabs[0]!;

  const header = (
    <PageHeader
      eyebrow={
        <span className="text-xs font-semibold uppercase tracking-wide text-primary-accent">
          {t("page.accountSettings", "Account Settings")}
        </span>
      }
      title={currentTab.label}
      description={currentTab.description}
    />
  );

  const nav = (
    <nav
      ref={navRef}
      aria-label={t("page.accountSettings", "Account Settings")}
      className={cn(
        // A scrolling strip of pills on phones and tablets; a card of rows
        // beside the content from `lg`. One element, so one tab order.
        "flex min-w-0 snap-x gap-2 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
        "lg:sticky lg:top-4 lg:snap-none lg:flex-col lg:gap-1 lg:overflow-visible lg:rounded-2xl lg:border lg:border-border-subtle lg:bg-surface-card lg:p-2 lg:pb-2 lg:shadow-[var(--shadow-card)]",
      )}
    >
      {tabs.map((tab) => {
        const Icon = tab.icon;
        const isActive = activeTab === tab.key;
        return (
          <button
            key={tab.key}
            type="button"
            onClick={() => handleSelectTab(tab.key)}
            aria-current={isActive ? "page" : undefined}
            className={cn(
              "group flex min-h-[44px] min-w-[44px] shrink-0 cursor-pointer snap-start items-center gap-2 whitespace-nowrap rounded-xl border px-3.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none",
              "lg:min-h-[52px] lg:w-full lg:gap-3 lg:border-transparent lg:px-2.5 lg:text-left",
              isActive
                ? "border-primary/40 bg-primary/10 text-primary-accent"
                : "border-border-subtle bg-surface-card text-text-muted hover:bg-surface-muted hover:text-text lg:bg-transparent",
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                "flex shrink-0 items-center justify-center lg:h-9 lg:w-9 lg:rounded-lg",
                isActive
                  ? "lg:bg-primary/15"
                  : "lg:bg-surface-muted lg:text-text-muted lg:group-hover:text-text",
              )}
            >
              <Icon className="h-4 w-4" />
            </span>
            <span className="sm:hidden">{tab.short}</span>
            <span className="hidden min-w-0 flex-1 truncate sm:inline">
              {tab.label}
            </span>
            <ChevronRight
              className="hidden h-4 w-4 shrink-0 opacity-60 lg:block"
              aria-hidden="true"
            />
          </button>
        );
      })}
    </nav>
  );

  if (isLoading) {
    return (
      <div className="mx-auto w-full min-w-0 max-w-[1440px] space-y-5">
        {header}
        <div className="grid items-start gap-5 lg:grid-cols-[240px_minmax(0,1fr)] xl:grid-cols-[264px_minmax(0,1fr)] xl:gap-8">
          {nav}
          <div role="status" className="min-w-0 space-y-4">
            <span className="sr-only">
              {t("page.loadingYourSettings", "Loading your settings...")}
            </span>
            <Skeleton className="h-40 w-full rounded-2xl" />
            <div className="grid gap-4 md:grid-cols-2">
              <Skeleton className="h-64 w-full rounded-2xl" />
              <Skeleton className="h-64 w-full rounded-2xl" />
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (loadFailed || !profile || !preferences) {
    return (
      <div className="mx-auto max-w-xl py-12">
        <ErrorState
          title={t("page.unableToLoadSettings", "Unable to load settings")}
          description={t(
            "page.anUnexpectedErrorOccurredWhile",
            "An unexpected error occurred while loading account details.",
          )}
          onRetry={() => window.location.reload()}
        />
      </div>
    );
  }

  return (
    // min-w-0 is what lets the section strip's own `overflow-x-auto` clip:
    // without it the container is sized by its content and the whole page
    // scrolls sideways instead (R6, measured at 320 px).
    <div className="mx-auto w-full min-w-0 max-w-[1440px] space-y-5 lg:space-y-6">
      {header}

      <div className="grid items-start gap-5 lg:grid-cols-[240px_minmax(0,1fr)] xl:grid-cols-[264px_minmax(0,1fr)] xl:gap-8">
        {nav}

        <div className="min-w-0" role="region" aria-label={currentTab.label}>
          {activeTab === "profile" && (
            <div className="space-y-4 lg:space-y-6">
              <div className="@container">
                <div className="grid items-stretch gap-4 @4xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
                  <ProfileHeroCard
                    profile={profile}
                    onAvatarUpdated={(avatarUrl) =>
                      handleProfileUpdated({
                        ...profile,
                        profile: { ...profile.profile, avatar_url: avatarUrl },
                      })
                    }
                  />
                  <ProfileCompletenessCard
                    profile={profile}
                    preferences={preferences}
                  />
                </div>
              </div>
              {/* Three across where each keeps ~330 px; otherwise the
                  details card takes a full row above the two short ones. */}
              <div className="@container">
                <div className="grid items-stretch gap-4 @2xl:grid-cols-2 @5xl:grid-cols-3">
                  <div className="min-w-0 @2xl:col-span-2 @5xl:col-span-1 [&>section]:h-full">
                    <ProfileSettings
                      initialProfile={profile}
                      onProfileUpdated={handleProfileUpdated}
                    />
                  </div>
                  <PreferencesSettings
                    id="learning-preferences"
                    initialPreferences={preferences}
                    onPreferencesUpdated={(updated) => setPreferences(updated)}
                  />
                </div>
              </div>
            </div>
          )}

          {activeTab === "learning" && <LearningProfileSettings />}

          {activeTab === "security" && (
            <SecuritySettings
              onLoggedOut={() => {
                void handleLoggedOut();
              }}
            />
          )}

          {activeTab === "privacy" && (
            <DataPrivacySettings
              initialProfile={profile}
              onProfileUpdated={handleProfileUpdated}
            />
          )}
        </div>
      </div>
    </div>
  );
}
