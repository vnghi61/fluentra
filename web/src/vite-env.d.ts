/// <reference types="vite/client" />

/**
 * Typed environment. Without this `import.meta.env['X']` is `any`, and the
 * type-aware lint rules correctly object: an untyped read of configuration is
 * how a misspelled variable becomes a silent `undefined` at run time.
 */
interface ImportMetaEnv {
  /** OTLP/HTTP traces endpoint. Absent means browser tracing stays off. */
  readonly VITE_OTEL_ENDPOINT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv & { readonly MODE: string };
}
