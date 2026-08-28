import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import {
  FileText,
  Loader2,
  Lock,
  Settings as SettingsIcon,
  Sliders,
  User,
  AlertCircle,
} from "lucide-react";
import {
  accountApi,
  type UserPreferences,
  type UserProfile,
  ProfileSettings,
  PreferencesSettings,
  SecuritySettings,
  DataPrivacySettings,
} from "@/features/account";
import { authApi } from "@/features/auth";
import { useAuthStore } from "@/stores/authStore";

type TabKey = "profile" | "preferences" | "security" | "privacy";

export function AccountSettingsPage(): React.JSX.Element {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<TabKey>("profile");
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [preferences, setPreferences] = useState<UserPreferences | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);

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
      <div className="flex min-h-[400px] flex-col items-center justify-center space-y-4">
        <Loader2 className="h-8 w-8 animate-spin text-primary-accent" />
        <p className="text-sm text-text-muted">
          {t("page.loadingYourSettings", "Loading your settings...")}
        </p>
      </div>
    );
  }

  if (error || !profile || !preferences) {
    return (
      <div className="mx-auto max-w-xl py-12">
        <div className="rounded-xl border border-danger/30 bg-danger/10 p-6 text-center space-y-4">
          <AlertCircle className="mx-auto h-8 w-8 text-danger-accent" />
          <h2 className="text-base font-semibold text-danger-accent">
            {t("page.unableToLoadSettings", "Unable to load settings")}
          </h2>
          <p className="text-xs text-danger-accent">
            {error ||
              t(
                "page.anUnexpectedErrorOccurredWhile",
                "An unexpected error occurred while loading account details.",
              )}
          </p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="rounded-lg bg-danger px-4 py-2 text-xs font-medium text-white hover:bg-danger transition-colors"
          >
            {t("page.retry", "Retry")}
          </button>
        </div>
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

  return (
    // min-w-0 is what lets the tab row's own `overflow-x-auto` actually clip:
    // without it the container is sized by its content and the whole page
    // scrolls sideways instead (R6, measured at 320 px).
    <div className="max-w-5xl mx-auto min-w-0 space-y-8 py-4">
      {/* Header */}
      <header className="space-y-1">
        <h1 className="text-2xl font-bold text-text flex items-center gap-2.5">
          <SettingsIcon className="h-7 w-7 text-primary-accent" />
          {t("page.accountSettings", "Account Settings")}
        </h1>
        <p className="text-sm text-text-muted">
          Manage your personal profile, study preferences, security credentials,
          and privacy options.
        </p>
      </header>

      {/* Tabs Navigation */}
      <div className="flex w-full min-w-0 border-b border-border-subtle overflow-x-auto gap-2 pb-px scrollbar-none">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => setActiveTab(tab.key)}
              className={`flex items-center gap-2 whitespace-nowrap px-4 py-3 text-sm font-medium border-b-2 transition-colors min-h-[44px] cursor-pointer ${
                isActive
                  ? "border-primary text-primary-accent bg-primary/5 rounded-t-lg"
                  : "border-transparent text-text-muted hover:text-text hover:border-border-subtle"
              }`}
            >
              <Icon className="h-4 w-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Tab Panels */}
      <div className="pt-2">
        {activeTab === "profile" && (
          <ProfileSettings
            initialProfile={profile}
            onProfileUpdated={(updated) => setProfile(updated)}
          />
        )}

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
  );
}
