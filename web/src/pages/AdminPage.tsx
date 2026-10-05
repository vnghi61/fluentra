import React, { useState } from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, Shield } from "lucide-react";
import { BottomSheet } from "@/components/ui/bottom-sheet";
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
  const [pickerOpen, setPickerOpen] = useState(false);

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

      {/* Mobile section picker for viewports below md */}
      {!isLoading && sections.length > 0 && (
        <div className="md:hidden">
          <button
            type="button"
            onClick={() => setPickerOpen(true)}
            aria-expanded={pickerOpen}
            aria-haspopup="dialog"
            className="flex items-center justify-between w-full px-4 py-3 min-h-[44px] rounded-xl border border-border bg-surface-card text-sm font-medium text-text hover:bg-surface-muted transition-colors cursor-pointer"
          >
            <span className="flex items-center gap-2 truncate">
              {currentSection && (
                <currentSection.Icon
                  className="h-4 w-4 shrink-0 text-primary"
                  aria-hidden="true"
                />
              )}
              <span>
                {t("page.adminSectionPicker", "Section: {{section}}", {
                  section: currentLabel,
                })}
              </span>
            </span>
            <ChevronDown
              className="h-4 w-4 shrink-0 text-text-muted"
              aria-hidden="true"
            />
          </button>

          <BottomSheet
            open={pickerOpen}
            onOpenChange={setPickerOpen}
            title={t("page.adminSectionsTitle", "Admin Sections")}
          >
            <div className="flex flex-col gap-1 pb-4">
              {sections.map(({ key, path, labelKey, labelFallback, Icon }) => {
                const isActive = visible === key;
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={() => {
                      setPickerOpen(false);
                      void navigate({ to: path });
                    }}
                    aria-current={isActive ? "page" : undefined}
                    className={`flex items-center gap-3 w-full px-4 py-3 min-h-[44px] text-left text-sm rounded-xl transition-colors cursor-pointer ${
                      isActive
                        ? "bg-primary/10 text-primary-accent font-semibold"
                        : "text-text hover:bg-surface-muted font-medium"
                    }`}
                  >
                    <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
                    <span className="flex-1 truncate">
                      {t(labelKey, labelFallback)}
                    </span>
                    {isActive && (
                      <span
                        className="h-2 w-2 rounded-full bg-primary shrink-0"
                        aria-hidden="true"
                      />
                    )}
                  </button>
                );
              })}
            </div>
          </BottomSheet>
        </div>
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
