import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AdminPageHeader } from "@/features/admin/components/AdminPageHeader";
import { ResponsiveDataList } from "@/features/admin/components/ResponsiveDataList";
import { StatusBadge } from "@/features/admin/components/StatusBadge";

describe("admin shared frame components", () => {
  it("uses a module title as the page heading and renders status text", () => {
    render(<><AdminPageHeader title="Learners" /><StatusBadge tone="success">Active</StatusBadge></>);
    expect(screen.getByRole("heading", { name: "Learners" })).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("renders table and mobile card list representations", () => {
    render(<ResponsiveDataList rows={[{ id: "one", name: "Example" }]} getRowKey={(row) => row.id} columns={[{ key: "name", header: "Name", cell: (row) => row.name }]} renderCard={(row) => <article>{row.name}</article>} />);
    const table = screen.getByRole("table");
    expect(table).toBeInTheDocument();
    expect(within(table).getByText("Example")).toBeInTheDocument();
    expect(screen.getAllByText("Example")).toHaveLength(2);
    expect(screen.getByRole("article")).toHaveTextContent("Example");
  });
});
