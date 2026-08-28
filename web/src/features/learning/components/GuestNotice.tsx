import React from "react";
import { Link } from "@tanstack/react-router";
import { CloudOff } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * Says, before anything is invested, that nothing is being kept.
 *
 * A guest can browse the catalogue and work through a lesson, and the honest
 * thing is to say so up front rather than only in the dialog at the end. It is a
 * line of text and two links, not a wall: the point is that the visitor knows,
 * not that they are stopped.
 */
export const GuestNotice: React.FC = () => {
  const { t } = useTranslation();

  return (
    <div
      role="note"
      className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-3 rounded-xl border border-border-subtle bg-surface-muted px-4 py-3 text-sm text-text-muted"
    >
      <CloudOff className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="flex-1">
        {t(
          "guest.notice.text",
          "You are browsing without an account. You can open any lesson and answer its exercises, but nothing is saved.",
        )}
      </span>
      <span className="flex items-center gap-3 shrink-0">
        <Link
          to="/login"
          className="inline-flex items-center min-h-[44px] font-semibold text-primary-accent hover:underline"
        >
          {t("nav.signIn", "Sign in")}
        </Link>
        <Link
          to="/register"
          className="inline-flex items-center min-h-[44px] font-semibold text-primary-accent hover:underline"
        >
          {t("nav.createAccount", "Create account")}
        </Link>
      </span>
    </div>
  );
};
