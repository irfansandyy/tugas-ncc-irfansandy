export class APIError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

const rawApiBaseUrl = process.env.NEXT_PUBLIC_API_BASE_URL;
export const API_BASE_URL = rawApiBaseUrl && rawApiBaseUrl.trim().length > 0 ? rawApiBaseUrl : "";

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
  token?: string | null
): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) {
        message = payload.error;
      }
    } catch {
      // Keep fallback error message if response body is not JSON.
    }
    throw new APIError(message, response.status);
  }

  return (await response.json()) as T;
}
