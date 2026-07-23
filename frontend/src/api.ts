import { message } from 'ant-design-vue';

export class ApiError extends Error {
  status: number;

  constructor(status: number, text: string) {
    super(text);
    this.status = status;
  }
}

type RequestOptions = RequestInit & { raw?: boolean };

const authTokenKey = 'sillygirl_admin_jwt';

export function getAuthToken() {
  return localStorage.getItem(authTokenKey) || '';
}

export function setAuthToken(token: string) {
  if (token) {
    localStorage.setItem(authTokenKey, token);
  }
}

export function clearAuthToken() {
  localStorage.removeItem(authTokenKey);
}

export async function request<T>(url: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const body = options.body;
  if (body && !(body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  const token = getAuthToken();
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const res = await fetch(url, {
    credentials: 'include',
    ...options,
    headers,
  });
  const contentType = res.headers.get('content-type') || '';
  const data = contentType.includes('application/json') ? await res.json() : await res.text();
  if (!res.ok) {
    throw new ApiError(res.status, typeof data === 'string' ? data : data?.errorMessage || res.statusText);
  }
  if (!options.raw && data && typeof data === 'object' && data.success === false) {
    throw new ApiError(200, data.errorMessage || '请求失败');
  }
  return data as T;
}

export function get<T>(url: string) {
  return request<T>(url);
}

export function post<T>(url: string, data?: unknown) {
  return request<T>(url, {
    method: 'POST',
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

export function put<T>(url: string, data?: unknown) {
  return request<T>(url, {
    method: 'PUT',
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

export function del<T>(url: string, data?: unknown) {
  return request<T>(url, {
    method: 'DELETE',
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

export async function saveStorage(updates: Record<string, unknown>, uuid?: string) {
  const query = uuid ? `?uuid=${encodeURIComponent(uuid)}` : '';
  const res = await put<{ success: boolean; messages?: Record<string, string>; errors?: Record<string, string> }>(
    `/api/storage${query}`,
    updates,
  );
  const errors = res.errors || {};
  const firstError = Object.values(errors).find(Boolean);
  if (firstError) {
    throw new ApiError(200, firstError);
  }
  const firstMessage = Object.values(res.messages || {}).find(Boolean);
  if (firstMessage) {
    message.info(firstMessage);
  }
  return res;
}

export function readStorage<T = Record<string, unknown>>(keys: string) {
  return get<{ success: boolean; data: T }>(`/api/storage?keys=${encodeURIComponent(keys)}`);
}
