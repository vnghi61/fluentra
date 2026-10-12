import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { useSubmitDraft } from "@/features/studio/hooks/useStudio";
import { server } from "./msw-server";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useSubmitDraft", () => {
  const requestedIds: string[] = [];

  beforeEach(() => {
    requestedIds.length = 0;
    server.use(
      http.post<{ id: string }>(
        "/api/v1/studio/courses/:id/submit",
        ({ params }) => {
          const id = params.id;
          requestedIds.push(id);
          return HttpResponse.json({
            id: "0199a1c2-3d4e-7f80-9abc-def012345678",
            draft_id: id,
            version: 1,
            status: "submitted",
            submitted_by: "0199a1c2-3d4e-7f80-9abc-def012345601",
            submitted_at: "2026-09-20T09:00:00Z",
            created_at: "2026-09-20T09:00:00Z",
            updated_at: "2026-09-20T09:00:00Z",
          });
        },
      ),
    );
  });

  it("submits draft using mutation variable id", async () => {
    const { result } = renderHook(() => useSubmitDraft(), {
      wrapper: createWrapper(),
    });

    const res = await result.current.mutateAsync("draft-123");

    expect(requestedIds).toEqual(["draft-123"]);
    expect(res.draft_id).toBe("draft-123");
  });

  it("rejects with 'No draft id' and sends no request when no id is provided", async () => {
    const { result } = renderHook(() => useSubmitDraft(), {
      wrapper: createWrapper(),
    });

    await expect(result.current.mutateAsync(undefined)).rejects.toThrow(
      "No draft id",
    );
    expect(requestedIds).toHaveLength(0);
  });

  it("falls back to hook argument when mutation variable is omitted", async () => {
    const { result } = renderHook(() => useSubmitDraft("draft-fallback"), {
      wrapper: createWrapper(),
    });

    const res = await result.current.mutateAsync(undefined);

    expect(requestedIds).toEqual(["draft-fallback"]);
    expect(res.draft_id).toBe("draft-fallback");
  });

  it("prefers mutation variable over hook argument", async () => {
    const { result } = renderHook(() => useSubmitDraft("draft-fallback"), {
      wrapper: createWrapper(),
    });

    const res = await result.current.mutateAsync("draft-override");

    expect(requestedIds).toEqual(["draft-override"]);
    expect(res.draft_id).toBe("draft-override");
  });

  it("rejects with 'No draft id' when hook id is empty string and no mutation variable is passed", async () => {
    const { result } = renderHook(() => useSubmitDraft(""), {
      wrapper: createWrapper(),
    });

    await expect(result.current.mutateAsync(undefined)).rejects.toThrow(
      "No draft id",
    );
    expect(requestedIds).toHaveLength(0);
  });
});
