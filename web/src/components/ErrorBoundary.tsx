import { Component, type ErrorInfo, type ReactNode } from "react";

import { tracer } from "@/lib/telemetry";

import i18n from "@/i18n";

/**
 * Translation for a class component, and for the one component that must never
 * throw. `useTranslation` is a hook and this is a class; the imperative form
 * works, and is wrapped because an error boundary that needs a working i18n
 * subsystem to render its own error message can fail exactly when it is needed.
 * The English default is always the answer of last resort.
 */
function tr(key: string, fallback: string): string {
  try {
    return i18n.isInitialized ? i18n.t(key, fallback) : fallback;
  } catch {
    return fallback;
  }
}

interface Props {
  children: ReactNode;
  fallback?: (error: Error, reset: () => void) => ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * The last line of defence. Without one, a render error unmounts the whole tree
 * and the learner is left staring at a white page with no way back — and,
 * because nothing is recorded, the first anyone hears of it is a support
 * message.
 *
 * The error is attached to a span rather than logged to the console: a console
 * message on someone else's phone helps nobody.
 */
export class ErrorBoundary extends Component<Props, State> {
  override state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    const span = tracer().startSpan("ui.error");
    span.recordException(error);
    span.setAttribute("ui.component_stack", info.componentStack ?? "unknown");
    span.end();
  }

  private readonly reset = (): void => {
    this.setState({ error: null });
  };

  override render(): ReactNode {
    const { error } = this.state;
    if (error === null) return this.props.children;
    if (this.props.fallback) return this.props.fallback(error, this.reset);

    return (
      <div role="alert" className="mx-auto max-w-md p-6 text-center">
        <h1 className="text-lg font-semibold">
          {tr("app.somethingBroke", "Something broke")}
        </h1>
        <p className="mt-2 text-sm text-text-muted">
          {tr("app.thisScreenFailedToRender", "This screen failed to render.")}
        </p>
        <button
          type="button"
          onClick={this.reset}
          className="mt-4 min-h-[44px] rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-fg hover:bg-primary-hover"
        >
          {tr("app.tryAgain", "Try again")}
        </button>
      </div>
    );
  }
}
