import { z } from "zod";

export const profileFormSchema = z.object({
  display_name: z
    .string()
    .min(1, "Display name cannot be empty")
    .max(100, "Display name is too long"),
  country: z
    .string()
    .max(2, "Country must be a 2-letter ISO code")
    .optional()
    .or(z.literal("")),
  timezone: z.string().min(1, "Timezone is required"),
  date_of_birth: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, "Format must be YYYY-MM-DD")
    .optional()
    .or(z.literal("")),
});

export type ProfileFormValues = z.infer<typeof profileFormSchema>;

export const preferencesFormSchema = z.object({
  locale: z.string().min(2, "Locale is required"),
  theme: z.enum(["light", "dark", "system"]),
  daily_goal_minutes: z
    .number()
    .min(5, "Daily goal must be at least 5 minutes")
    .max(180, "Daily goal cannot exceed 180 minutes"),
  notification_channels: z
    .array(z.enum(["in_app", "email", "push"]))
    .default([]),
  quiet_hours_enabled: z.boolean().default(false),
  quiet_hours_start: z.string().default("22:00"),
  quiet_hours_end: z.string().default("07:00"),
  ai_processing_opt_out: z.boolean().default(false),
});

export type PreferencesFormValues = z.infer<typeof preferencesFormSchema>;

export const changePasswordSchema = z
  .object({
    current_password: z
      .string()
      .min(8, "Current password must be at least 8 characters"),
    new_password: z
      .string()
      .min(8, "New password must be at least 8 characters"),
    confirm_new_password: z
      .string()
      .min(8, "Confirmation must be at least 8 characters"),
  })
  .refine((data) => data.new_password === data.confirm_new_password, {
    message: "Passwords do not match",
    path: ["confirm_new_password"],
  });

export type ChangePasswordFormValues = z.infer<typeof changePasswordSchema>;
