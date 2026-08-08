import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEventDefault from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { I18nextProvider } from 'react-i18next';
import { describe, expect, it } from 'vitest';

import i18n, { initI18n } from '@/i18n';
import { HomePage } from '@/routes/HomePage';

import { server } from './msw-server';

function renderHome() {
  initI18n('en');
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <HomePage />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

describe('HomePage', () => {
  it('renders the translated tagline', () => {
    renderHome();
    expect(screen.getByText('Learn English that stays learned')).toBeInTheDocument();
  });

  // The point of this one is that MSW is actually intercepting: setup.ts fails
  // the run on an unhandled request, so a passing test proves the request left
  // the client and was matched.
  it('reports success when the API answers', async () => {
    const user = userEventDefault.setup();
    renderHome();
    await user.click(screen.getByRole('button', { name: 'Ping the API' }));
    expect(await screen.findByText('API reachable')).toBeInTheDocument();
  });

  it('reports failure when the API returns a problem document', async () => {
    server.use(
      http.get('/api/v1/ping', () =>
        HttpResponse.json({ title: 'Unavailable', status: 503 }, { status: 503 }),
      ),
    );
    const user = userEventDefault.setup();
    renderHome();
    await user.click(screen.getByRole('button', { name: 'Ping the API' }));
    expect(await screen.findByText('API unreachable')).toBeInTheDocument();
  });
});
