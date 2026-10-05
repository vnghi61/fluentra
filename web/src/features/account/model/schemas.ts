import { z } from "zod";

export const profileFormSchema = z.object({
  display_name: z
    .string()
    .min(1, "validation.displayNameRequired")
    .max(100, "validation.displayNameTooLong"),
  country: z
    .string()
    .max(2, "validation.countryCodeLength")
    .optional()
    .or(z.literal("")),
  timezone: z.string().min(1, "validation.timezoneRequired"),
  date_of_birth: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, "validation.dateFormat")
    .optional()
    .or(z.literal("")),
});

export type ProfileFormValues = z.infer<typeof profileFormSchema>;

export const preferencesFormSchema = z.object({
  locale: z.string().min(2, "validation.localeRequired"),
  theme: z.enum(["light", "dark", "system"]),
  daily_goal_minutes: z
    .number()
    .min(5, "validation.dailyGoalMin")
    .max(180, "validation.dailyGoalMax"),
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
    current_password: z.string().min(8, "validation.currentPasswordMinLength"),
    new_password: z.string().min(8, "validation.newPasswordMin8"),
    confirm_new_password: z
      .string()
      .min(8, "validation.confirmPasswordMinLength"),
  })
  .refine((data) => data.new_password === data.confirm_new_password, {
    message: "validation.passwordsDoNotMatch",
    path: ["confirm_new_password"],
  });

export type ChangePasswordFormValues = z.infer<typeof changePasswordSchema>;
