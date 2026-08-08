import { Component, type ErrorInfo, type ReactNode } from 'react';

import { tracer } from '@/lib/telemetry';

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
    const span = tracer().startSpan('ui.error');
    span.recordException(error);
    span.setAttribute('ui.component_stack', info.componentStack ?? 'unknown');
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
        <h1 className="text-lg font-semibold">Something broke</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
          This screen failed to render.
        </p>
        <button
          type="button"
          onClick={this.reset}
          className="mt-4 rounded-md bg-slate-900 px-4 py-2 text-sm text-white dark:bg-slate-100 dark:text-slate-900"
        >
          Try again
        </button>
      </div>
    );
  }
}
