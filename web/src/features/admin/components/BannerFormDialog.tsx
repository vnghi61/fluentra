import * as React from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { AlertCircle } from "lucide-react";

import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { NativeSelect } from "@/components/ui/native-select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  BANNER_KINDS,
  BANNER_LIMITS,
  BANNER_TONES,
  BannerCard,
  BannerSlide,
  SLIDE_RECOMMENDED_SIZE,
  isSafeBannerUrl,
  useCreateBanner,
  useUpdateBanner,
  type Banner,
} from "@/features/banners";
import {
  bannerFormSchema,
  bannerToForm,
  emptyBannerForm,
  formToBannerRequest,
  SERVER_FIELD_TO_FORM,
  type BannerFormValues,
} from "../model/bannerForm";
import { AdminNotice } from "./AdminSurface";
import { BannerImageField } from "./BannerImageField";

const FORM_ID = "admin-banner-form";

const SERVER_CODE_TO_MESSAGE: Record<string, string> = {
  required: "adminBanners.err.titleRequired",
  too_long: "adminBanners.err.tooLong",
  unsafe_url: "adminBanners.err.urlUnsafe",
  cta_incomplete: "adminBanners.err.ctaPair",
  out_of_range: "adminBanners.err.priorityRange",
  ends_before_start: "adminBanners.err.windowOrder",
  not_found: "adminBanners.err.imageMissing",
  in_use: "adminBanners.err.imageInUse",
};

interface ServerFieldError {
  field?: unknown;
  code?: unknown;
}

export interface BannerFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The banner being edited; absent to create one. */
  banner?: Banner | null | undefined;
  onSaved?: ((banner: Banner) => void) | undefined;
}

