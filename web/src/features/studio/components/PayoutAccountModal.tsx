import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { Building2, CheckCircle2, ShieldCheck, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  usePayoutAccount,
  useUpsertPayoutAccount,
} from "../hooks/useStudio";

interface PayoutAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const COMMON_BANKS = [
  { code: "VCB", name: "Vietcombank (Ngân hàng Ngoại thương)" },
  { code: "MB", name: "MB Bank (Ngân hàng Quân Đội)" },
  { code: "TCB", name: "Techcombank (Ngân hàng Kỹ Thương)" },
  { code: "ACB", name: "ACB (Ngân hàng Á Châu)" },
  { code: "BIDV", name: "BIDV (Ngân hàng Đầu tư và Phát triển)" },
  { code: "CTG", name: "VietinBank (Ngân hàng Công Thương)" },
  { code: "TPB", name: "TPBank (Ngân hàng Tiên Phong)" },
  { code: "VPB", name: "VPBank (Ngân hàng Việt Nam Thịnh Vượng)" },
  { code: "STB", name: "Sacombank (Ngân hàng Sài Gòn Thương Tín)" },
  { code: "HDB", name: "HDBank (Ngân hàng Phát triển TP.HCM)" },
];

export function PayoutAccountModal({
  isOpen,
  onClose,
}: PayoutAccountModalProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const { data: existingAccount } = usePayoutAccount();
  const upsertMutation = useUpsertPayoutAccount();

  const [bankCode, setBankCode] = useState("MB");
  const [accountNumber, setAccountNumber] = useState("");
  const [accountHolder, setAccountHolder] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Seed existing account data during render rather than from an effect
  const [seededAccountId, setSeededAccountId] = useState<string | null>(null);
  if (existingAccount && (existingAccount.account_number ?? "") !== seededAccountId) {
    setSeededAccountId(existingAccount.account_number ?? "");
    if (existingAccount.bank_code) setBankCode(existingAccount.bank_code);
    if (existingAccount.account_number)
      setAccountNumber(existingAccount.account_number);
    if (existingAccount.account_holder_name)
      setAccountHolder(existingAccount.account_holder_name);
  }

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(false);

    if (!accountNumber.trim()) {
      setError(
        t(
          "studio.payout.errAccountNumberRequired",
          "Account number is required",
        ),
      );
      return;
    }
    if (!accountHolder.trim()) {
      setError(
        t(
          "studio.payout.errAccountHolderRequired",
          "Account holder name is required",
        ),
      );
      return;
    }

    try {
      await upsertMutation.mutateAsync({
        bank_code: bankCode,
        account_number: accountNumber.trim(),
        account_holder_name: accountHolder.trim().toUpperCase(),
        is_default: true,
      });
      setSuccess(true);
      setTimeout(() => {
        onClose();
        setSuccess(false);
      }, 1200);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : t("studio.payout.errSaveFailed", "Failed to save payout account"),
      );
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="payout-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
    >
      <div className="relative w-full max-w-lg rounded-xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-5">
        <div className="flex items-center justify-between border-b border-border-subtle pb-4">
          <div className="flex items-center gap-2">
            <Building2 className="h-5 w-5 text-primary-accent" />
            <h2
              id="payout-modal-title"
              className="text-lg font-bold text-text"
            >
              {t("studio.payout.accountTitle", "Payout Bank Account")}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("app.close", "Close")}
            className="rounded-lg p-1.5 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex items-start gap-3 rounded-lg border border-primary/20 bg-primary/5 p-3 text-xs text-text-muted">
          <ShieldCheck className="h-5 w-5 shrink-0 text-primary-accent" />
          <p>
            {t(
              "studio.payout.securityNotice",
              "Your bank details are encrypted and securely stored for earnings payout. Account numbers are never logged or publicly exposed (BR-STUDIO-09).",
            )}
          </p>
        </div>

        {error && (
          <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
            {error}
          </div>
        )}

        {success && (
          <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success font-medium">
            <CheckCircle2 className="h-4 w-4 shrink-0" />
            {t(
              "studio.payout.saveSuccess",
              "Payout account saved successfully!",
            )}
          </div>
        )}

        <form onSubmit={(e) => { void handleSubmit(e); }} className="space-y-4">
          <div>
            <label
              htmlFor="bank-code-select"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.payout.bankLabel", "Bank")}
            </label>
            <select
              id="bank-code-select"
              value={bankCode}
              onChange={(e) => setBankCode(e.target.value)}
              className="w-full rounded-lg border border-border-subtle bg-surface-base px-3 py-2 text-base sm:text-sm text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            >
              {COMMON_BANKS.map((bank) => (
                <option key={bank.code} value={bank.code}>
                  {bank.code} - {bank.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label
              htmlFor="account-number-input"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.payout.accountNumberLabel", "Account Number")}
            </label>
            <Input
              id="account-number-input"
              type="text"
              value={accountNumber}
              onChange={(e) => setAccountNumber(e.target.value)}
              placeholder="e.g. 1017588888"
              required
              className="w-full"
            />
          </div>

          <div>
            <label
              htmlFor="account-holder-input"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.payout.accountHolderLabel", "Account Holder Name")}
            </label>
            <Input
              id="account-holder-input"
              type="text"
              value={accountHolder}
              onChange={(e) => setAccountHolder(e.target.value.toUpperCase())}
              placeholder="e.g. NGUYEN VAN A"
              required
              className="w-full uppercase font-medium"
            />
            <span className="text-[11px] text-text-muted mt-1 block">
              {t(
                "studio.payout.accountHolderHelp",
                "Unaccented uppercase letters matching your bank account.",
              )}
            </span>
          </div>

          <div className="flex justify-end gap-3 pt-3 border-t border-border-subtle">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={upsertMutation.isPending}
            >
              {t("app.cancel", "Cancel")}
            </Button>
            <Button
              type="submit"
              variant="primary"
              disabled={upsertMutation.isPending}
            >
              {upsertMutation.isPending
                ? t("app.saving", "Saving...")
                : t("studio.payout.saveButton", "Save Account")}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
