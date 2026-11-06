/// <reference types="vite/client" />

/**
 * Typed environment. Without this `import.meta.env['X']` is `any`, and the
 * type-aware lint rules correctly object: an untyped read of configuration is
 * how a misspelled variable becomes a silent `undefined` at run time.
 */
interface ImportMetaEnv {
  /** OTLP/HTTP traces endpoint. Absent means browser tracing stays off. */
  readonly VITE_OTEL_ENDPOINT?: string;
  /** GA4 measurement id (G-XXXXXXXXXX). Absent means no analytics; read only in production builds. */
  readonly VITE_GA_MEASUREMENT_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv & { readonly MODE: string };
}
