import { z } from "zod";

export const createContentSchema = z.object({
  slug: z.string().trim().min(1, "admin.contentFieldRequired"),
  kind: z.string().trim().min(1, "admin.contentFieldRequired"),
  cefr_level: z.enum(["A1", "A2", "B1", "B2", "C1", "C2"]),
  body: z.string().refine((value) => {
    try {
      const parsed: unknown = JSON.parse(value);
      return (
        typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
      );
    } catch {
      return false;
    }
  }, "admin.contentJsonObject"),
});

export type CreateContentFormValues = z.infer<typeof createContentSchema>;

export const adminActionReasonSchema = z.object({
  reason: z
    .string()
    .trim()
    .min(10, "validation.reasonMinLength")
    .max(500, "validation.reasonMaxLength"),
});

export type AdminActionReasonFormValues = z.infer<
  typeof adminActionReasonSchema
>;

export const createFeatureFlagSchema = z.object({
  key: z
    .string()
    .trim()
    .min(2, "validation.flagKeyMinLength")
    .regex(/^[a-z0-9_]+$/, "validation.flagKeyFormat"),
  description: z.string().trim().min(5, "validation.descriptionMinLength"),
  enabled: z.boolean().default(false),
  rollout_percent: z
    .number()
    .min(0, "validation.rolloutMin")
    .max(100, "validation.rolloutMax"),
  owner: z.string().trim().min(2, "validation.flagOwnerRequired"),
  expires_on: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, "validation.expiryFormat"),
});

export type CreateFeatureFlagFormValues = z.infer<
  typeof createFeatureFlagSchema
>;
