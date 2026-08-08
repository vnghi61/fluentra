// Fetch API wrapper with X-Request-Id and traceparent interceptors
export interface RequestOptions extends RequestInit {
  headers?: Record<string, string>;
}

export function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, (c) =>
    (
      +c ^
      (crypto.getRandomValues(new Uint8Array(1))[0]! & (15 >> (+c / 4)))
    ).toString(16)
  );
}

export async function apiFetch<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const requestId = generateUUID();
  const traceId = generateUUID().replace(/-/g, '');
  const spanId = generateUUID().replace(/-/g, '').substring(0, 16);
  const traceparent = `00-${traceId}-${spanId}-01`;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Request-Id': requestId,
    'traceparent': traceparent,
    ...options.headers,
  };

  const response = await fetch(endpoint, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let errorBody: unknown;
    try {
      errorBody = await response.json();
    } catch {
      errorBody = { title: response.statusText, status: response.status };
    }
    throw errorBody;
  }

  return response.json() as Promise<T>;
}
