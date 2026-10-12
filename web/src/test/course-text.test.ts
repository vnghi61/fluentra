import i18n from "i18next";
import { describe, expect, it } from "vitest";

import en from "@/i18n/en.json";
import vi from "@/i18n/vi.json";
import {
  courseDescription,
  courseTitle,
} from "@/features/lesson/model/courseText";

async function translator(lng: "en" | "vi") {
  const instance = i18n.createInstance();
  await instance.init({
    lng,
    resources: { en: { translation: en }, vi: { translation: vi } },
  });
  return instance.t;
}

describe("courseTitle", () => {
  it("shows a Foundation course in the reader's language", async () => {
    const course = {
      slug: "english-tenses",
      title: "English Tenses",
      description: "x",
    };
    expect(courseTitle(await translator("vi"), course)).toBe(
      "Các thì tiếng Anh",
    );
    expect(courseTitle(await translator("en"), course)).toBe("English Tenses");
    expect(courseDescription(await translator("vi"), course)).toContain(
      "Mười hai thì",
    );
  });

  it("keeps the authored title of a course with no translation", async () => {
    const course = { slug: "my-own-course", title: "My own course" };
    expect(courseTitle(await translator("vi"), course)).toBe("My own course");
  });
});
