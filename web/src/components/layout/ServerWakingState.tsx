import * as React from "react";
import { useTranslation } from "react-i18next";

import { OrbitBrandMark } from "./OrbitBrandMark";

export function ServerWakingState(): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      className="flex min-h-full flex-1 flex-col items-center justify-center px-4 py-12 text-center"
    >
      <OrbitBrandMark size="md" animated />
      <h1 className="mt-6 text-2xl font-bold leading-tight tracking-tight text-text">
        {t("server.wakingTitle", "Starting the server...")}
      </h1>
      <p className="mt-2 max-w-md text-base leading-relaxed text-text-muted">
        {t(
          "server.wakingDescription",
          "Please wait a moment. The first start can take up to a minute.",
        )}
      </p>
    </div>
  );
}
