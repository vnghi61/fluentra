import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";

import { router } from "@/app/router";
import { initApp } from "@/app/bootstrap";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import i18n from "@/i18n";
import { startTelemetry } from "@/lib/telemetry";

import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 5 * 60 * 1000, retry: 1 },
  },
});

async function bootstrap(): Promise<void> {
  // Boot-time silent refresh before first render, so returning learners never see a login screen
  await initApp();

  const rootElement = document.getElementById("root");
  if (rootElement === null) {
    throw new Error("index.html is missing #root");
  }

  createRoot(rootElement).render(
    <StrictMode>
      <ErrorBoundary>
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
          </QueryClientProvider>
        </I18nextProvider>
      </ErrorBoundary>
    </StrictMode>,
  );
}

void bootstrap();

// Started after the first render is scheduled, so the SDK never delays paint.
// It is also the only place the collector endpoint is read: a build with no
// VITE_OTEL_ENDPOINT simply has no browser tracing, rather than failing.
const endpoint = import.meta.env.VITE_OTEL_ENDPOINT;
if (endpoint !== undefined && endpoint.length > 0) {
  void startTelemetry({
    endpoint,
    serviceName: "fluentra-web",
    environment: import.meta.env.MODE,
  });
}
