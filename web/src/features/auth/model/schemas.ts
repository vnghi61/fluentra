import { z } from "zod";

export const loginSchema = z.object({
  email: z.string().min(1, "Email is required").email("Please enter a valid email address"),
  password: z.string().min(1, "Password is required"),
  remember_device: z.boolean().default(true),
});

export type LoginFormData = z.infer<typeof loginSchema>;

export const registerSchema = z.object({
  email: z.string().min(1, "Email is required").email("Please enter a valid email address"),
  display_name: z.string().min(1, "Name is required").max(50, "Name is too long"),
  password: z
    .string()
    .min(12, "Password must be at least 12 characters for security"),
  locale: z.string().default("en"),
  timezone: z.string().default("UTC"),
});

export type RegisterFormData = z.infer<typeof registerSchema>;

export const otpSchema = z.object({
  code: z
    .string()
    .length(6, "Verification code must be exactly 6 digits")
    .regex(/^\d{6}$/, "Code must contain digits only"),
});

export type OtpFormData = z.infer<typeof otpSchema>;

export const forgotPasswordSchema = z.object({
  email: z.string().min(1, "Email is required").email("Please enter a valid email address"),
});

export type ForgotPasswordFormData = z.infer<typeof forgotPasswordSchema>;

export const resetPasswordSchema = z.object({
  code: z
    .string()
    .length(6, "Reset code must be exactly 6 digits")
    .regex(/^\d{6}$/, "Code must contain digits only"),
  password: z
    .string()
    .min(12, "New password must be at least 12 characters for security"),
});

export type ResetPasswordFormData = z.infer<typeof resetPasswordSchema>;
