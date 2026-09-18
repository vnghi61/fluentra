/**
 * Makes a presigned storage URL reachable from the device the page is open on.
 *
 * In development the API signs URLs for the object store it talks to —
 * `http://localhost:9000`, or `http://minio:9000` inside compose. A browser on
 * the same machine reaches that. A phone opening the dev server over the LAN
 * does not: `localhost` on the phone is the phone, so every recording upload,
 * listening clip and recorded pronunciation failed there while the same page
 * worked on the laptop. And once the page is served over HTTPS (which a phone
 * needs before it will open the microphone), an `http://` store is mixed content
 * the browser blocks outright.
 *
 * So, off the machine, a development store URL is rewritten to the dev server's
 * own origin under `/__storage`, which Vite proxies to the store with the host
 * the URL was signed for. The signature covers the path and query, both kept
 * byte for byte. Production URLs (R2, S3) are public hosts and pass through.
 */

export const STORAGE_PROXY_PREFIX = "/__storage";

const DEV_STORE_HOSTS = new Set(["localhost", "127.0.0.1", "minio"]);

function isLocalPage(hostname: string): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

export function reachableStorageUrl(
  url: string,
  page: Pick<Location, "hostname" | "origin"> | undefined = typeof window ===
  "undefined"
    ? undefined
    : window.location,
): string {
  if (!page) return url;
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return url;
  }
  if (!DEV_STORE_HOSTS.has(parsed.hostname)) return url;

  if (isLocalPage(page.hostname) && parsed.protocol === "http:") {
    // Same machine: the store is reachable directly, except by its compose name.
    return parsed.hostname === "minio"
      ? url.replace("//minio:", "//localhost:")
      : url;
  }
  return `${page.origin}${STORAGE_PROXY_PREFIX}${parsed.pathname}${parsed.search}`;
}
