import React, { useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertCircle, Flag, Loader2, X } from "lucide-react";
import {
  createFeatureFlagSchema,
  type CreateFeatureFlagFormValues,
} from "../model/schemas";
import { adminApi, type FeatureFlag } from "../api/adminApi";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface CreateFeatureFlagModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (created: FeatureFlag) => void;
}

export const CreateFeatureFlagModal: React.FC<CreateFeatureFlagModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Next month default date
  const defaultExpiry = new Date();
  defaultExpiry.setMonth(defaultExpiry.getMonth() + 1);
  const defaultExpiryString = defaultExpiry.toISOString().slice(0, 10);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
    reset,
  } = useForm<CreateFeatureFlagFormValues>({
    resolver: (zodResolver as (schema: unknown) => never)(createFeatureFlagSchema),
    defaultValues: {
      key: "",
      description: "",
      enabled: false,
      rollout_percent: 0,
      owner: "@backend-team",
      expires_on: defaultExpiryString,
    },
  });

  if (!isOpen) return null;

  const onSubmit = async (values: CreateFeatureFlagFormValues) => {
    setIsSubmitting(true);
    setError(null);

    try {
      const created = await adminApi.createFlag({
        key: values.key,
        description: values.description,
        enabled: values.enabled,
        rollout_percent: values.rollout_percent,
        owner: values.owner,
        expires_on: values.expires_on,
      });

      reset();
      onSuccess(created);
      onClose();
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to create feature flag.",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-flag-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-lg rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <h2
            id="create-flag-modal-title"
            className="text-lg font-semibold text-slate-100 flex items-center gap-2"
          >
            <Flag className="h-5 w-5 text-indigo-400" />
            Create Feature Flag
          </h2>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3.5 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <form
          onSubmit={(e) => {
            void handleSubmit(onSubmit)(e);
          }}
          className="space-y-4"
        >
          {/* Key */}
          <div className="space-y-1.5">
            <Label htmlFor="flag-key">Flag Identifier Key</Label>
            <Input
              id="flag-key"
              {...register("key")}
              placeholder="e.g. streaks_v2, ai_grading_v2"
              aria-invalid={!!errors.key}
            />
            {errors.key && (
              <p className="text-xs text-rose-400">{errors.key.message}</p>
            )}
          </div>

          {/* Description */}
          <div className="space-y-1.5">
            <Label htmlFor="flag-desc">Description</Label>
            <Input
              id="flag-desc"
              {...register("description")}
              placeholder="What does this feature flag control?"
              aria-invalid={!!errors.description}
            />
            {errors.description && (
              <p className="text-xs text-rose-400">{errors.description.message}</p>
            )}
          </div>

          {/* Owner & Expiry */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="flag-owner">Owner / Team</Label>
              <Input
                id="flag-owner"
                {...register("owner")}
                placeholder="@team"
                aria-invalid={!!errors.owner}
              />
              {errors.owner && (
                <p className="text-xs text-rose-400">{errors.owner.message}</p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="flag-expiry">Expires On (Future)</Label>
              <Input
                id="flag-expiry"
                type="date"
                {...register("expires_on")}
                aria-invalid={!!errors.expires_on}
              />
              {errors.expires_on && (
                <p className="text-xs text-rose-400">{errors.expires_on.message}</p>
              )}
            </div>
          </div>

          {/* Rollout % */}
          <div className="space-y-1.5">
            <Label htmlFor="flag-rollout">Rollout Percentage (0 - 100%)</Label>
            <Input
              id="flag-rollout"
              type="number"
              min={0}
              max={100}
              {...register("rollout_percent", { valueAsNumber: true })}
              aria-invalid={!!errors.rollout_percent}
            />
            {errors.rollout_percent && (
              <p className="text-xs text-rose-400">{errors.rollout_percent.message}</p>
            )}
          </div>

          {/* Enabled Checkbox */}
          <div className="flex items-center gap-2.5 pt-1">
            <Controller
              control={control}
              name="enabled"
              render={({ field }) => (
                <Checkbox
                  id="flag-enabled"
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              )}
            />
            <Label htmlFor="flag-enabled" className="text-sm font-medium cursor-pointer">
              Enable immediately on creation
            </Label>
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-slate-800 pt-4 mt-6">
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating...
                </>
              ) : (
                "Save Feature Flag"
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
