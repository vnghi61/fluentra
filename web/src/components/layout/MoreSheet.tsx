import * as React from "react";
import { Link } from "@tanstack/react-router";
import {
  ChevronRight,
  GraduationCap,
  Settings,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { BottomSheet } from "@/components/ui/bottom-sheet";

export interface MoreSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  signedIn: boolean;
  isAdmin: boolean;
}

/**
 * Lazy-loaded secondary destinations sheet for mobile navigation.
 *
 * It is only fetched and mounted when the user taps "More" on the bottom navigation
 * bar, ensuring the full bottom sheet and motion physics code stay out of the
 * initial entry-chunk bundle.
 */
export default function MoreSheet({
  open,
  onOpenChange,
  signedIn,
  isAdmin,
}: MoreSheetProps): React.JSX.Element {
  const { t } = useTranslation();

  const handleClose = () => {
    onOpenChange(false);
  };

  return (
    <BottomSheet
      open={open}
      onOpenChange={onOpenChange}
      title={t("nav.more", "More")}
    >
      <div className="flex flex-col gap-1 pb-4">
        {/* Exams */}
        <Link
          to="/exams"
          onClick={handleClose}
          className="flex min-h-[44px] items-center gap-3.5 rounded-xl p-3 transition-colors hover:bg-surface-muted"
        >
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
            <GraduationCap className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-sm font-medium text-text">
              {t("nav.exams", "Exams")}
            </div>
            <div className="truncate text-xs text-text-muted">
              {t("nav.moreDescriptions.exams", "Practice mock exams and tests")}
            </div>
          </div>
          <ChevronRight
            className="h-5 w-5 shrink-0 text-text-muted"
            aria-hidden="true"
          />
        </Link>

        {/* Studio */}
        <Link
          to="/studio"
          onClick={handleClose}
          className="flex min-h-[44px] items-center gap-3.5 rounded-xl p-3 transition-colors hover:bg-surface-muted"
        >
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
            <Sparkles className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-sm font-medium text-text">
              {t("nav.studio", "Studio")}
            </div>
            <div className="truncate text-xs text-text-muted">
              {t(
                "nav.moreDescriptions.studio",
                "Author and manage learning content",
              )}
            </div>
          </div>
          <ChevronRight
            className="h-5 w-5 shrink-0 text-text-muted"
            aria-hidden="true"
          />
        </Link>

        {/* Settings - only when signedIn */}
        {signedIn && (
          <Link
            to="/settings"
            onClick={handleClose}
            className="flex min-h-[44px] items-center gap-3.5 rounded-xl p-3 transition-colors hover:bg-surface-muted"
          >
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
              <Settings className="h-5 w-5" aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-text">
                {t("nav.settings", "Settings")}
              </div>
              <div className="truncate text-xs text-text-muted">
                {t(
                  "nav.moreDescriptions.settings",
                  "Account preferences and theme",
                )}
              </div>
            </div>
            <ChevronRight
              className="h-5 w-5 shrink-0 text-text-muted"
              aria-hidden="true"
            />
          </Link>
        )}

        {/* Admin - only when signedIn and isAdmin */}
        {signedIn && isAdmin && (
          <Link
            to="/admin"
            onClick={handleClose}
            className="flex min-h-[44px] items-center gap-3.5 rounded-xl p-3 transition-colors hover:bg-surface-muted"
          >
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
              <ShieldCheck className="h-5 w-5" aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-text">
                {t("nav.admin", "Admin")}
              </div>
              <div className="truncate text-xs text-text-muted">
                {t(
                  "nav.moreDescriptions.admin",
                  "Platform administration and analytics",
                )}
              </div>
            </div>
            <ChevronRight
              className="h-5 w-5 shrink-0 text-text-muted"
              aria-hidden="true"
            />
          </Link>
        )}
      </div>
    </BottomSheet>
  );
}
