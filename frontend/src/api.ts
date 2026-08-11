import { message } from "ant-design-vue";

export class ApiError extends Error {
  status: number;
  data: unknown;

  constructor(status: number, text: string, data: unknown = null) {
    super(text);
    this.status = status;
    this.data = data;
  }
}

type RequestOptions = RequestInit & { raw?: boolean };

type ApiEnvelope<T> = {
  status: boolean;
  message: string;
  data: T;
};

const authTokenKey = "sillygirl_admin_jwt";
const adminAuthExpiredEvent = "sillygirl:admin-auth-expired";
const adminAuthPublicPaths = new Set([
  "/api/admin/sessions",
  "/api/admin/setup",
]);

let authExpiredNotifiedAt = 0;

export function setAuthToken(token: string, _expiresIn?: number) {
  localStorage.setItem(authTokenKey, token.trim());
}

export function clearAuthToken() {
  localStorage.removeItem(authTokenKey);
}

export function getAuthToken() {
  return localStorage.getItem(authTokenKey)?.trim() || "";
}

function apiPath(url: string) {
  try {
    return new URL(url, window.location.origin).pathname;
  } catch (_) {
    return url.split("?")[0] || "";
  }
}

function isAdminProtectedRequest(url: string) {
  const path = apiPath(url);
  return path.startsWith("/api/admin/") && !adminAuthPublicPaths.has(path);
}

function handleAdminAuthExpired() {
  clearAuthToken();
  window.dispatchEvent(new CustomEvent(adminAuthExpiredEvent));
  if (
    window.location.pathname.startsWith("/admin") &&
    window.location.pathname !== "/admin/"
  ) {
    window.history.replaceState({}, "", "/admin/");
  }
  const now = Date.now();
  if (now - authExpiredNotifiedAt > 3000) {
    authExpiredNotifiedAt = now;
    message.warning("登录已过期，请重新登录");
  }
}

function isApiEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  if (!value || typeof value !== "object") return false;
  const envelope = value as Record<string, unknown>;
  return (
    typeof envelope.status === "boolean" &&
    typeof envelope.message === "string" &&
    "data" in envelope
  );
}

function envelopeErrorText(envelope: ApiEnvelope<unknown>) {
  if (envelope.data && typeof envelope.data === "object") {
    const errors = (envelope.data as Record<string, unknown>).errors;
    const candidates = Array.isArray(errors)
      ? errors
      : errors && typeof errors === "object"
        ? Object.values(errors)
        : [];
    for (const candidate of candidates) {
      if (typeof candidate === "string" && candidate.trim()) {
        return candidate.trim();
      }
      if (candidate && typeof candidate === "object") {
        const detail = (candidate as Record<string, unknown>).message;
        if (typeof detail === "string" && detail.trim()) {
          return detail.trim();
        }
      }
    }
  }
  return envelope.message || "请求失败";
}

export async function request<T>(
  url: string,
  options: RequestOptions = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  const body = options.body;
  if (body && !(body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const token = getAuthToken();
  if (token && !headers.has("token")) {
    headers.set("token", token);
  }
  const adminProtected = isAdminProtectedRequest(url);
  const res = await fetch(url, {
    ...options,
    headers,
  });
  const contentType = res.headers.get("content-type") || "";
  const isJSON = contentType.includes("/json") || contentType.includes("+json");
  const data = isJSON ? await res.json().catch(() => null) : await res.text();
  const errorText =
    typeof data === "string"
      ? data
      : isApiEnvelope(data)
        ? envelopeErrorText(data)
        : res.statusText || "服务响应格式错误";
  if (adminProtected && res.status === 401) {
    handleAdminAuthExpired();
  }
  if (options.raw) {
    if (!res.ok) {
      throw new ApiError(
        res.status,
        errorText,
        isApiEnvelope(data) ? data.data : null,
      );
    }
    return data as T;
  }
  if (!isApiEnvelope(data)) {
    throw new ApiError(res.status, "服务响应格式错误");
  }
  if (!res.ok || !data.status) {
    throw new ApiError(res.status, errorText, data.data);
  }
  return data as T;
}

export function get<T>(url: string, options: RequestInit = {}) {
  return request<T>(url, options);
}

export function post<T>(url: string, data?: unknown) {
  return request<T>(url, {
    method: "POST",
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

export async function saveStorage(
  updates: Record<string, unknown>,
  uuid?: string,
) {
  const query = uuid ? `?uuid=${encodeURIComponent(uuid)}` : "";
  const res = await post<
    ApiEnvelope<{
      messages?: Record<string, string>;
      errors?: Record<string, string>;
      changes?: Record<string, boolean>;
    }>
  >(`/api/admin/storage/values${query}`, updates);
  const payload = res.data;
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
