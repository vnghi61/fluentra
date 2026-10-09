import * as React from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import {
  SystemStatePage,
  systemStateActionClass,
} from "@/components/layout/SystemStatePage";

export function NotFoundPage(): React.JSX.Element {
  const router = useRouter();
  const { t } = useTranslation();

  return (
    <SystemStatePage
      variant="404"
      actions={
        <>
          <Link
            to="/"
            className={`${systemStateActionClass} bg-primary text-primary-fg hover:bg-primary-hover`}
          >
            {t("systemState.home", "Back to home")}
          </Link>
          <button
            type="button"
            onClick={() => void router.invalidate()}
            className={`${systemStateActionClass} border border-border bg-transparent text-text hover:bg-surface-muted`}
          >
            {t("systemState.retry", "Try again")}
          </button>
        </>
      }
    />
  );
}
