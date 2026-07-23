export function timestamp(value?: number) {
  if (!value) return '-';
  return new Date(value * 1000).toLocaleString();
}

export function uid(prefix: string) {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

export function asArray(value?: string[] | null) {
  return Array.isArray(value) ? value : [];
}

export function splitTags(value: string) {
  return value
    .split(/[\n,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}
