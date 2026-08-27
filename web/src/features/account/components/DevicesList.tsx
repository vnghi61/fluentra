import React, { useEffect, useState } from "react";
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

interface DevicesListProps {
  onLoggedOut?: (() => void) | undefined;
}

export const DevicesList: React.FC<DevicesListProps> = ({ onLoggedOut }) => {
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
              : "Failed to load trusted devices.",
          );
          setIsLoading(false);
        }
      }
    }

    void loadDevices();
    return () => {
      isMounted = false;
    };
  }, []);

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
        err instanceof Error ? err.message : "Failed to untrust device.",
      );
    } finally {
      setUntrustingId(null);
    }
  };

  return (
    <div className="rounded-xl border border-border-subtle bg-surface-card/60 p-6 space-y-4">
      <div className="space-y-1">
        <h3 className="text-base font-semibold text-text flex items-center gap-2">
          <Laptop className="h-5 w-5 text-primary-accent" />
          Trusted Devices
        </h3>
        <p className="text-xs text-text-muted">
          Devices you selected &quot;Stay signed in&quot; on. They remain signed
          in for up to 90 days.
        </p>
      </div>

      {error && (
        <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3.5 text-xs text-danger-accent">
          <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-primary-accent" />
        </div>
      ) : devices.length === 0 ? (
        <p className="text-xs text-text-muted py-4 text-center">
          No trusted devices found.
        </p>
      ) : (
        <div className="space-y-3">
          {devices.map((device) => {
            const isMobile =
              device.label?.toLowerCase().includes("mobile") ||
              device.label?.toLowerCase().includes("android") ||
              device.label?.toLowerCase().includes("iphone");

            return (
              <div
                key={device.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-subtle bg-surface-card/40 p-4"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-surface-muted text-text-muted">
                    {isMobile ? (
                      <Smartphone className="h-5 w-5" />
                    ) : (
                      <Laptop className="h-5 w-5" />
                    )}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium text-text">
                        {device.label || "Unknown Device"}
                      </p>
                      {device.current && (
                        <span className="inline-flex items-center rounded-full border border-primary/40 bg-primary/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-primary-accent">
                          This device
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-text-muted mt-0.5">
                      <span>
                        Last seen:{" "}
                        {new Date(device.last_seen_at).toLocaleDateString()}
                      </span>
                      <span>
                        Expires:{" "}
                        {new Date(device.idle_expires_at).toLocaleDateString()}
                      </span>
                    </div>
                  </div>
                </div>

                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setConfirmDevice(device)}
                  // Icon-only: the `sm` size gives 44 px of height but only
                  // 40 px of width, so the square hit area R1 asks for has to
                  // be stated explicitly.
                  className="min-w-11 text-text-muted hover:text-danger-accent hover:bg-danger/10"
                >
                  <Trash2 className="h-4 w-4" />
                  <span className="sr-only">Untrust device</span>
                </Button>
              </div>
            );
          })}
        </div>
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
                Stop trusting this device?
              </h4>
            </div>

            <p className="text-sm text-text-muted leading-relaxed">
              Untrusting{" "}
              <strong className="text-white">
                {confirmDevice.label || "this device"}
              </strong>{" "}
              will immediately revoke its persistent session.
            </p>

            <div className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-xs text-warning-accent">
              {confirmDevice.current ? (
                <strong>
                  This is the device you are using right now. Untrusting it will
                  sign you out of this browser immediately.
                </strong>
              ) : (
                <span>
                  This device will stop being signed in. The device you are
                  using now is unaffected.
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
                Cancel
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
                    Untrusting...
                  </>
                ) : (
                  "Untrust & Sign Out"
                )}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
