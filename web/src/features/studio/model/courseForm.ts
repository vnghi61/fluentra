import { z } from "zod";

import { CEFR_LEVELS, PRICE } from "./courseStructure";

/**
 * The course details form. Messages are i18n keys under `studio.editor.err`,
 * translated where they are shown.
 */
export const courseFormSchema = z
  .object({
    title: z.string().trim().min(1, "titleRequired").max(120, "titleTooLong"),
    slug: z
      .string()
      .trim()
      .min(3, "slugShort")
      .max(80, "slugLong")
      .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, "slugFormat"),
    description: z.string().max(2000, "descTooLong"),
    cefrLevel: z.enum(CEFR_LEVELS),
    isFree: z.boolean(),
    priceVnd: z.number(),
  })
  .superRefine((v, ctx) => {
    if (v.isFree) return;
    if (
      !Number.isInteger(v.priceVnd) ||
      v.priceVnd < PRICE.min ||
      v.priceVnd > PRICE.max
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["priceVnd"],
        message: "priceBounds",
      });
    }
  });

export type CourseFormValues = z.infer<typeof courseFormSchema>;

export const defaultCourseForm: CourseFormValues = {
  title: "",
  slug: "",
  description: "",
  cefrLevel: "B1",
  isFree: true,
  priceVnd: 49_000,
};
