import { z } from "zod";

export const loginSchema = z.object({
  email: z
    .string()
    .min(1, "validation.emailRequired")
    .email("validation.emailInvalid"),
  password: z.string().min(1, "validation.passwordRequired"),
  remember_device: z.boolean().default(true),
});

export type LoginFormData = z.infer<typeof loginSchema>;

export const registerSchema = z.object({
  email: z
    .string()
    .min(1, "validation.emailRequired")
    .email("validation.emailInvalid"),
  display_name: z
    .string()
    .min(1, "validation.nameRequired")
    .max(50, "validation.nameTooLong"),
  password: z.string().min(12, "validation.passwordMinLength"),
  locale: z.string().default("en"),
  timezone: z.string().default("UTC"),
});

export type RegisterFormData = z.infer<typeof registerSchema>;

export const otpSchema = z.object({
  code: z
    .string()
    .length(6, "validation.verificationCodeLength")
    .regex(/^\d{6}$/, "validation.codeDigitsOnly"),
});

export type OtpFormData = z.infer<typeof otpSchema>;

export const forgotPasswordSchema = z.object({
  email: z
    .string()
    .min(1, "validation.emailRequired")
    .email("validation.emailInvalid"),
});

export type ForgotPasswordFormData = z.infer<typeof forgotPasswordSchema>;

export const resetPasswordSchema = z.object({
  code: z
    .string()
    .length(6, "validation.resetCodeLength")
    .regex(/^\d{6}$/, "validation.codeDigitsOnly"),
  password: z.string().min(12, "validation.newPasswordMinLength"),
});

export type ResetPasswordFormData = z.infer<typeof resetPasswordSchema>;
