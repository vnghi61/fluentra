import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";
import { SheetDialog } from "@/components/ui/sheet";
import type { AdminSection } from "../model/sections";

export function AdminModuleSwitcher({
  sections,
  active,
}: {
  sections: AdminSection[];
  active?: AdminSection | undefined;
}): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [open, setOpen] = React.useState(false);
  const activeLabel = active ? t(active.labelKey, active.labelFallback) : "";
  return (
    <div className="md:hidden">
      <button
        type="button"
        className="flex min-h-[44px] w-full items-center justify-between rounded-xl border border-border bg-surface-card px-4 text-sm font-medium text-text"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
      >
        <span className="min-w-0 truncate">
          {t("page.adminSectionPicker", "Section: {{section}}", {
            section: activeLabel,
          })}
        </span>
        <ChevronDown className="h-4 w-4 shrink-0" aria-hidden="true" />
      </button>
      <SheetDialog
        open={open}
        onOpenChange={setOpen}
        title={t("page.adminSectionsTitle", "Admin Sections")}
      >
        <div className="flex flex-col gap-1 pb-4">
          {sections.map((section) => {
            const selected = section.key === active?.key;
            const Icon = section.Icon;
            return (
              <button
                key={section.key}
                type="button"
                aria-current={selected ? "page" : undefined}
                className="flex min-h-11 items-center gap-3 rounded-xl px-4 text-left text-sm hover:bg-surface-muted"
                onClick={() => {
                  setOpen(false);
                  void navigate({ to: section.path });
                }}
              >
                <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
                {t(section.labelKey, section.labelFallback)}
              </button>
            );
          })}
        </div>
      </SheetDialog>
    </div>
  );
}
