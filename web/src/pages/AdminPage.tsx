import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { BookA, BookOpen, Flag, Gauge, Shield, Users } from "lucide-react";
import {
  AdminUserList,
  AdminFeatureFlags,
  AdminAIUsage,
  AdminContentList,
  AdminVocabulary,
} from "@/features/admin";
import {
  PERMISSIONS,
  usePermissions,
} from "@/features/admin/model/permissions";

type AdminTab = "users" | "content" | "vocabulary" | "flags" | "ai";

export function AdminPage(): React.JSX.Element {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<AdminTab>("users");
  const { can, isLoading } = usePermissions();

  // The route already refuses non-admins. This is the finer cut: `admin` is one
  // role but its permissions are individually grantable, so an administrator
  // without `system.flags` must not be shown a Feature Flags tab whose every
  // action answers 403. The server still enforces both — see each operation's
  // x-permission in api/openapi/openapi.yaml.
  const tabs = [
    ...(can(PERMISSIONS.userList)
      ? [
          {
            key: "users" as AdminTab,
            label: t("page.learnerManagement"),
            icon: Users,
          },
        ]
      : []),
    ...(can(PERMISSIONS.contentEdit) ||
    can(PERMISSIONS.contentReview) ||
    can(PERMISSIONS.contentPublish)
      ? [
          {
            key: "content" as AdminTab,
            label: t("page.contentLibrary"),
            icon: BookOpen,
          },
        ]
      : []),
    ...(can(PERMISSIONS.contentEdit) || can(PERMISSIONS.contentCreate)
      ? [
          {
            key: "vocabulary" as AdminTab,
            label: t("page.vocabulary"),
            icon: BookA,
          },
        ]
      : []),
    ...(can(PERMISSIONS.systemFlags)
      ? [
          {
            key: "flags" as AdminTab,
            label: t("page.featureFlags"),
            icon: Flag,
          },
        ]
      : []),
    // The whole point of the AI usage view is that an administrator sees a
    // provider run out before a learner meets a queued word, and a component
    // that is exported but never rendered shows nobody anything.
    ...(can(PERMISSIONS.adminDashboard)
      ? [
          {
            key: "ai" as AdminTab,
            label: t("page.aiUsage"),
            icon: Gauge,
          },
        ]
      : []),
  ];

  // Nothing is rendered on a guess: until the read lands, and if it fails,
  // no administrative surface is offered.
  const visible = tabs.some((tab) => tab.key === activeTab)
    ? activeTab
    : tabs[0]?.key;

  return (
    <div className="max-w-6xl mx-auto space-y-8 py-4">
      {/* Header */}
      <header className="space-y-1">
        <div className="flex items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-md bg-primary/10 px-2 py-0.5 text-xs font-semibold text-primary-accent border border-primary/20 uppercase tracking-wider">
            <Shield className="h-3 w-3" />
            {t("page.administration")}
          </span>
        </div>
        <h1 className="text-2xl font-bold text-text">
          {t("page.platformAdministration")}
        </h1>
        <p className="text-sm text-text-muted">
          Manage platform learners, enforce moderation, and configure system
          feature flags.
        </p>
      </header>

      {/* Tabs */}
      <div className="flex border-b border-border-subtle gap-2 pb-px overflow-x-auto">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = visible === tab.key;
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
      <div>
        {isLoading ? (
          <p className="text-sm text-text-muted">Checking your permissions…</p>
        ) : tabs.length === 0 ? (
          <p className="text-sm text-text-muted">
            {t("page.yourAccountHoldsNoAdministrativePermissions")}
          </p>
        ) : (
          <>
            {visible === "users" && <AdminUserList />}
            {visible === "content" && <AdminContentList />}
            {visible === "vocabulary" && <AdminVocabulary />}
            {visible === "flags" && <AdminFeatureFlags />}
            {visible === "ai" && <AdminAIUsage />}
          </>
        )}
      </div>
    </div>
  );
}
