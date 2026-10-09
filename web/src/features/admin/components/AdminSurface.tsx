import * as React from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw, Shield, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { AdminPageHeader } from "./AdminPageHeader";

/**
 * The building blocks of an admin section, lifted from the content library so
 * every section reads the same way: a header, a filter card, an optional row of
 * summary tiles, and one data panel that holds the table, the mobile cards and
 * the pager. Presentation only — data and actions stay in each section.
 */

export const adminTable = {
  table: "w-full table-fixed border-collapse text-left text-sm",
  head: "border-b border-border-subtle bg-surface-muted/60 text-xs font-semibold text-text-muted",
  th: "px-4 py-3",
  body: "divide-y divide-border-subtle",
  row: "transition-colors hover:bg-surface-muted/40",
  td: "px-4 py-2",
} as const;

export function AdminSectionHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: React.ReactNode;
}): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <header className="space-y-2">
      <p className="inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-primary-accent">
        <Shield aria-hidden="true" className="h-4 w-4" />
        {t("page.administration")}
      </p>
      <AdminPageHeader
        title={title}
        {...(description ? { description } : {})}
        actions={actions}
      />
    </header>
  );
}

export function AdminFilterCard({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}): React.JSX.Element {
  return (
    <div
      className={cn(
        "space-y-3 rounded-2xl border border-border-subtle bg-surface-card p-4",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function AdminDataPanel({
  children,
  className,
  ...rest
}: React.HTMLAttributes<HTMLDivElement>): React.JSX.Element {
  return (
    <div
      {...rest}
      className={cn(
        "overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-sm",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function AdminRefreshButton({
  onClick,
  busy,
}: {
  onClick: () => void;
  busy: boolean;
}): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <Button
      type="button"
      variant="outline"
      size="icon"
      onClick={onClick}
      disabled={busy}
      aria-label={t("admin.refresh")}
      className="shrink-0"
    >
      <RefreshCw
        aria-hidden="true"
        className={cn(
          "h-4 w-4",
          busy && "animate-spin motion-reduce:animate-none",
        )}
      />
    </Button>
  );
}

export function AdminActiveFilters({
  children,
  onReset,
  ...rest
}: {
  children: React.ReactNode;
  onReset: () => void;
} & React.HTMLAttributes<HTMLDivElement>): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <div {...rest} className="flex flex-wrap items-center gap-2 text-xs">
      <span className="text-text-muted">{t("admin.activeFilters")}</span>
      {children}
      <Button type="button" variant="ghost" size="sm" onClick={onReset}>
        <X aria-hidden="true" className="h-4 w-4" />
        {t("admin.resetFilters")}
      </Button>
    </div>
  );
}

export function AdminFilterChip({
  children,
}: {
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <span className="max-w-full break-all rounded-lg bg-surface-muted px-2.5 py-1.5 text-text">
      {children}
    </span>
  );
}

export interface AdminStat {
  key: string;
  label: string;
  value: React.ReactNode;
  Icon: React.ElementType;
  tone: string;
}

export const adminStatTones = {
  primary: "bg-primary/10 text-primary-accent",
  success: "bg-success/10 text-success-accent",
  warning: "bg-warning/10 text-warning-accent",
  danger: "bg-danger/10 text-danger-accent",
  neutral: "bg-surface-muted text-text-muted",
} as const;

export function AdminStatGrid({
  stats,
  loading = false,
  label,
}: {
  stats: AdminStat[];
  loading?: boolean;
  label: string;
}): React.JSX.Element {
  return (
    <section
      aria-label={label}
      className={cn(
        "grid grid-cols-2 gap-3",
        stats.length === 3 ? "lg:grid-cols-3" : "lg:grid-cols-4",
      )}
    >
      {stats.map(({ key, label: statLabel, value, Icon, tone }) => (
        <div
          key={key}
          className="flex min-w-0 flex-col gap-3 rounded-2xl border border-border-subtle bg-surface-card p-4 sm:flex-row sm:items-center sm:gap-4 sm:p-5"
        >
          <span
            className={cn(
              "flex h-10 w-10 shrink-0 items-center justify-center rounded-full",
              tone,
            )}
          >
            <Icon aria-hidden="true" className="h-5 w-5" />
          </span>
          <div className="min-w-0 flex-1">
            {loading ? (
              <Skeleton className="mb-1 h-8 w-16" />
            ) : (
              <p
                data-testid={`admin-stat-${key}`}
                className="truncate text-2xl font-bold tabular-nums text-text"
              >
                {value}
              </p>
            )}
            <p className="mt-0.5 text-sm text-text-muted">{statLabel}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

export function AdminListSkeleton({
  label,
  rows = 5,
  ...rest
}: {
  label: string;
  rows?: number;
} & React.HTMLAttributes<HTMLDivElement>): React.JSX.Element {
  return (
    <div
      role="status"
      aria-label={label}
      aria-busy="true"
      className="divide-y divide-border-subtle"
      {...rest}
    >
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className="flex h-20 items-center gap-3 px-4">
          <Skeleton className="h-9 w-9 shrink-0 rounded-full" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-3.5 w-1/2" />
            <Skeleton className="h-3 w-1/3" />
          </div>
          <Skeleton className="hidden h-6 w-24 rounded-full md:block" />
        </div>
      ))}
    </div>
  );
}

export function AdminRowIcon({
  icon: Icon,
  className,
}: {
  icon: React.ElementType;
  className?: string;
}): React.JSX.Element {
  return (
    <span
      className={cn(
        "flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/5 text-primary-accent",
        className,
      )}
    >
      <Icon aria-hidden="true" className="h-4 w-4" />
    </span>
  );
}

export function AdminTag({
  children,
  title,
}: {
  children: React.ReactNode;
  title?: string;
}): React.JSX.Element {
  return (
    <span
      title={title}
      className="inline-block max-w-full break-all rounded-full bg-primary/5 px-2.5 py-1 text-xs font-medium text-text-muted"
    >
      {children}
    </span>
  );
}

export function AdminNotice({
  tone,
  children,
}: {
  tone: "danger" | "success" | "warning";
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <div
      role={tone === "success" ? "status" : "alert"}
      className={cn(
        "flex flex-wrap items-center justify-between gap-2 rounded-xl border px-4 py-2.5 text-sm",
        tone === "danger" && "border-danger/20 bg-danger/5 text-danger-accent",
        tone === "success" &&
          "border-success/20 bg-success/5 text-success-accent",
        tone === "warning" && "border-warning/20 bg-warning/5 text-text-muted",
      )}
    >
      {children}
    </div>
  );
}

export function AdminSubTabs<K extends string>({
  tabs,
  active,
  onChange,
  label,
}: {
  tabs: readonly {
    key: K;
    label: string;
    Icon?: React.ElementType;
    count?: number;
  }[];
  active: K;
  onChange: (key: K) => void;
  label: string;
}): React.JSX.Element {
  return (
    <div
      role="tablist"
      aria-label={label}
      className="inline-flex max-w-full gap-1 overflow-x-auto rounded-xl border border-border-subtle bg-surface-card p-1"
    >
      {tabs.map(({ key, label: tabLabel, Icon, count }) => (
        <button
          key={key}
          type="button"
          role="tab"
          aria-selected={active === key}
          onClick={() => onChange(key)}
          className={cn(
            "inline-flex min-h-11 items-center gap-2 whitespace-nowrap rounded-lg px-4 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
            active === key
              ? "bg-primary/10 text-primary-accent"
              : "text-text-muted hover:bg-surface-muted hover:text-text",
          )}
        >
          {Icon && <Icon aria-hidden="true" className="h-4 w-4" />}
          {tabLabel}
          {count !== undefined && count > 0 && (
            <span className="rounded-full bg-surface-muted px-1.5 py-0.5 text-xs tabular-nums text-text-muted">
              {count}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}
