import { z } from "zod";

export const adminActionReasonSchema = z.object({
  reason: z
    .string()
    .trim()
    .min(10, "Reason must be at least 10 characters long")
    .max(500, "Reason cannot exceed 500 characters"),
});

export type AdminActionReasonFormValues = z.infer<typeof adminActionReasonSchema>;

export const createFeatureFlagSchema = z.object({
  key: z
    .string()
    .trim()
    .min(2, "Flag key must be at least 2 characters")
    .regex(/^[a-z0-9_]+$/, "Flag key must be lowercase alphanumeric with underscores"),
  description: z
    .string()
    .trim()
    .min(5, "Description must be at least 5 characters"),
  enabled: z.boolean().default(false),
  rollout_percent: z
    .number()
    .min(0, "Rollout cannot be negative")
    .max(100, "Rollout cannot exceed 100%"),
  owner: z
    .string()
    .trim()
    .min(2, "Owner is required (e.g. @team or email)"),
  expires_on: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, "Expiry must be in YYYY-MM-DD format"),
});

export type CreateFeatureFlagFormValues = z.infer<typeof createFeatureFlagSchema>;
