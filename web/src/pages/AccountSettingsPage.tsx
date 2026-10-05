import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearch } from "@tanstack/react-router";
import {
  ArrowLeft,
  ChevronRight,
  FileText,
  GraduationCap,
  Lock,
  Settings as SettingsIcon,
  Sliders,
  User,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  accountApi,
  type UserPreferences,
  type UserProfile,
  ProfileSettings,
  LearningProfileSettings,
  PreferencesSettings,
  SecuritySettings,
  DataPrivacySettings,
} from "@/features/account";
import { authApi } from "@/features/auth";
import { useAuthStore } from "@/stores/authStore";

type TabKey = "profile" | "learning" | "preferences" | "security" | "privacy";

const VALID_TABS: readonly TabKey[] = [
  "profile",
  "learning",
  "preferences",
  "security",
  "privacy",
];

function isTabKey(key: unknown): key is TabKey {
  return (
    typeof key === "string" && (VALID_TABS as readonly string[]).includes(key)
  );
}

export function AccountSettingsPage(): React.JSX.Element {
  const { t } = useTranslation();
  const search: Record<string, unknown> = useSearch({ strict: false });
  const rawSection =
    typeof search["section"] === "string"
      ? search["section"].trim()
      : undefined;
  const activeSection: TabKey | undefined = isTabKey(rawSection)
    ? rawSection
    : undefined;
  const activeTab: TabKey = activeSection ?? "profile";

  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [preferences, setPreferences] = useState<UserPreferences | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);

  const handleSelectTab = (key: TabKey) => {
    void navigate({
      to: "/settings",
      search: { section: key },
    });
  };

  const handleBack = () => {
    void navigate({
      to: "/settings",
    });
  };

  useEffect(() => {
    async function loadAccountData() {
      setIsLoading(true);
      setError(null);
      try {
        const [meData, prefData] = await Promise.all([
          accountApi.getMe(),
          accountApi.getPreferences(),
        ]);
        setProfile(meData);
        setPreferences(prefData);
      } catch (err: unknown) {
        setError(
          err instanceof Error
            ? err.message
            : t(
                "page.failedToLoadAccountSettings",
                "Failed to load account settings.",
              ),
        );
      } finally {
        setIsLoading(false);
      }
    }

    void loadAccountData();
  }, [t]);

  const handleLoggedOut = async () => {
    try {
      await authApi.logout();
    } finally {
      clearAuth();
      void navigate({ to: "/login" });
    }
  };

  if (isLoading) {
    return (
      <div role="status" className="mx-auto max-w-5xl space-y-5">
        <span className="sr-only">
          {t("page.loadingYourSettings", "Loading your settings...")}
        </span>
        <Skeleton className="h-8 w-48" />
        <div className="grid gap-5 md:grid-cols-[220px_minmax(0,1fr)]">
          <div className="space-y-3">
            {Array.from({ length: 5 }, (_, index) => (
              <Skeleton key={index} className="h-12 w-full" />
            ))}
          </div>
          <Skeleton className="h-64 w-full rounded-2xl" />
        </div>
      </div>
    );
  }

  if (error || !profile || !preferences) {
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

  const tabs = [
    {
      key: "profile" as TabKey,
      label: t("page.profileAvatar", "Profile & Avatar"),
      icon: User,
    },
    {
      key: "learning" as TabKey,
      label: t("page.learningProfile", "Learning Profile"),
      icon: GraduationCap,
    },
    {
      key: "preferences" as TabKey,
      label: t("page.learningPreferences", "Learning Preferences"),
      icon: Sliders,
    },
    {
      key: "security" as TabKey,
      label: t("page.securityDevices", "Security & Devices"),
      icon: Lock,
    },
    {
      key: "privacy" as TabKey,
      label: t("page.dataPrivacy", "Data & Privacy"),
      icon: FileText,
    },
  ];

  const currentTab = tabs.find((tab) => tab.key === activeTab);

  return (
    // min-w-0 is what lets the tab row's own `overflow-x-auto` actually clip:
    // without it the container is sized by its content and the whole page
    // scrolls sideways instead (R6, measured at 320 px).
    <div className="mx-auto min-w-0 max-w-[1180px] space-y-5">
      {/* Mobile Subpage Header with Back button */}
      {activeSection && (
        <div className="flex md:hidden items-center gap-3">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleBack}
            className="gap-1.5 px-2 text-text-muted hover:text-text min-h-[44px] min-w-[44px]"
            aria-label={t("page.backToSettings", "Back to Settings")}
          >
            <ArrowLeft className="h-5 w-5 shrink-0" aria-hidden="true" />
            <span className="text-sm font-medium">
              {t("page.accountSettingsShort", "Settings")}
            </span>
          </Button>
          <span className="text-text-muted" aria-hidden="true">
            |
          </span>
          <h2 className="truncate text-lg font-semibold text-text">
            {currentTab?.label}
          </h2>
        </div>
      )}

      {/* Main Page Header */}
      <header className={cn("space-y-1", activeSection && "hidden md:block")}>
        <h1 className="text-2xl font-bold text-text flex items-center gap-2.5">
          <SettingsIcon
            className="h-6 w-6 text-primary-accent"
            aria-hidden="true"
          />
          {t("page.accountSettings", "Account Settings")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "page.settingsDescription",
            "Your profile, learning preferences and account security.",
          )}
        </p>
      </header>

      <div className="grid items-start gap-5 md:grid-cols-[232px_minmax(0,1fr)] lg:gap-6">
        {/* Grouped settings navigation: mobile index list, or desktop side-tab */}
        <nav
          aria-label={t("page.accountSettings", "Account Settings")}
          className={cn(
            "grid gap-1 rounded-2xl border border-border-subtle bg-surface-card p-2 shadow-[var(--shadow-card)] md:sticky md:top-4",
            activeSection && "hidden md:grid",
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
                  "flex min-h-[44px] min-w-[44px] cursor-pointer max-md:min-h-[56px] items-center gap-3 rounded-xl px-3 text-left text-[15px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary md:min-h-[44px] md:text-sm",
                  // The mobile hub is a menu: nothing in it is "current" yet,
                  // so the highlight applies only beside an open panel.
                  isActive
                    ? "text-text hover:bg-surface-muted md:bg-primary/10 md:text-primary-accent md:hover:bg-primary/10"
                    : "text-text hover:bg-surface-muted md:text-text-muted md:hover:text-text",
                )}
              >
                <Icon
                  className="h-5 w-5 shrink-0 md:h-4 md:w-4"
                  aria-hidden="true"
                />
                <span className="min-w-0 flex-1 truncate">{tab.label}</span>
                <ChevronRight
                  className="h-4 w-4 shrink-0 text-text-muted md:hidden"
                  aria-hidden="true"
                />
              </button>
            );
          })}
        </nav>

        {/* Tab Panels: hidden on mobile index, visible on mobile subpage or desktop */}
        <div
          className={cn("min-w-0", !activeSection && "hidden md:block")}
          role="region"
          aria-label={currentTab?.label}
        >
          {activeTab === "profile" && (
            <ProfileSettings
              initialProfile={profile}
              onProfileUpdated={(updated) => setProfile(updated)}
            />
          )}

          {activeTab === "learning" && <LearningProfileSettings />}

          {activeTab === "preferences" && (
            <PreferencesSettings
              initialPreferences={preferences}
              onPreferencesUpdated={(updated) => setPreferences(updated)}
            />
          )}

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
              onProfileUpdated={(updated) => setProfile(updated)}
            />
          )}
        </div>
      </div>
    </div>
  );
}
