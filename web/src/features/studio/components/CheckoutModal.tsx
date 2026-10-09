import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle,
  Check,
  CheckCircle2,
  Copy,
  GraduationCap,
  Loader2,
  QrCode,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { studioApi } from "../api/studioApi";
import { useBillingOrder } from "../hooks/useStudio";
import { formatVnd } from "@/lib/currency";

interface CheckoutModalProps {
  courseId: string;
  courseTitle: string;
  priceVnd: number;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export function CheckoutModal({
  courseId,
  courseTitle,
  priceVnd,
  isOpen,
  onClose,
  onSuccess,
}: CheckoutModalProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const isFree = priceVnd === 0;

  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [claimSuccess, setClaimSuccess] = useState(false);
  const [claimLoading, setClaimLoading] = useState(false);
  const [claimError, setClaimError] = useState<string | null>(null);

  const {
    data: orderData,
    isLoading: orderLoading,
    error: orderQueryError,
  } = useQuery({
    queryKey: ["studio", "order", courseId],
    queryFn: () => studioApi.purchaseCourse(courseId),
    enabled: isOpen && !isFree,
    staleTime: 5 * 60 * 1000,
  });

  const order = orderData ?? null;
  const loading = isFree ? claimLoading : orderLoading;
  const error = isFree
    ? claimError
    : orderQueryError instanceof Error
      ? orderQueryError.message
      : null;

  // Poll order status if order was created
  const { data: polledOrder } = useBillingOrder(order?.order_id, 3000);
  const isPaid = polledOrder?.status === "paid";

  if (!isOpen) return null;

  const handleCopy = (text: string, fieldName: string) => {
    void navigator.clipboard.writeText(text);
    setCopiedField(fieldName);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const handleClaimFree = async () => {
    setClaimLoading(true);
    setClaimError(null);
    try {
      await studioApi.claimCourse(courseId);
      setClaimSuccess(true);
    } catch (err) {
      setClaimError(
        err instanceof Error
          ? err.message
          : t("studio.checkout.errClaimFailed", "Failed to claim free course"),
      );
    } finally {
      setClaimLoading(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !claimLoading) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <GraduationCap
              className="h-5 w-5 shrink-0 text-primary-accent"
              aria-hidden="true"
            />
            {isFree
              ? t("studio.checkout.claimTitle", "Enroll in Course")
              : t("studio.checkout.payTitle", "Course Purchase (VietQR)")}
          </DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-6">
          {/* Course Summary */}
          <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 space-y-1">
            <span className="text-sm text-text-muted">
              {t("studio.checkout.course", "Course")}
            </span>
            <div className="break-words text-base font-semibold leading-snug text-text">
              {courseTitle}
            </div>
            <div className="flex items-center gap-2 pt-2">
              <span className="text-sm text-text-muted">
                {t("studio.checkout.price", "Price")}:
              </span>
              <span className="text-base font-bold tabular-nums text-primary-accent">
                {isFree
                  ? t("studio.courses.freeBadge", "Free")
                  : formatVnd(priceVnd)}
              </span>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger-accent font-medium">
              {error}
            </div>
          )}

          {/* Paid Course: Success State */}
          {isPaid ? (
            <div className="py-8 text-center space-y-4">
              <div className="h-16 w-16 rounded-full bg-success/10 text-success flex items-center justify-center mx-auto animate-bounce">
                <CheckCircle2 className="h-10 w-10" aria-hidden="true" />
              </div>
              <div>
                <h3 className="text-xl font-bold leading-tight text-text">
                  {t("studio.checkout.paymentConfirmed", "Payment Verified!")}
                </h3>
                <p className="mx-auto mt-1 max-w-sm text-base leading-relaxed text-text-muted">
                  {t(
                    "studio.checkout.accessGranted",
                    "Your bank transfer was matched successfully. You now have full lifetime access to this course.",
                  )}
                </p>
              </div>
              <Button
                variant="primary"
                size="lg"
                onClick={() => {
                  onSuccess();
                  onClose();
                }}
                className="w-full"
              >
                {t("studio.checkout.startLearning", "Start Learning Now")}
              </Button>
            </div>
          ) : isFree ? (
            /* Free Course Flow */
            <div className="py-4 space-y-4 text-center">
              {claimSuccess ? (
                <div className="flex items-center justify-center gap-2 text-success-accent font-semibold py-4">
                  <CheckCircle2
                    className="h-5 w-5 shrink-0"
                    aria-hidden="true"
                  />
                  <span>
                    {t(
                      "studio.checkout.enrolledSuccess",
                      "Enrolled successfully! Loading curriculum...",
                    )}
                  </span>
                </div>
              ) : (
                <>
                  <p className="text-base leading-relaxed text-text-muted">
                    {t(
                      "studio.checkout.freeConfirmText",
                      "This is a free community course. Click below to add it to your library and unlock all syllabus activities.",
                    )}
                  </p>
                  <Button
                    variant="primary"
                    size="lg"
                    disabled={loading}
                    onClick={() => {
                      void handleClaimFree();
                    }}
                    className="w-full"
                  >
                    {loading
                      ? t("studio.checkout.enrolling", "Enrolling...")
                      : t("studio.checkout.confirmEnroll", "Enroll for Free")}
                  </Button>
                </>
              )}
            </div>
          ) : loading ? (
            <div className="py-12 text-center space-y-3">
              <Loader2
                className="h-8 w-8 animate-spin text-primary-accent mx-auto"
                aria-hidden="true"
              />
              <p className="text-sm text-text-muted">
                {t(
                  "studio.checkout.generatingOrder",
                  "Generating secure VietQR transfer code...",
                )}
              </p>
            </div>
          ) : order ? (
            /* VietQR Payment Details */
            <div className="space-y-5">
              {/* VietQR Image */}
              <div className="flex flex-col items-center justify-center rounded-xl border border-border-subtle bg-white p-4 shadow-sm">
                <img
                  src={order.qr_url}
                  alt="VietQR Transfer Code"
                  className="w-56 h-auto rounded-lg shadow-sm"
                />
                <span className="mt-2 flex items-center gap-1.5 text-center text-sm text-text-muted">
                  <QrCode className="h-4 w-4 shrink-0" aria-hidden="true" />
                  {t(
                    "studio.checkout.scanInstruction",
                    "Scan using any Vietnamese Banking App or Momo",
                  )}
                </span>
              </div>

              {/* Transfer Details Card */}
              <div className="rounded-xl border border-border-subtle bg-surface-muted p-4 space-y-3 text-sm">
                {/* Reference */}
                <div className="flex items-center justify-between gap-3 pb-2 border-b border-border-subtle">
                  <div className="min-w-0">
                    <span className="text-text-muted block text-sm">
                      {t(
                        "studio.checkout.refLabel",
                        "Transfer Content / Description",
                      )}
                    </span>
                    <span className="break-all font-mono text-base font-bold text-primary-accent">
                      {order.reference}
                    </span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(order.reference, "ref")}
                    className="shrink-0"
                  >
                    {copiedField === "ref" ? (
                      <Check
                        className="h-4 w-4 text-success"
                        aria-hidden="true"
                      />
                    ) : (
                      <Copy className="h-4 w-4" aria-hidden="true" />
                    )}
                    {copiedField === "ref"
                      ? t("app.copied", "Copied")
                      : t("app.copy", "Copy")}
                  </Button>
                </div>

                {/* Amount */}
                <div className="flex items-center justify-between gap-3 pb-2 border-b border-border-subtle">
                  <div className="min-w-0">
                    <span className="text-text-muted block text-sm">
                      {t("studio.checkout.amountLabel", "Exact Amount")}
                    </span>
                    <span className="font-mono text-sm font-bold text-text">
                      {formatVnd(order.amount_vnd)}
                    </span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      handleCopy(String(order.amount_vnd), "amount")
                    }
                    className="shrink-0"
                  >
                    {copiedField === "amount" ? (
                      <Check
                        className="h-4 w-4 text-success"
                        aria-hidden="true"
                      />
                    ) : (
                      <Copy className="h-4 w-4" aria-hidden="true" />
                    )}
                    {copiedField === "amount"
                      ? t("app.copied", "Copied")
                      : t("app.copy", "Copy")}
                  </Button>
                </div>

                {/* Bank & Account */}
                <div className="flex items-center justify-between gap-3 pb-2 border-b border-border-subtle">
                  <div className="min-w-0">
                    <span className="text-text-muted block text-sm">
                      {t("studio.checkout.bankLabel", "Bank & Account Number")}
                    </span>
                    <span className="break-all font-mono text-sm font-semibold text-text">
                      {order.bank_code} • {order.account_number}
                    </span>
                    <span className="block break-words text-sm text-text-muted uppercase">
                      {order.account_holder_name}
                    </span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopy(order.account_number, "acc")}
                    className="shrink-0"
                  >
                    {copiedField === "acc" ? (
                      <Check
                        className="h-4 w-4 text-success"
                        aria-hidden="true"
                      />
                    ) : (
                      <Copy className="h-4 w-4" aria-hidden="true" />
                    )}
                    {copiedField === "acc"
                      ? t("app.copied", "Copied")
                      : t("app.copy", "Copy")}
                  </Button>
                </div>
              </div>

              {/* Warning Callout */}
              <div className="flex items-start gap-2.5 rounded-lg border border-warning/30 bg-warning/10 p-3 text-base leading-relaxed text-text">
                <AlertTriangle
                  className="mt-0.5 h-5 w-5 shrink-0 text-warning"
                  aria-hidden="true"
                />
                <p>
                  {t(
                    "studio.checkout.warningContent",
                    "You MUST input the exact Transfer Content code above. Our automated banking webhook reconciles payments within 5–30 seconds.",
                  )}
                </p>
              </div>

              {/* Waiting status with spinner */}
              <div className="flex items-center justify-center gap-2 text-sm text-text-muted pt-1">
                <Loader2
                  className="h-4 w-4 shrink-0 animate-spin text-primary-accent"
                  aria-hidden="true"
                />
                <span>
                  {t(
                    "studio.checkout.waitingForPayment",
                    "Awaiting bank transfer confirmation...",
                  )}
                </span>
              </div>
            </div>
          ) : null}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}
