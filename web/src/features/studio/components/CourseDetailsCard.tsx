import React from "react";
import { Controller, useWatch, type UseFormReturn } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Gift, Info, Wallet } from "lucide-react";

import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import { Textarea } from "@/components/ui/textarea";
import { formatVnd } from "@/lib/currency";
import { cn } from "@/lib/utils";
import type { CourseFormValues } from "../model/courseForm";
import { CEFR_LEVELS, PRICE, slugify } from "../model/courseStructure";

interface Props {
  form: UseFormReturn<CourseFormValues>;
  /** Once the creator types a slug, the title stops rewriting it. */
  slugTouched: boolean;
  onSlugTouched: () => void;
}

function FieldError({
  id,
  message,
}: {
  id: string;
  message?: string | undefined;
}): React.JSX.Element | null {
  const { t } = useTranslation();
  if (!message) return null;
  return (
    <p id={id} role="alert" className="text-sm font-medium text-danger-accent">
      {t(`studio.editor.err.${message}`, {
        min: formatVnd(PRICE.min),
        max: formatVnd(PRICE.max),
      })}
    </p>
  );
}

const fieldLabel = "text-sm font-medium text-text";

/** Title, URL, description, level and price: what a learner sees in the catalogue. */
export function CourseDetailsCard({
  form,
  slugTouched,
  onSlugTouched,
}: Props): React.JSX.Element {
  const { t } = useTranslation();
  const {
    register,
    control,
    setValue,
    formState: { errors },
  } = form;
  const [isFree, priceVnd, description] = useWatch({
    control,
    name: ["isFree", "priceVnd", "description"],
  });

  const titleField = register("title", {
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
      if (!slugTouched) {
        setValue("slug", slugify(e.target.value), {
          shouldDirty: true,
          shouldValidate: Boolean(errors.slug),
        });
      }
    },
  });
  const slugField = register("slug", { onChange: onSlugTouched });

  return (
    <Card className="p-4 sm:p-6">
      <div className="mb-5">
        <h2 className="text-lg font-semibold leading-snug text-text md:text-xl">
          {t("studio.editor.metadataTitle")}
        </h2>
        <p className="text-sm text-text-muted">
          {t("studio.editor.metadataHint")}
        </p>
      </div>

      <div className="grid gap-5 md:grid-cols-2">
        <div className="space-y-1.5 md:col-span-2">
          <label htmlFor="course-title" className={fieldLabel}>
            {t("studio.editor.titleLabel")}
            <span className="text-danger-accent" aria-hidden="true">
              {" "}
              *
            </span>
          </label>
          <Input
            id="course-title"
            {...titleField}
            maxLength={120}
            error={Boolean(errors.title)}
            aria-describedby={errors.title ? "course-title-err" : undefined}
            placeholder={t("studio.editor.titlePlaceholder")}
          />
          <FieldError id="course-title-err" message={errors.title?.message} />
        </div>

        <div className="space-y-1.5 md:col-span-2">
          <label htmlFor="course-slug" className={fieldLabel}>
            {t("studio.editor.slugLabel")}
          </label>
          <div
            className={cn(
              "flex h-11 items-center overflow-hidden rounded-lg border bg-surface focus-within:ring-2 focus-within:ring-offset-2 focus-within:ring-offset-surface",
              errors.slug
                ? "border-danger focus-within:ring-danger"
                : "border-border focus-within:border-primary focus-within:ring-primary",
            )}
          >
            <span className="hidden h-full items-center border-r border-border-subtle bg-surface-muted px-3 text-sm text-text-muted sm:flex">
              /courses/
            </span>
            <input
              id="course-slug"
              {...slugField}
              maxLength={80}
              autoCapitalize="none"
              spellCheck={false}
              aria-invalid={errors.slug ? "true" : undefined}
              aria-describedby="course-slug-hint"
              placeholder={t("studio.editor.slugPlaceholder")}
              className="h-11 min-w-11 flex-1 bg-transparent px-3 text-base text-text placeholder:text-text-muted focus:outline-none md:text-sm"
            />
          </div>
          {errors.slug ? (
            <FieldError id="course-slug-hint" message={errors.slug.message} />
          ) : (
            <p id="course-slug-hint" className="text-sm text-text-muted">
              {t("studio.editor.slugHint")}
            </p>
          )}
        </div>

        <div className="space-y-1.5 md:col-span-2">
          <div className="flex items-baseline justify-between">
            <label htmlFor="course-desc" className={fieldLabel}>
              {t("studio.editor.descLabel")}
            </label>
            <span className="text-xs tabular-nums text-text-muted">
              {description.length}/2000
            </span>
          </div>
          <Textarea
            id="course-desc"
            rows={4}
            maxLength={2000}
            {...register("description")}
            placeholder={t("studio.editor.descPlaceholder")}
          />
        </div>

        <div className="space-y-1.5">
          <label htmlFor="course-cefr" className={fieldLabel}>
            {t("studio.editor.cefrLabel")}
          </label>
          <NativeSelect id="course-cefr" {...register("cefrLevel")}>
            {CEFR_LEVELS.map((lvl) => (
              <option key={lvl} value={lvl}>
                {lvl} · {t(`studio.editor.cefr.${lvl}`)}
              </option>
            ))}
          </NativeSelect>
        </div>
      </div>

      <fieldset className="mt-6 space-y-3 border-t border-border-subtle pt-5">
        <legend className="sr-only">{t("studio.editor.pricingLabel")}</legend>
        <p className={fieldLabel} aria-hidden="true">
          {t("studio.editor.pricingLabel")}
        </p>
        <Controller
          control={control}
          name="isFree"
          render={({ field }) => (
            <div className="grid gap-3 sm:grid-cols-2">
              {(
                [
                  {
                    value: true,
                    icon: Gift,
                    title: t("studio.editor.priceFree"),
                    desc: t("studio.editor.priceFreeDesc"),
                  },
                  {
                    value: false,
                    icon: Wallet,
                    title: t("studio.editor.pricePaid"),
                    desc: t("studio.editor.pricePaidDesc", {
                      min: formatVnd(PRICE.min),
                    }),
                  },
                ] as const
              ).map((opt) => {
                const checked = field.value === opt.value;
                const Icon = opt.icon;
                return (
                  <label
                    key={String(opt.value)}
                    className={cn(
                      "flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 transition-colors focus-within:ring-2 focus-within:ring-primary focus-within:ring-offset-2 focus-within:ring-offset-surface",
                      checked
                        ? "border-primary bg-primary/5"
                        : "border-border-subtle hover:bg-surface-muted/60",
                    )}
                  >
                    <input
                      type="radio"
                      name={field.name}
                      checked={checked}
                      onChange={() => field.onChange(opt.value)}
                      className="sr-only"
                    />
                    <span
                      className={cn(
                        "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg",
                        checked
                          ? "bg-primary text-primary-fg"
                          : "bg-surface-muted text-text-muted",
                      )}
                      aria-hidden="true"
                    >
                      <Icon className="h-5 w-5" />
                    </span>
                    <span className="min-w-0">
                      <span className="block text-sm font-semibold text-text">
                        {opt.title}
                      </span>
                      <span className="block text-sm text-text-muted">
                        {opt.desc}
                      </span>
                    </span>
                  </label>
                );
              })}
            </div>
          )}
        />

        {!isFree && (
          <div className="space-y-2 rounded-xl bg-surface-muted/60 p-3.5">
            <label htmlFor="course-price" className={fieldLabel}>
              {t("studio.editor.priceLabel")}
            </label>
            <Controller
              control={control}
              name="priceVnd"
              render={({ field }) => (
                <div className="relative max-w-xs">
                  <Input
                    id="course-price"
                    inputMode="numeric"
                    autoComplete="off"
                    value={
                      Number.isFinite(field.value) && field.value > 0
                        ? field.value.toLocaleString("vi-VN")
                        : ""
                    }
                    onChange={(e) =>
                      field.onChange(Number(e.target.value.replace(/\D/g, "")))
                    }
                    onBlur={field.onBlur}
                    error={Boolean(errors.priceVnd)}
                    aria-describedby="course-price-help"
                    className="pr-10 font-semibold tabular-nums"
                  />
                  <span className="pointer-events-none absolute right-3.5 top-1/2 -translate-y-1/2 text-sm text-text-muted">
                    ₫
                  </span>
                </div>
              )}
            />
            <div
              className="flex flex-wrap gap-2"
              role="group"
              aria-label={t("studio.editor.priceSuggestions")}
            >
              {PRICE.suggestions.map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() =>
                    setValue("priceVnd", p, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  className={cn(
                    "min-h-11 rounded-full border px-3.5 text-sm font-semibold tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
                    priceVnd === p
                      ? "border-primary bg-primary/10 text-primary-accent"
                      : "border-border-subtle text-text-muted hover:bg-surface",
                  )}
                >
                  {formatVnd(p)}
                </button>
              ))}
            </div>
            {errors.priceVnd ? (
              <FieldError
                id="course-price-help"
                message={errors.priceVnd.message}
              />
            ) : (
              <p
                id="course-price-help"
                className="flex items-start gap-1.5 text-sm text-text-muted"
              >
                <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                {t("studio.editor.priceHelp", {
                  min: formatVnd(PRICE.min),
                  max: formatVnd(PRICE.max),
                  share: formatVnd(
                    Math.floor((priceVnd || 0) * PRICE.creatorShare),
                  ),
                })}
              </p>
            )}
          </div>
        )}
      </fieldset>
    </Card>
  );
}
