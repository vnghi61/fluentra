import * as React from "react";

export function AdminPageHeader({ title, description, count, actions }: { title: string; description?: string; count?: number; actions?: React.ReactNode }): React.JSX.Element {
  return <header className="flex flex-wrap items-start justify-between gap-4">
    <div className="min-w-0">
      <div className="flex flex-wrap items-center gap-2"><h1 className="text-2xl font-bold text-text md:text-3xl">{title}</h1>{count !== undefined && <span className="rounded-full bg-surface-muted px-2.5 py-1 text-sm text-text-muted">{count}</span>}</div>
      {description && <p className="mt-1 text-sm text-text-muted">{description}</p>}
    </div>
    {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
  </header>;
}
