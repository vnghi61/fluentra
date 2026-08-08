import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach, beforeAll } from 'vitest';

import { server } from './msw-server';

// `error` rather than `warn`: an unmocked request in a test means the test is
// silently talking to nothing, and a warning is easy to miss in CI output.
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
