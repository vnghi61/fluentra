import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { apiFetch } from '@/api/client';
import { withSpan } from '@/lib/telemetry';
import { toggleTheme } from '@/lib/theme';

interface PingResponse {
  status: string;
}

/**
 * The home screen exists to prove the whole path works end to end: a click
 * starts a browser span, the fetch instrumentation attaches its `traceparent`,
 * and the API's span becomes a child of this one in the same trace.
 */
export function HomePage(): React.JSX.Element {
  const { t } = useTranslation();

  const ping = useMutation({
    mutationFn: () => withSpan('ui.ping', () => apiFetch<PingResponse>('/api/v1/ping')),
  });

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">{t('app.name')}</h1>
        <p className="text-sm text-slate-400">{t('app.tagline')}</p>
      </header>

      <div className="flex flex-wrap gap-3">
        <button
          type="button"
          onClick={() => ping.mutate()}
          disabled={ping.isPending}
          className="min-h-11 rounded-lg bg-indigo-600 px-4 text-sm font-medium text-white disabled:opacity-60"
        >
          {ping.isPending ? t('status.checking') : t('action.ping')}
        </button>

        <button
          type="button"
          onClick={() => toggleTheme()}
          className="min-h-11 rounded-lg border border-slate-700 px-4 text-sm font-medium"
        >
          {t('action.toggleTheme')}
        </button>
      </div>

      <p role="status" className="text-sm">
        {ping.isSuccess && <span className="text-emerald-400">{t('status.ok')}</span>}
        {ping.isError && <span className="text-red-400">{t('status.failed')}</span>}
      </p>
    </div>
  );
}
