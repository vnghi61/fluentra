import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEventDefault from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { AdminBanners } from "@/features/admin";
import type { components } from "@/types/api";
import { server } from "./msw-server";

type Banner = components["schemas"]["Banner"];
type BannerRequest = components["schemas"]["BannerRequest"];

const DAY = 24 * 60 * 60 * 1000;

function banner(overrides: Partial<Banner>): Banner {
  return {
    id: "0199c3e4-5f60-7a82-9cde-f01234567890",
    kind: "announcement",
    title: "IELTS mock week",
    body: "Free until Sunday.",
    cta_label: "Start",
    cta_url: "/exams",
    tone: "promo",
    is_active: true,
    priority: 10,
    dismissible: true,
    starts_at: null,
    ends_at: null,
    created_by: "0199a1c2-3d4e-7f80-9abc-def012345678",
    created_at: "2026-10-01T09:15:00Z",
    updated_at: "2026-10-01T09:15:00Z",
    ...overrides,
  };
}

function renderAdminBanners() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AdminBanners />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

describe("AdminBanners", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("lists banners with a status computed from the switch and the window", async () => {
    const now = Date.now();
    server.use(
      http.get("/api/v1/admin/banners", () =>
        HttpResponse.json({
          items: [
            banner({
              id: "0199c3e4-5f60-7a82-9cde-f01234567801",
              title: "Live now",
            }),
            banner({
              id: "0199c3e4-5f60-7a82-9cde-f01234567802",
              title: "Next week",
              starts_at: new Date(now + 7 * DAY).toISOString(),
            }),
            banner({
              id: "0199c3e4-5f60-7a82-9cde-f01234567803",
              title: "Last month",
              ends_at: new Date(now - 30 * DAY).toISOString(),
            }),
            banner({
              id: "0199c3e4-5f60-7a82-9cde-f01234567804",
              title: "Switched off",
              is_active: false,
            }),
          ],
        } satisfies components["schemas"]["BannerList"]),
      ),
    );

    renderAdminBanners();

    const table = await screen.findByRole("table", {
      name: "Dashboard banners",
    });
    const rowFor = (title: string) => {
      const cell = within(table).getByText(title);
      const row = cell.closest("tr");
      if (!row) throw new Error(`no row for ${title}`);
      return within(row);
    };
    expect(rowFor("Live now").getByText("Active")).toBeInTheDocument();
    expect(rowFor("Next week").getByText("Scheduled")).toBeInTheDocument();
    expect(rowFor("Last month").getByText("Expired")).toBeInTheDocument();
    expect(rowFor("Switched off").getByText("Off")).toBeInTheDocument();
    expect(
      rowFor("Live now").getByText("Announcement (pop-up) · Promotion"),
    ).toBeInTheDocument();
  });

  it("shows the empty state with a create action", async () => {
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
    );
    renderAdminBanners();

    expect(await screen.findByText("No banners yet")).toBeInTheDocument();
  });

  it("refuses an unsafe link in the form and never sends it", async () => {
    const user = userEventDefault.setup();
    let posted = 0;
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
      http.post("/api/v1/admin/banners", () => {
        posted += 1;
        return HttpResponse.json(banner({}), { status: 201 });
      }),
    );
    renderAdminBanners();

    await user.click(
      (await screen.findAllByRole("button", { name: "Create banner" }))[0]!,
    );
    const dialog = await screen.findByRole("dialog", { name: "Create banner" });

    await user.type(within(dialog).getByLabelText("Title"), "Spring offer");
    await user.type(within(dialog).getByLabelText("Button text"), "Open");
    await user.type(
      within(dialog).getByLabelText("Button link"),
      "javascript:alert(1)",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );

    expect(
      await within(dialog).findByText(
        "Use an internal path starting with / or an https:// link.",
      ),
    ).toBeInTheDocument();
    expect(posted).toBe(0);

    // A protocol-relative link is refused the same way.
    const link = within(dialog).getByLabelText("Button link");
    await user.clear(link);
    await user.type(link, "//evil.example");
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );
    expect(
      await within(dialog).findByText(
        "Use an internal path starting with / or an https:// link.",
      ),
    ).toBeInTheDocument();
    expect(posted).toBe(0);
  });

  it("creates a banner from a valid form and shows the live preview", async () => {
    const user = userEventDefault.setup();
    let body: BannerRequest | null = null;
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
      http.post("/api/v1/admin/banners", async ({ request }) => {
        body = (await request.json()) as BannerRequest;
        return HttpResponse.json(banner({ title: body.title }), {
          status: 201,
        });
      }),
    );
    renderAdminBanners();

    await user.click(
      (await screen.findAllByRole("button", { name: "Create banner" }))[0]!,
    );
    const dialog = await screen.findByRole("dialog", { name: "Create banner" });

    await user.type(within(dialog).getByLabelText("Title"), "Spring offer");
    // The preview follows the form as it is typed.
    const preview = within(dialog).getByRole("complementary", {
      name: "Preview",
    });
    expect(within(preview).getByText("Spring offer")).toBeInTheDocument();

    await user.type(within(dialog).getByLabelText("Button text"), "Open");
    await user.type(within(dialog).getByLabelText("Button link"), "/exams");
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );

    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({
      title: "Spring offer",
      cta_label: "Open",
      cta_url: "/exams",
      tone: "info",
      is_active: false,
      dismissible: true,
      starts_at: null,
      ends_at: null,
    });
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Create banner" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("uploads an image, previews it and saves its key with the alt text", async () => {
    const user = userEventDefault.setup();
    const storageUrl = "https://storage.test/fluentra-media/upload";
    const imageKey = "banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg";
    const steps: string[] = [];
    let body: BannerRequest | null = null;
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
      http.post(
        "/api/v1/admin/banners/image-upload-intent",
        async ({ request }) => {
          const req = (await request.json()) as { content_type: string };
          steps.push(`intent:${req.content_type}`);
          return HttpResponse.json({
            upload_url: storageUrl,
            method: "PUT",
            object_key: "banners/raw/admin/raw-1",
            expires_at: "2026-10-08T09:25:00Z",
            max_bytes: 5242880,
            content_type: req.content_type,
          });
        },
      ),
      http.put(storageUrl, () => {
        steps.push("upload");
        return new HttpResponse(null, { status: 200 });
      }),
      http.post("/api/v1/admin/banners/images", async ({ request }) => {
        const req = (await request.json()) as { object_key: string };
        steps.push(`process:${req.object_key}`);
        return HttpResponse.json(
          {
            image_key: imageKey,
            image_url: "https://storage.test/signed/banner.jpg",
            width: 1600,
            height: 900,
          },
          { status: 201 },
        );
      }),
      http.post("/api/v1/admin/banners", async ({ request }) => {
        body = (await request.json()) as BannerRequest;
        return HttpResponse.json(banner({ title: body.title }), {
          status: 201,
        });
      }),
    );
    renderAdminBanners();

    await user.click(
      (await screen.findAllByRole("button", { name: "Create banner" }))[0]!,
    );
    const dialog = await screen.findByRole("dialog", { name: "Create banner" });
    await user.type(within(dialog).getByLabelText("Title"), "Mock week");

    const input = document.getElementById("banner-image") as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        files: [new File(["png"], "hero.png", { type: "image/png" })],
      },
    });

    // The form preview and the field both show the processed picture.
    const preview = within(dialog).getByRole("complementary", {
      name: "Preview",
    });
    await waitFor(() =>
      expect(preview.querySelector("img")?.getAttribute("src")).toBe(
        "https://storage.test/signed/banner.jpg",
      ),
    );
    expect(steps).toEqual([
      "intent:image/png",
      "upload",
      "process:banners/raw/admin/raw-1",
    ]);

    await user.type(
      within(dialog).getByLabelText("Image description"),
      "Learners in a mock test",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );

    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({
      image_key: imageKey,
      image_alt: "Learners in a mock test",
    });
  });

  it("refuses an unsupported image type without uploading anything", async () => {
    const user = userEventDefault.setup();
    let intents = 0;
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
      http.post("/api/v1/admin/banners/image-upload-intent", () => {
        intents += 1;
        return HttpResponse.json({}, { status: 500 });
      }),
    );
    renderAdminBanners();

    await user.click(
      (await screen.findAllByRole("button", { name: "Create banner" }))[0]!,
    );
    const dialog = await screen.findByRole("dialog", { name: "Create banner" });
    const input = document.getElementById("banner-image") as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        files: [new File(["gif"], "anim.gif", { type: "image/gif" })],
      },
    });

    expect(
      await within(dialog).findByText(
        "Only JPEG, PNG or WebP images are accepted.",
      ),
    ).toBeInTheDocument();
    expect(intents).toBe(0);
  });

  it("keeps the image when the active switch is flipped", async () => {
    const user = userEventDefault.setup();
    const imageKey = "banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg";
    let body: BannerRequest | null = null;
    const withImage = banner({
      title: "Pictured",
      image_key: imageKey,
      image_url: "https://storage.test/signed/banner.jpg",
      image_alt: "A class",
      image_width: 1600,
      image_height: 900,
    });
    server.use(
      http.get("/api/v1/admin/banners", () =>
        HttpResponse.json({ items: [withImage] }),
      ),
      http.put("/api/v1/admin/banners/:id", async ({ request }) => {
        body = (await request.json()) as BannerRequest;
        return HttpResponse.json({ ...withImage, is_active: body.is_active });
      }),
    );
    renderAdminBanners();

    const table = await screen.findByRole("table", {
      name: "Dashboard banners",
    });
    await user.click(
      within(table).getByRole("switch", { name: "Turn on banner: Pictured" }),
    );

    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({
      is_active: false,
      image_key: imageKey,
      image_alt: "A class",
    });
  });

  it("creates a promo slide: image required, no text, button or dismissal", async () => {
    const user = userEventDefault.setup();
    const storageUrl = "https://storage.test/fluentra-media/slide-upload";
    const imageKey = "banners/images/0199c3e4-5f60-7a82-9cde-f01234567898.jpg";
    let body: BannerRequest | null = null;
    server.use(
      http.get("/api/v1/admin/banners", () => HttpResponse.json({ items: [] })),
      http.post("/api/v1/admin/banners/image-upload-intent", () =>
        HttpResponse.json({
          upload_url: storageUrl,
          method: "PUT",
          object_key: "banners/raw/admin/raw-2",
          expires_at: "2026-10-08T09:25:00Z",
          max_bytes: 5242880,
          content_type: "image/png",
        }),
      ),
      http.put(storageUrl, () => new HttpResponse(null, { status: 200 })),
      http.post("/api/v1/admin/banners/images", () =>
        HttpResponse.json(
          {
            image_key: imageKey,
            image_url: "https://storage.test/signed/slide.jpg",
            width: 1500,
            height: 500,
          },
          { status: 201 },
        ),
      ),
      http.post("/api/v1/admin/banners", async ({ request }) => {
        body = (await request.json()) as BannerRequest;
        return HttpResponse.json(banner({ kind: "slide", title: body.title }), {
          status: 201,
        });
      }),
    );
    renderAdminBanners();

    await user.click(
      (await screen.findAllByRole("button", { name: "Create banner" }))[0]!,
    );
    const dialog = await screen.findByRole("dialog", { name: "Create banner" });
    await user.click(
      within(dialog).getByRole("radio", { name: /Promo slide/ }),
    );

    // A slide has no body, button text, tone or dismissal to fill in.
    expect(within(dialog).queryByLabelText("Body")).toBeNull();
    expect(within(dialog).queryByLabelText("Button text")).toBeNull();

    await user.type(within(dialog).getByLabelText("Slide name"), "Autumn sale");
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );
    expect(
      await within(dialog).findByText("A slide needs an image."),
    ).toBeInTheDocument();
    expect(body).toBeNull();

    const input = document.getElementById("banner-image") as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        files: [new File(["png"], "sale.png", { type: "image/png" })],
      },
    });
    await waitFor(() =>
      expect(
        within(dialog)
          .getByRole("complementary", { name: "Preview" })
          .querySelector("img")
          ?.getAttribute("src"),
      ).toBe("https://storage.test/signed/slide.jpg"),
    );
    await user.type(
      within(dialog).getByLabelText(/Link when the image is tapped/),
      "/pricing",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Create banner" }),
    );

    await waitFor(() => expect(body).not.toBeNull());
    expect(body).toMatchObject({
      kind: "slide",
      title: "Autumn sale",
      body: "",
      cta_label: null,
      cta_url: "/pricing",
      dismissible: false,
      image_key: imageKey,
    });
  });
});
