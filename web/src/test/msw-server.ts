import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';

// Default MSW API handlers matching OpenAPI contracts
export const handlers = [
  http.get('/api/v1/ping', () => {
    return HttpResponse.json({ status: 'ok', timestamp: new Date().toISOString() });
  }),
];

export const server = setupServer(...handlers);
