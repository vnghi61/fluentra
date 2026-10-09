import React from "react";
import { useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
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
  AdminBanners,
  AdminSectionHeader,
  AdminModuleSwitcher,
} from "@/features/admin";
import {
  type AdminSectionKey,
  useVisibleAdminSections,
} from "@/features/admin";

// Sections that render their own header because it carries an action.
const SELF_HEADED: ReadonlySet<AdminSectionKey> = new Set([
  "content",
  "questions",
  "flags",
  "banners",
]);

const SECTION_DESCRIPTIONS: Partial<Record<AdminSectionKey, string>> = {
  users: "admin.userManagementDescription",
  review: "adminReview.pageDescription",
  reports: "adminReports.desc",
  vocabulary: "admin.vocabularyDescription",
  ai: "admin.aiUsageSubtitle",
  moderation: "adminModeration.subtitle",
  payouts: "adminPayouts.subtitle",
};

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
      {/* Header: the section is the page; "Administration" is only context.
        Sections whose header carries an action draw their own. */}
      {currentSection && visible && !SELF_HEADED.has(visible) && (
        <AdminSectionHeader
          title={currentLabel}
          {...(SECTION_DESCRIPTIONS[visible]
            ? { description: t(SECTION_DESCRIPTIONS[visible]) }
            : {})}
        />
      )}

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
          <p className="text-base leading-relaxed text-text-muted">
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
            {visible === "banners" && <AdminBanners />}
            {visible === "ai" && <AdminAIUsage />}
            {visible === "moderation" && <AdminStudioModeration />}
            {visible === "payouts" && <AdminPayoutsList />}
          </>
        )}
      </div>
    </div>
  );
}
