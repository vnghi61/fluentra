import React from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Shield } from "lucide-react";
import {
  AdminUserList,
  AdminFeatureFlags,
  AdminAIUsage,
  AdminContentList,
  AdminReviewQueue,
  AdminQuestionBank,
  AdminReportedContentList,
  AdminVocabulary,
  AdminStudioModeration,
  AdminPayoutsList,
} from "@/features/admin";
import {
  type AdminSectionKey,
  useVisibleAdminSections,
} from "@/features/admin/model/sections";

export function AdminPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const { sections, isLoading } = useVisibleAdminSections();

  // The address decides, not component state. /admin names no section, and a
  // section the permissions do not grant is not one to render, so both fall
  // back to the first section this administrator actually holds.
  const asked = params["section"];
  const visible: AdminSectionKey | undefined =
    sections.find((section) => section.key === asked)?.key ?? sections[0]?.key;

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
        <p className="text-sm text-text-muted">{t("page.adminIntro")}</p>
      </header>

      {/* The sections live in the sidebar now. It is drawn from `md` up, so this
          row is what carries them on a phone — the same destinations, not a
          second source of truth. */}
      <div className="flex border-b border-border-subtle gap-2 pb-px overflow-x-auto md:hidden">
        {sections.map(({ key, path, labelKey, labelFallback, Icon }) => {
          const isActive = visible === key;
          return (
            <button
              key={key}
              type="button"
              onClick={() => void navigate({ to: path })}
              className={`flex items-center gap-2 whitespace-nowrap px-4 py-3 text-sm font-medium border-b-2 transition-colors min-h-[44px] cursor-pointer ${
                isActive
                  ? "border-primary text-primary-accent bg-primary/5 rounded-t-lg"
                  : "border-transparent text-text-muted hover:text-text hover:border-border-subtle"
              }`}
            >
              <Icon className="h-4 w-4" />
              {t(labelKey, labelFallback)}
            </button>
          );
        })}
      </div>

      {/* Tab Panels */}
      <div>
        {isLoading ? (
          <p className="text-sm text-text-muted">
            {t("page.checkingYourPermissions")}
          </p>
        ) : sections.length === 0 ? (
          <p className="text-sm text-text-muted">
            {t("page.yourAccountHoldsNoAdministrativePermissions")}
          </p>
        ) : (
          <>
            {visible === "users" && <AdminUserList />}
            {visible === "content" && <AdminContentList />}
            {visible === "review" && <AdminReviewQueue />}
            {visible === "questions" && <AdminQuestionBank />}
            {visible === "reports" && <AdminReportedContentList />}
            {visible === "vocabulary" && <AdminVocabulary />}
            {visible === "flags" && <AdminFeatureFlags />}
            {visible === "ai" && <AdminAIUsage />}
            {visible === "moderation" && <AdminStudioModeration />}
            {visible === "payouts" && <AdminPayoutsList />}
          </>
        )}
      </div>
    </div>
  );
}
