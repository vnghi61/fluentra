import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";

import i18n, { initI18n, loadLocale } from "@/i18n";
import { BannerCard, type BannerCardData } from "@/features/banners";

const base: BannerCardData = {
  title: "IELTS mock week",
  body: "Free until Sunday.",
  cta_label: null,
  cta_url: null,
  tone: "promo",
  dismissible: false,
};

function renderCard(banner: BannerCardData) {
  return render(
    <I18nextProvider i18n={i18n}>
      <BannerCard banner={banner} />
    </I18nextProvider>,
  );
}

describe("BannerCard image", () => {
  beforeEach(async () => {
    await initI18n("en");
    await loadLocale("en");
    await i18n.changeLanguage("en");
  });

  it("reserves the picture's size and describes it", () => {
    renderCard({
      ...base,
      image_url: "https://storage.test/signed/banner.jpg",
      image_alt: "Learners in a mock test",
      image_width: 1600,
      image_height: 900,
    });

    const img = screen.getByRole("img", { name: "Learners in a mock test" });
    expect(img).toHaveAttribute("width", "1600");
    expect(img).toHaveAttribute("height", "900");
    expect(img.parentElement?.style.aspectRatio).toBe("1600 / 900");
  });

  it("drops a picture that fails to load and keeps the text", () => {
    const { container } = renderCard({
      ...base,
      image_url: "https://storage.test/signed/expired.jpg",
      image_alt: "",
      image_width: 800,
      image_height: 400,
    });

    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    fireEvent.error(img!);
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByText("IELTS mock week")).toBeInTheDocument();
  });

  it("renders a text-only banner without an image", () => {
    const { container } = renderCard(base);
    expect(container.querySelector("img")).toBeNull();
  });
});
