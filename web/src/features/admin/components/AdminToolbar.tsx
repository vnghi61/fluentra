import * as React from "react";

export function AdminToolbar({ search, filters, actions }: { search?: React.ReactNode; filters?: React.ReactNode; actions?: React.ReactNode }): React.JSX.Element {
  return <div className="flex flex-col gap-3 md:flex-row md:items-center">
    {search && <div className="min-w-0 flex-1">{search}</div>}
    {filters && <div className="flex flex-wrap items-center gap-2">{filters}</div>}
    {actions && <div className="flex items-center gap-2 md:ml-auto">{actions}</div>}
  </div>;
}
