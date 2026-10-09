import React, { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Building2, CheckCircle2, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Combobox, type ComboboxOption } from "@/components/ui/combobox";
import { usePayoutAccount, useUpsertPayoutAccount } from "../hooks/useStudio";

interface PayoutAccountModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const COMMON_BANKS = [
  { code: "VCB", name: "Vietcombank (Ngân hàng Ngoại thương)" },
  { code: "CTG", name: "VietinBank (Ngân hàng Công Thương)" },
  { code: "BIDV", name: "BIDV (Ngân hàng Đầu tư và Phát triển)" },
  { code: "VBA", name: "Agribank (Ngân hàng Nông nghiệp)" },
  { code: "MB", name: "MB Bank (Ngân hàng Quân Đội)" },
  { code: "TCB", name: "Techcombank (Ngân hàng Kỹ Thương)" },
  { code: "ACB", name: "ACB (Ngân hàng Á Châu)" },
  { code: "VPB", name: "VPBank (Ngân hàng Việt Nam Thịnh Vượng)" },
  { code: "TPB", name: "TPBank (Ngân hàng Tiên Phong)" },
  { code: "STB", name: "Sacombank (Ngân hàng Sài Gòn Thương Tín)" },
  { code: "HDB", name: "HDBank (Ngân hàng Phát triển TP.HCM)" },
  { code: "VIB", name: "VIB (Ngân hàng Quốc tế)" },
  { code: "SHB", name: "SHB (Ngân hàng Sài Gòn - Hà Nội)" },
  { code: "MSB", name: "MSB (Ngân hàng Hàng Hải)" },
  { code: "OCB", name: "OCB (Ngân hàng Phương Đông)" },
  { code: "LPB", name: "LPBank (Ngân hàng Lộc Phát Việt Nam)" },
  { code: "SSB", name: "SeABank (Ngân hàng Đông Nam Á)" },
  { code: "EIB", name: "Eximbank (Ngân hàng Xuất Nhập Khẩu)" },
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

  const bankOptions = useMemo<ComboboxOption[]>(() => {
    const list: ComboboxOption[] = COMMON_BANKS.map((bank) => ({
      value: bank.code,
      label: `${bank.code} - ${bank.name}`,
      searchTerms: `${bank.code} ${bank.name}`,
    }));
    if (bankCode && !list.some((b) => b.value === bankCode)) {
      list.unshift({
        value: bankCode,
        label: bankCode,
        searchTerms: bankCode,
      });
    }
    return list;
  }, [bankCode]);

  // Seed existing account data during render rather than from an effect
  const [seededAccountId, setSeededAccountId] = useState<string | null>(null);
  if (
    existingAccount &&
    (existingAccount.account_number ?? "") !== seededAccountId
  ) {
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
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !upsertMutation.isPending) onClose();
      }}
    >
      <DialogContent showClose={!upsertMutation.isPending}>
        <DialogHeader>
          <div className="flex items-start gap-3">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent">
              <Building2 className="h-5 w-5" aria-hidden="true" />
            </span>
            <div className="min-w-0 space-y-1">
              <DialogTitle className="break-words">
                {t("studio.payout.accountTitle", "Payout Bank Account")}
              </DialogTitle>
              <DialogDescription>
                {t(
                  "studio.payout.accountDescription",
                  "Add the bank account where you want to receive your earnings.",
                )}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            void handleSubmit(e);
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <DialogBody className="space-y-5">
            <div className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/5 p-3 text-base leading-relaxed text-text-muted">
              <ShieldCheck
                className="mt-0.5 h-5 w-5 shrink-0 text-primary-accent"
                aria-hidden="true"
              />
              <p>
                {t(
                  "studio.payout.securityNotice",
                  "Your bank details are protected and used only to pay your course earnings.",
                )}
              </p>
            </div>

            {error && (
              <div
                role="alert"
                className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger-accent font-medium"
              >
                {error}
              </div>
            )}

            {success && (
              <div
                role="status"
                className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success-accent font-medium"
              >
                <CheckCircle2 className="h-4 w-4 shrink-0" aria-hidden="true" />
                {t(
                  "studio.payout.saveSuccess",
                  "Payout account saved successfully!",
                )}
              </div>
            )}

            <div>
              <label
                htmlFor="bank-code-select"
                className="block text-sm font-semibold text-text mb-2"
              >
                {t("studio.payout.bankLabel", "Bank")}
              </label>
              <input type="hidden" name="bank_code" value={bankCode} />
              <Combobox
                id="bank-code-select"
                value={bankCode}
                options={bankOptions}
                onChange={(next) => setBankCode(next)}
                disabled={upsertMutation.isPending}
                placeholder={t("studio.payout.selectBank", "Select a bank...")}
                searchPlaceholder={t(
                  "studio.payout.searchBank",
                  "Search bank by name or code...",
                )}
                emptyText={t("studio.payout.noBankFound", "No bank found")}
              />
            </div>

            <div>
              <label
                htmlFor="account-number-input"
                className="block text-sm font-semibold text-text mb-2"
              >
                {t("studio.payout.accountNumberLabel", "Account Number")}
              </label>
              <Input
                id="account-number-input"
                type="text"
                inputMode="numeric"
                autoComplete="off"
                value={accountNumber}
                onChange={(e) => setAccountNumber(e.target.value)}
                placeholder={t(
                  "studio.payout.accountNumberPlaceholder",
                  "e.g. 1017588888",
                )}
                required
                disabled={upsertMutation.isPending}
                className="w-full"
              />
            </div>

            <div>
              <label
                htmlFor="account-holder-input"
                className="block text-sm font-semibold text-text mb-2"
              >
                {t("studio.payout.accountHolderLabel", "Account Holder Name")}
              </label>
              <Input
                id="account-holder-input"
                type="text"
                autoComplete="name"
                aria-describedby="account-holder-help"
                value={accountHolder}
                onChange={(e) => setAccountHolder(e.target.value.toUpperCase())}
                placeholder={t(
                  "studio.payout.accountHolderPlaceholder",
                  "e.g. NGUYEN VAN A",
                )}
                required
                disabled={upsertMutation.isPending}
                className="w-full uppercase font-medium"
              />
              <span
                id="account-holder-help"
                className="text-sm leading-relaxed text-text-muted mt-2 block"
              >
                {t(
                  "studio.payout.accountHolderHelp",
                  "Unaccented uppercase letters matching your bank account.",
                )}
              </span>
            </div>
          </DialogBody>
          <DialogFooter className="flex-col-reverse sm:flex-row">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={upsertMutation.isPending}
              className="w-full sm:w-auto"
            >
              {t("app.cancel", "Cancel")}
            </Button>
            <Button
              type="submit"
              variant="primary"
              disabled={upsertMutation.isPending}
              isLoading={upsertMutation.isPending}
              className="w-full sm:w-auto"
            >
              {upsertMutation.isPending
                ? t("app.saving", "Saving...")
                : t("studio.payout.saveButton", "Save Account")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
