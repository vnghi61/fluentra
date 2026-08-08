import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AppShell } from '../components/layout/AppShell';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
});

export const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <AppShell>
        <div className="space-y-6">
          <header>
            <h1 className="text-2xl font-bold text-white">Welcome back to Fluentra</h1>
            <p className="text-slate-400 text-sm">Master English competencies with SRS and AI practice.</p>
          </header>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-2">
              <div className="text-sm font-semibold text-slate-300">Vocabulary Mastered</div>
              <div className="text-3xl font-extrabold text-indigo-400">1,240</div>
            </div>
            <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-2">
              <div className="text-sm font-semibold text-slate-300">Daily Streak</div>
              <div className="text-3xl font-extrabold text-amber-400">14 days</div>
            </div>
            <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl space-y-2">
              <div className="text-sm font-semibold text-slate-300">Target CEFR Level</div>
              <div className="text-3xl font-extrabold text-emerald-400">B2 / Upper-Int</div>
            </div>
          </div>
        </div>
      </AppShell>
    </QueryClientProvider>
  );
};
