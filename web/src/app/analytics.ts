import type { AnyRouter } from "@tanstack/react-router";

const MEASUREMENT_ID = /^G-[A-Z0-9]{4,20}$/;
const NOT_FOUND_PATH = "/404";

interface AnalyticsEnv {
  readonly PROD: boolean;
  readonly VITE_GA_MEASUREMENT_ID?: string;
}

/** The measurement id to load, or null for dev, tests and builds without one. */
export function analyticsMeasurementId(env: AnalyticsEnv): string | null {
  const id = env.VITE_GA_MEASUREMENT_ID?.trim() ?? "";
  if (!env.PROD || !MEASUREMENT_ID.test(id)) return null;
  return id;
}

// Only the route template is sent: queries carry reset tokens and OAuth codes, paths carry learner ids.
function routeTemplate(router: AnyRouter): string {
  const leaf = router.state.matches.at(-1);
  if (leaf === undefined || leaf.routeId === "__root__") return NOT_FOUND_PATH;
  const path: unknown = leaf.fullPath;
  return typeof path === "string" ? path : NOT_FOUND_PATH;
}

function stripQuery(url: string): string {
  if (url === "") return "";
  try {
    const parsed = new URL(url);
    return `${parsed.origin}${parsed.pathname}`;
  } catch {
    return "";
  }
}

/** Loads gtag.js and sends a page_view per resolved path change. Returns a stop function. */
export function startAnalytics(
  router: AnyRouter,
  measurementId: string,
): () => void {
  const dataLayer: unknown[] = ((
    window as { dataLayer?: unknown[] }
  ).dataLayer ??= []);
  function gtag(..._args: unknown[]): void {
    // gtag.js ignores plain arrays; it only processes an Arguments object.
    // eslint-disable-next-line prefer-rest-params
    dataLayer.push(arguments);
  }

  gtag("consent", "default", {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "granted",
  });
  gtag("js", new Date());
  gtag("config", measurementId, {
    send_page_view: false,
    allow_google_signals: false,
    allow_ad_personalization_signals: false,
    page_referrer: stripQuery(document.referrer),
  });

  let lastPathname: string | null = null;
  const track = (pathname: string) => {
    if (pathname === lastPathname) return;
    lastPathname = pathname;
    const pageLocation = `${window.location.origin}${routeTemplate(router)}`;
    gtag("set", { page_location: pageLocation });
    gtag("event", "page_view", { page_location: pageLocation });
  };

  // The first navigation may already have resolved before this runs.
  const resolved = router.state.resolvedLocation;
  if (router.state.status === "idle" && resolved !== undefined) {
    track(resolved.pathname);
  }
  const unsubscribe = router.subscribe("onResolved", (event) => {
    track(event.toLocation.pathname);
  });

  const script = document.createElement("script");
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(measurementId)}`;
  document.head.appendChild(script);

  return unsubscribe;
}
