import * as React from "react";

export interface ResponsiveDataColumn<T> { key: string; header: React.ReactNode; cell: (row: T) => React.ReactNode; }
export function ResponsiveDataList<T>({ rows, columns, renderCard, loading, empty, error, getRowKey }: {
  rows: T[]; columns: ResponsiveDataColumn<T>[]; renderCard: (row: T) => React.ReactNode;
  getRowKey: (row: T) => React.Key; loading?: React.ReactNode; empty?: React.ReactNode; error?: React.ReactNode;
}): React.JSX.Element {
  if (error) return <>{error}</>;
  if (loading) return <>{loading}</>;
  if (rows.length === 0) return <>{empty}</>;
  return <>
    <div className="space-y-3 md:hidden">{rows.map((row) => <React.Fragment key={getRowKey(row)}>{renderCard(row)}</React.Fragment>)}</div>
    <div className="hidden overflow-x-auto md:block"><table className="w-full border-collapse text-left text-sm"><thead><tr className="border-b border-border-subtle">{columns.map((column) => <th key={column.key} className="px-4 py-3 font-semibold text-text-muted">{column.header}</th>)}</tr></thead><tbody>{rows.map((row) => <tr key={getRowKey(row)} className="h-14 border-b border-border-subtle">{columns.map((column) => <td key={column.key} className="px-4 py-2">{column.cell(row)}</td>)}</tr>)}</tbody></table></div>
  </>;
}
