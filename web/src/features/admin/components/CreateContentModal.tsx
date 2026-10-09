import React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import {
  createContentSchema,
  type CreateContentFormValues,
} from "../model/schemas";
import {
  adminApi,
  type ContentItem,
  type CreateContentItemRequest,
} from "../api/adminApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { NativeSelect } from "@/components/ui/native-select";
import { Textarea } from "@/components/ui/textarea";

export function CreateContentModal({
  open,
  onOpenChange,
  onCreated,
  kindOptions,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (item: ContentItem) => void;
  kindOptions: readonly { value: string; label: string }[];
}): React.JSX.Element {
  const { t } = useTranslation();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateContentFormValues>({
    resolver: zodResolver(createContentSchema),
    defaultValues: {
      slug: "",
      kind: "vocab_word",
      cefr_level: "A1",
      body: "{}",
    },
  });
  const [error, setError] = React.useState<string | null>(null);
  const submit = async (values: CreateContentFormValues) => {
    setError(null);
    try {
      const body: unknown = JSON.parse(values.body);
      const created = await adminApi.createContent({
        ...values,
        body: body as CreateContentItemRequest["body"],
      });
      onCreated(created);
      onOpenChange(false);
    } catch {
      setError(t("admin.failedToCreateContent"));
    }
  };
  const fieldError = (name: "slug" | "kind" | "body") =>
    errors[name] && (
      <p
        id={`content-create-${name}-error`}
        className="text-sm text-danger-accent"
      >
        {t(errors[name]?.message ?? "admin.contentFieldRequired")}
      </p>
    );

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!isSubmitting) onOpenChange(next);
      }}
    >
      <DialogContent showClose={false}>
        <DialogHeader>
          <DialogTitle className="pr-12">{t("admin.addContent")}</DialogTitle>
          <DialogDescription className="mt-1">
            {t("admin.createContentHint")}
          </DialogDescription>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            disabled={isSubmitting}
            onClick={() => onOpenChange(false)}
            aria-label={t("common.close")}
            className="absolute right-3 top-3"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </Button>
        </DialogHeader>
        <DialogBody>
          <form
            id="create-content-form"
            noValidate
            onSubmit={(event) => {
              void handleSubmit(submit)(event);
            }}
          >
            <fieldset disabled={isSubmitting} className="space-y-5">
              {error && (
                <div
                  role="alert"
                  className="rounded-xl border border-danger/20 bg-danger/10 p-3 text-sm text-danger-accent"
                >
                  {error}
                </div>
              )}
              <div className="space-y-2">
                <Label htmlFor="content-create-slug">
                  {t("admin.contentSlug")}
                </Label>
                <Input
                  id="content-create-slug"
                  autoComplete="off"
                  {...register("slug")}
                  aria-invalid={Boolean(errors.slug)}
                  aria-describedby={
                    errors.slug
                      ? "content-create-slug-error"
                      : "content-create-slug-hint"
                  }
                />
                <p
                  id="content-create-slug-hint"
                  className="text-sm text-text-muted"
                >
                  {t("admin.contentSlugHint")}
                </p>
                {fieldError("slug")}
              </div>
              <div className="grid gap-5 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="content-create-kind">{t("admin.kind")}</Label>
                  <Input
                    id="content-create-kind"
                    list="content-create-kinds"
                    autoComplete="off"
                    {...register("kind")}
                    aria-invalid={Boolean(errors.kind)}
                    aria-describedby={
                      errors.kind ? "content-create-kind-error" : undefined
                    }
                  />
                  <datalist id="content-create-kinds">
                    {kindOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </datalist>
                  {fieldError("kind")}
                </div>
                <div className="space-y-2">
                  <Label htmlFor="content-create-level">
                    {t("admin.cefrLevel")}
                  </Label>
                  <NativeSelect
                    id="content-create-level"
                    {...register("cefr_level")}
                  >
                    {["A1", "A2", "B1", "B2", "C1", "C2"].map((level) => (
                      <option key={level}>{level}</option>
                    ))}
                  </NativeSelect>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="content-create-body">
                  {t("admin.contentBodyJson")}
                </Label>
                <Textarea
                  id="content-create-body"
                  rows={7}
                  spellCheck={false}
                  {...register("body")}
                  aria-invalid={Boolean(errors.body)}
                  aria-describedby={
                    errors.body
                      ? "content-create-body-error"
                      : "content-create-body-hint"
                  }
                  className="min-h-40 resize-y font-mono"
                />
                <p
                  id="content-create-body-hint"
                  className="text-sm text-text-muted"
                >
                  {t("admin.contentBodyHint")}
                </p>
                {fieldError("body")}
              </div>
            </fieldset>
          </form>
        </DialogBody>
        <DialogFooter className="flex-col-reverse sm:flex-row">
          <Button
            type="button"
            variant="outline"
            disabled={isSubmitting}
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            form="create-content-form"
            isLoading={isSubmitting}
          >
            {t("admin.createDraft")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