export function BannerFormDialog({
  open,
  onOpenChange,
  banner,
  onSaved,
}: BannerFormDialogProps): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="break-words">
            {banner
              ? t("adminBanners.editTitle")
              : t("adminBanners.createTitle")}
          </DialogTitle>
          <DialogDescription>
            {t("adminBanners.formDescription")}
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so each opening starts from the banner. */}
        {open && (
          <BannerForm
            banner={banner ?? null}
            onCancel={() => onOpenChange(false)}
            onSaved={(saved) => {
              onSaved?.(saved);
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function BannerForm({
  banner,
  onCancel,
  onSaved,
}: {
  banner: Banner | null;
  onCancel: () => void;
  onSaved: (banner: Banner) => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  const createBanner = useCreateBanner();
  const updateBanner = useUpdateBanner();
  const [formError, setFormError] = React.useState<string | null>(null);
  const [imageBusy, setImageBusy] = React.useState(false);

  const form = useForm<BannerFormValues>({
    resolver: zodResolver(bannerFormSchema),
    defaultValues: banner ? bannerToForm(banner) : emptyBannerForm,
    mode: "onTouched",
  });
  const values = useWatch({ control: form.control });
  const isPending = createBanner.isPending || updateBanner.isPending;
  const imageKey = values.imageKey ?? "";
  const imageUrl = values.imageUrl ?? "";
  const isSlide = values.kind === "slide";

  const onSubmit = async (submitted: BannerFormValues) => {
    setFormError(null);
    const body = formToBannerRequest(submitted);
    try {
      const saved = banner
        ? await updateBanner.mutateAsync({ id: banner.id, body })
        : await createBanner.mutateAsync(body);
      onSaved(saved);
    } catch (err: unknown) {
      if (!applyServerFieldErrors(err)) {
        setFormError(
          err instanceof Error && err.message
            ? err.message
            : t("adminBanners.saveFailed"),
        );
      }
    }
  };

  /** Maps a 422's per-field violations onto the form; true when any landed. */
  function applyServerFieldErrors(err: unknown): boolean {
    if (!(err instanceof ApiError)) return false;
    const raw: unknown = err.problem.errors;
    if (!Array.isArray(raw)) return false;
    let applied = false;
    for (const entry of raw as ServerFieldError[]) {
      const field =
        typeof entry.field === "string"
          ? SERVER_FIELD_TO_FORM[entry.field]
          : undefined;
      if (!field) continue;
      const code = typeof entry.code === "string" ? entry.code : "";
      form.setError(
        field,
        {
          message: SERVER_CODE_TO_MESSAGE[code] ?? "adminBanners.err.invalid",
        },
        { shouldFocus: !applied },
      );
      applied = true;
    }
    return applied;
  }

  const previewUrl = (values.ctaUrl ?? "").trim();
  const preview = {
    title: (values.title ?? "").trim() || t("adminBanners.previewPlaceholder"),
    body: (values.body ?? "").trim(),
    cta_label: (values.ctaLabel ?? "").trim() || null,
    cta_url: previewUrl && isSafeBannerUrl(previewUrl) ? previewUrl : null,
    tone: values.tone ?? "info",
    dismissible: values.dismissible ?? true,
    image_url: imageKey && imageUrl ? imageUrl : null,
    image_alt: (values.imageAlt ?? "").trim(),
  };

  return (
    <>
      <DialogBody>
        <Form {...form}>
          <form
            id={FORM_ID}
            noValidate
            onSubmit={(event) => {
              void form.handleSubmit(onSubmit)(event);
            }}
            className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,18rem)]"
          >
            <div className="space-y-4">
              {formError && (
                <AdminNotice tone="danger">
                  <span className="flex items-start gap-2">
                    <AlertCircle
                      aria-hidden="true"
                      className="h-5 w-5 shrink-0"
                    />
                    {formError}
                  </span>
                </AdminNotice>
              )}

              <FormField
                control={form.control}
                name="kind"
                render={({ field }) => (
                  <FormItem>
                    <fieldset className="space-y-2">
                      <legend className="text-sm font-medium text-text">
                        {t("adminBanners.fieldKind")}
                      </legend>
                      <div className="grid gap-2 sm:grid-cols-2">
                        {BANNER_KINDS.map((kind) => {
                          const checked = field.value === kind;
                          return (
                            <label
                              key={kind}
                              className={cn(
                                "flex min-h-[44px] cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors focus-within:ring-2 focus-within:ring-primary motion-reduce:transition-none",
                                checked
                                  ? "border-primary bg-primary/5"
                                  : "border-border-subtle hover:border-primary/50",
                              )}
                            >
                              <input
                                type="radio"
                                name={field.name}
                                value={kind}
                                checked={checked}
                                onChange={() => {
                                  field.onChange(kind);
                                  form.clearErrors();
                                }}
                                className="sr-only"
                              />
                              {/* The whole card is the 44px target; this is
                                only the visible radio mark. */}
                              <span
                                aria-hidden="true"
                                className={cn(
                                  "mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2",
                                  checked ? "border-primary" : "border-border",
                                )}
                              >
                                {checked && (
                                  <span className="h-2 w-2 rounded-full bg-primary" />
                                )}
                              </span>
                              <span className="min-w-0">
                                <span className="block text-sm font-semibold text-text">
                                  {t(`adminBanners.kind.${kind}`)}
                                </span>
                                <span className="block text-sm text-text-muted">
                                  {t(`adminBanners.kindHint.${kind}`)}
                                </span>
                              </span>
                            </label>
                          );
                        })}
                      </div>
                    </fieldset>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="title"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {isSlide
                        ? t("adminBanners.fieldSlideTitle")
                        : t("adminBanners.fieldTitle")}
                    </FormLabel>
                    <FormControl>
                      <Input {...field} maxLength={BANNER_LIMITS.title} />
                    </FormControl>
                    {isSlide && (
                      <FormDescription>
                        {t("adminBanners.slideTitleHint")}
                      </FormDescription>
                    )}
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isSlide && (
                <FormField
                  control={form.control}
                  name="body"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("adminBanners.fieldBody")}</FormLabel>
                      <FormControl>
                        <Textarea
                          {...field}
                          rows={3}
                          maxLength={BANNER_LIMITS.body}
                        />
                      </FormControl>
                      <FormDescription>
                        {t("adminBanners.plainTextHint", {
                          count: (field.value ?? "").length,
                          max: BANNER_LIMITS.body,
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <div className="space-y-3">
                <FormField
                  control={form.control}
                  name="imageKey"
                  render={() => (
                    <FormItem>
                      <FormLabel htmlFor="banner-image">
                        {isSlide
                          ? t("adminBanners.image.slideLabel")
                          : t("adminBanners.image.label")}
                      </FormLabel>
                      <BannerImageField
                        id="banner-image"
                        value={
                          imageKey && imageUrl
                            ? { key: imageKey, url: imageUrl }
                            : null
                        }
                        onBusyChange={setImageBusy}
                        disabled={isPending}
                        hint={
                          isSlide
                            ? t("adminBanners.image.slideHint", {
                                size: SLIDE_RECOMMENDED_SIZE,
                              })
                            : undefined
                        }
                        aspect={isSlide ? "slide" : "wide"}
                        onChange={(image) => {
                          const opts = { shouldDirty: true } as const;
                          form.setValue("imageKey", image?.key ?? "", opts);
                          form.setValue("imageUrl", image?.url ?? "", opts);
                          if (!image) form.setValue("imageAlt", "", opts);
                          form.clearErrors("imageKey");
                        }}
                      />
                      <FormMessage />
                    </FormItem>
                  )}
                />
                {imageKey && (
                  <FormField
                    control={form.control}
                    name="imageAlt"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t("adminBanners.image.altLabel")}
                        </FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            maxLength={BANNER_LIMITS.imageAlt}
                            autoComplete="off"
                          />
                        </FormControl>
                        <FormDescription>
                          {t("adminBanners.image.altHint")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>

              <div className={cn("grid gap-4", !isSlide && "sm:grid-cols-2")}>
                {!isSlide && (
                  <FormField
                    control={form.control}
                    name="ctaLabel"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("adminBanners.fieldCtaLabel")}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            maxLength={BANNER_LIMITS.ctaLabel}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name="ctaUrl"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {isSlide
                          ? t("adminBanners.fieldSlideUrl")
                          : t("adminBanners.fieldCtaUrl")}
                      </FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          inputMode="url"
                          autoComplete="off"
                          placeholder="/exams"
                        />
                      </FormControl>
                      <FormDescription>
                        {t("adminBanners.ctaUrlHint")}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                {!isSlide && (
                  <FormField
                    control={form.control}
                    name="tone"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("adminBanners.fieldTone")}</FormLabel>
                        <FormControl>
                          <NativeSelect {...field}>
                            {BANNER_TONES.map((tone) => (
                              <option key={tone} value={tone}>
                                {t(`adminBanners.tone.${tone}`)}
                              </option>
                            ))}
                          </NativeSelect>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name="priority"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("adminBanners.fieldPriority")}</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          inputMode="numeric"
                          min={0}
                          max={BANNER_LIMITS.priorityMax}
                          step={1}
                          name={field.name}
                          ref={field.ref}
                          onBlur={field.onBlur}
                          value={Number.isNaN(field.value) ? "" : field.value}
                          onChange={(event) =>
                            field.onChange(
                              event.target.value === ""
                                ? Number.NaN
                                : Number(event.target.value),
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t("adminBanners.priorityHint")}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="startsAt"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("adminBanners.fieldStartsAt")}</FormLabel>
                      <FormControl>
                        <Input {...field} type="datetime-local" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="endsAt"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("adminBanners.fieldEndsAt")}</FormLabel>
                      <FormControl>
                        <Input {...field} type="datetime-local" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <p className="-mt-2 text-sm text-text-muted">
                {t("adminBanners.windowHint")}
              </p>

              <div className="grid gap-2 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="isActive"
                  render={({ field }) => (
                    <FormItem className="flex min-h-11 items-center justify-between gap-3 space-y-0 rounded-xl border border-border-subtle px-3 py-2">
                      <FormLabel>{t("adminBanners.fieldActive")}</FormLabel>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                {!isSlide && (
                  <FormField
                    control={form.control}
                    name="dismissible"
                    render={({ field }) => (
                      <FormItem className="flex min-h-11 items-center justify-between gap-3 space-y-0 rounded-xl border border-border-subtle px-3 py-2">
                        <FormLabel>
                          {t("adminBanners.fieldDismissible")}
                        </FormLabel>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                )}
              </div>
            </div>

            <aside
              aria-label={t("adminBanners.previewLabel")}
              className="space-y-2 lg:sticky lg:top-0 lg:self-start"
            >
              <p className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                {t("adminBanners.previewLabel")}
              </p>
              {isSlide ? (
                <BannerSlide
                  slide={{
                    title: preview.title,
                    cta_url: preview.cta_url,
                    image_url: preview.image_url,
                    image_alt: preview.image_alt,
                  }}
                  preview
                />
              ) : (
                <BannerCard banner={preview} preview />
              )}
              <p className="text-sm text-text-muted">
                {isSlide
                  ? t("adminBanners.previewHintSlide")
                  : t("adminBanners.previewHint")}
              </p>
            </aside>
          </form>
        </Form>
      </DialogBody>
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={isPending}
        >
          {t("common.cancel")}
        </Button>
        <Button
          type="submit"
          form={FORM_ID}
          isLoading={isPending}
          disabled={imageBusy}
        >
          {banner ? t("common.saveChanges") : t("adminBanners.create")}
        </Button>
      </DialogFooter>
    </>
  );
}
