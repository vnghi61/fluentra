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
});

// Notification channels, quiet hours and the AI-processing opt-out are no
// longer edited in the interface (AI grading is always on). They are not on the
// form, and a save sends back whatever the server stored — see `replacementFor`.

export type PreferencesFormValues = z.infer<typeof preferencesFormSchema>;

/** The "Learning preferences" card: daily goal and daily practice level. */
export const learningPreferencesSchema = z.object({
  daily_goal_minutes: z
    .number()
    .min(5, "validation.dailyGoalMin")
    .max(180, "validation.dailyGoalMax"),
  // "" is "not chosen yet", sent as null.
  practice_level: z.enum(["", "A2", "B1", "B2"]),
});

export type LearningPreferencesValues = z.infer<
  typeof learningPreferencesSchema
>;

/** The "Appearance" card: display mode and interface language. */
export const appearanceSchema = preferencesFormSchema.pick({
  locale: true,
  theme: true,
});

export type AppearanceValues = z.infer<typeof appearanceSchema>;

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
