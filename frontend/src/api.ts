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
const authExpiresAtKey = 'sillygirl_admin_expires_at';
const adminAuthExpiredEvent = 'sillygirl:admin-auth-expired';
const adminAuthPublicPaths = new Set([
  '/api/admin/login',
  '/api/admin/register',
  '/api/admin/setup/status',
  '/api/admin/outlogin',
]);

let authExpiredNotifiedAt = 0;

export function getAuthToken() {
  return localStorage.getItem(authTokenKey) || '';
}

export function setAuthToken(token: string, expiresIn?: number) {
  if (token) {
    localStorage.setItem(authTokenKey, token);
    if (Number(expiresIn || 0) > 0) {
      localStorage.setItem(authExpiresAtKey, String(Date.now() + Number(expiresIn) * 1000));
    } else {
      localStorage.removeItem(authExpiresAtKey);
    }
  }
}

export function clearAuthToken() {
  localStorage.removeItem(authTokenKey);
  localStorage.removeItem(authExpiresAtKey);
}

function apiPath(url: string) {
  try {
    return new URL(url, window.location.origin).pathname;
  } catch (_) {
    return url.split('?')[0] || '';
  }
}

function isAdminProtectedRequest(url: string) {
  const path = apiPath(url);
  return path.startsWith('/api/admin/') && !adminAuthPublicPaths.has(path);
}

function isTokenExpired(token: string) {
  if (!token) return true;
  const expiresAt = Number(localStorage.getItem(authExpiresAtKey) || 0);
  if (!expiresAt) return true;
  return expiresAt <= Date.now() + 10_000;
}

function handleAdminAuthExpired() {
  clearAuthToken();
  window.dispatchEvent(new CustomEvent(adminAuthExpiredEvent));
  if (window.location.pathname.startsWith('/admin') && window.location.pathname !== '/admin/') {
    window.history.replaceState({}, '', '/admin/');
  }
  const now = Date.now();
  if (now - authExpiredNotifiedAt > 3000) {
    authExpiredNotifiedAt = now;
    message.warning('登录已过期，请重新登录');
  }
}

export async function request<T>(url: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const body = options.body;
  if (body && !(body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  const token = getAuthToken();
  const adminProtected = isAdminProtectedRequest(url);
  if (adminProtected && isTokenExpired(token)) {
    handleAdminAuthExpired();
    throw new ApiError(401, '登录已过期，请重新登录');
  }
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
  const errorText = typeof data === 'string'
    ? data
    : data?.message || data?.errorMessage || data?.msg || data?.error || res.statusText || '请求失败';
  if (!res.ok) {
    if (adminProtected && res.status === 401) {
      handleAdminAuthExpired();
    }
    throw new ApiError(res.status, errorText);
  }
  if (!options.raw && data && typeof data === 'object' && (data.status === false || data.success === false)) {
    throw new ApiError(200, errorText);
  }
  return data as T;
}

export function get<T>(url: string, options: RequestInit = {}) {
  return request<T>(url, options);
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
  const res = await put<{ data?: { messages?: Record<string, string>; errors?: Record<string, string>; changes?: Record<string, boolean> }; messages?: Record<string, string>; errors?: Record<string, string> }>(
    `/api/admin/storage${query}`,
    updates,
  );
  const payload = res.data || res;
  const errors = payload.errors || {};
  const firstError = Object.values(errors).find(Boolean);
  if (firstError) {
    throw new ApiError(200, firstError);
  }
  const firstMessage = Object.values(payload.messages || {}).find(Boolean);
  if (firstMessage) {
    message.info(firstMessage);
  }
  return res;
}

export function readStorage<T = Record<string, unknown>>(keys: string) {
  return get<{ status: boolean; message: string; data: T }>(`/api/admin/storage?keys=${encodeURIComponent(keys)}`);
}
