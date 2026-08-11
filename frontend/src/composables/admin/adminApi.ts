export type ApiEnvelope<T> = {
  status: boolean;
  message: string;
  data: T;
};

export function apiData<T>(response: ApiEnvelope<T>): T {
  return response.data;
}
