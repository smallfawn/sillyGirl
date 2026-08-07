export type ApiEnvelope<T> = {
  status?: boolean;
  message?: string;
  data: T;
};

export function apiData<T>(response: ApiEnvelope<T> | T): T {
  if (
    response &&
    typeof response === "object" &&
    "data" in (response as Record<string, unknown>)
  ) {
    return (response as ApiEnvelope<T>).data;
  }
  return response as T;
}
