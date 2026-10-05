import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Laptop,
  Loader2,
  ShieldAlert,
  Trash2,
  AlertCircle,
  Smartphone,
} from "lucide-react";
import { accountApi, type TrustedDevice } from "../api/accountApi";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatDate,
  isMobileLabel,
  settingsAlert,
  settingsCard,
  settingsCardDesc,
  settingsCardTitle,
} from "./settingsStyles";

interface DevicesListProps {
  onLoggedOut?: (() => void) | undefined;
}

export const DevicesList: React.FC<DevicesListProps> = ({ onLoggedOut }) => {
  const { t, i18n } = useTranslation();
  const [devices, setDevices] = useState<TrustedDevice[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [untrustingId, setUntrustingId] = useState<string | null>(null);
  const [confirmDevice, setConfirmDevice] = useState<TrustedDevice | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;
    async function loadDevices() {
      try {
        const data = await accountApi.listDevices();
        if (isMounted) {
          setDevices(data.devices);
          setIsLoading(false);
        }
      } catch (err: unknown) {
        if (isMounted) {
          setError(
            err instanceof Error
              ? err.message
              : t(
                  "account.failedToLoadTrustedDevices",
                  "Failed to load trusted devices.",
                ),
          );
          setIsLoading(false);
        }
      }
    }

    void loadDevices();
    return () => {
      isMounted = false;
    };
  }, [t]);

  const handleUntrust = async (device: TrustedDevice) => {
    setUntrustingId(device.id);
    setError(null);

    try {
      await accountApi.untrustDevice(device.id);
      setDevices((prev) => prev.filter((d) => d.id !== device.id));
      setConfirmDevice(null);

      // Untrusting the device the caller is on signs them out, and the server
      // marks that device `current` precisely so this does not have to guess.
      if (device.current) {
        onLoggedOut?.();
      }
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : t("account.failedToUntrustDevice", "Failed to untrust device."),
      );
    } finally {
      setUntrustingId(null);
    }
  };

  return (
    <section className={`${settingsCard} space-y-3`}>
      <div className="space-y-1">
        <h3 className={settingsCardTitle}>
          <Laptop className="h-5 w-5 text-primary-accent" aria-hidden />
          {t("account.trustedDevices", "Trusted Devices")}
        </h3>
        <p className={settingsCardDesc}>
          {t(
            "account.trustedDevicesDesc",
            'Devices where you chose "Stay signed in". They stay signed in for up to 90 days.',
          )}
        </p>
      </div>

      {error && (
        <div role="alert" className={settingsAlert("error")}>
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          <span>{error}</span>
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-14 w-full rounded-xl" />
      ) : devices.length === 0 ? (
        <p className="text-sm text-text-muted">
          {t("account.noTrustedDevicesFound", "No trusted devices found.")}
        </p>
      ) : (
        <ul className="divide-y divide-border-subtle rounded-xl border border-border-subtle">
          {devices.map((device) => {
            const Icon = isMobileLabel(device.label) ? Smartphone : Laptop;
            return (
              <li
                key={device.id}
                className="flex items-center gap-3 px-3 py-3 sm:px-4"
              >
                <span
                  className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-text-muted"
                  aria-hidden="true"
                >
                  <Icon className="h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
                    <p className="truncate text-sm font-medium text-text">
                      {device.label ||
                        t("account.unknownDevice", "Unknown Device")}
                    </p>
                    {device.current && (
                      <span className="shrink-0 rounded-full border border-primary/30 bg-primary/10 px-2 py-0.5 text-[11px] font-semibold text-primary-accent">
                        {t("account.thisDevice", "This device")}
                      </span>
                    )}
                  </div>
                  <p className="truncate text-xs text-text-muted">
                    {t("account.lastSeenOn", {
                      date: formatDate(device.last_seen_at, i18n.language),
                      defaultValue: `Last seen ${formatDate(device.last_seen_at, i18n.language)}`,
                    })}
                    {" · "}
                    {t("account.expiresOn", {
                      date: formatDate(device.idle_expires_at, i18n.language),
                      defaultValue: `Expires ${formatDate(device.idle_expires_at, i18n.language)}`,
                    })}
                  </p>
                </div>

                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setConfirmDevice(device)}
                  // Icon-only: the `sm` size gives 44 px of height but only
                  // 40 px of width, so the square hit area R1 asks for has to
                  // be stated explicitly.
                  className="min-w-11 shrink-0 text-text-muted hover:bg-danger/10 hover:text-danger-accent"
                >
                  <Trash2 className="h-4 w-4" aria-hidden />
                  <span className="sr-only">
                    {t("account.untrustDevice", "Untrust device")}
                  </span>
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      {/* Untrust Confirmation Modal */}
      {confirmDevice && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/80 backdrop-blur-sm p-4"
        >
          <div className="w-full max-w-md rounded-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-4">
            <div className="flex items-center gap-3 text-danger-accent">
              <ShieldAlert className="h-6 w-6" />
              <h4 className="text-base font-semibold text-text">
                {t(
                  "account.stopTrustingThisDevice",
                  "Stop trusting this device?",
                )}
              </h4>
            </div>

            <p className="text-sm leading-relaxed text-text-muted">
              {t("account.untrustDeviceBody", {
                device:
                  confirmDevice.label ||
                  t("account.thisDeviceLower", "this device"),
                defaultValue: `Untrusting ${confirmDevice.label || "this device"} immediately revokes its persistent session.`,
              })}
            </p>

            <div className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-xs text-warning-accent">
              {confirmDevice.current ? (
                <strong>
                  {t(
                    "account.untrustCurrentWarning",
                    "This is the device you are using right now. Untrusting it will sign you out of this browser immediately.",
                  )}
                </strong>
              ) : (
                <span>
                  {t(
                    "account.untrustOtherWarning",
                    "This device will stop being signed in. The device you are using now is unaffected.",
                  )}
                </span>
              )}
            </div>

            <div className="flex items-center justify-end gap-3 pt-2">
              <Button
                type="button"
                variant="ghost"
                onClick={() => setConfirmDevice(null)}
                disabled={!!untrustingId}
              >
                {t("account.cancel", "Cancel")}
              </Button>
              <Button
                type="button"
                variant="destructive"
                onClick={() => {
                  void handleUntrust(confirmDevice);
                }}
                disabled={!!untrustingId}
              >
                {untrustingId ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t("account.untrusting", "Untrusting...")}
                  </>
                ) : (
                  t("account.untrustSignOut", "Untrust & Sign Out")
                )}
              </Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
};
