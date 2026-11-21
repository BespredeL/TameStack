export const getErrorMessage = (err: unknown, fallback?: string): string => {
  const message = (err as { message?: unknown } | null | undefined)?.message;
  if (typeof message === 'string' && message) return message;
  return fallback !== undefined ? fallback : String(err);
};