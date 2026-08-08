export function timestamp(value?: number) {
  if (!value) return "-";
  return new Date(value * 1000).toLocaleString();
}
