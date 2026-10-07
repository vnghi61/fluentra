import React from "react";
import { useParams } from "@tanstack/react-router";
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
  AdminPageHeader,
  AdminModuleSwitcher,
} from "@/features/admin";
import {
  type AdminSectionKey,
  useVisibleAdminSections,
} from "@/features/admin";

export function AdminPage(): React.JSX.Element {
  const { t } = useTranslation();
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
  const currentSection =
    sections.find((section) => section.key === visible) ?? sections[0];
  const currentLabel = currentSection
    ? t(currentSection.labelKey, currentSection.labelFallback)
    : "";

  return (
    <div className="mx-auto w-full space-y-5 py-2 md:space-y-6">
      {/* Header: the section is the page; "Administration" is only context. */}
      <div className="space-y-2"><p className="inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-primary-accent">
          <Shield className="h-3 w-3" aria-hidden="true" />
          {t("page.administration")}
        </p>{currentSection && <AdminPageHeader title={currentLabel} />}</div>

      {/* Mobile section picker for viewports below md */}
      {!isLoading && sections.length > 0 && (
        <AdminModuleSwitcher sections={sections} active={currentSection} />
      )}

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
