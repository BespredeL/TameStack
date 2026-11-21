import { useCallback, useEffect, useRef, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import type { DependencyList } from 'react';
import { useServerRequest } from './useServerRequest';
import { useServer } from '../context/ServerContext';

export interface UseServerDataOptions<T> {
  initial: T;
  reset?: T;
  loadingMessage?: string;
  enabled?: boolean;
  clearError?: boolean;
  onStart?: () => void;
  onSuccess?: (data: T) => void;
  onError?: (message: string) => void;
}

// Loads server-scoped data with a latest-wins ticket guard, loading/error state
// and page-loading integration. Re-fetches when selectedServerId or deps change;
// reload() re-runs the current fetcher on demand (e.g. after a mutation).
export function useServerData<T>(
  fetcher: (serverId: string) => Promise<T>,
  deps: DependencyList,
  options: UseServerDataOptions<T>,
) {
  const { selectedServerId, setPageLoading } = useServer();
  const req = useServerRequest();
  const [data, setData] = useState<T>(options.initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ref = useRef({ fetcher, options, req });
  ref.current = { fetcher, options, req };

  const reload = useCallback(async () => {
    const { fetcher: fn, options: opts, req: guard } = ref.current;
    const serverId = selectedServerId;
    if (!serverId || opts.enabled === false) return;
    const gen = guard.next();
    if ('reset' in opts) setData(opts.reset as T);
    if (opts.clearError !== false && !opts.onError) setError(null);
    setLoading(true);
    if (opts.loadingMessage) setPageLoading(true, opts.loadingMessage);
    opts.onStart?.();
    try {
      const res = await fn(serverId);
      if (!guard.isCurrent(gen)) return;
      setData(res);
      opts.onSuccess?.(res);
    } catch (err: unknown) {
      if (!guard.isCurrent(gen)) return;
      const msg = getErrorMessage(err);
      if (opts.onError) opts.onError(msg);
      else setError(msg);
    } finally {
      if (guard.isCurrent(gen)) {
        setLoading(false);
        if (opts.loadingMessage) setPageLoading(false);
      }
    }
  }, [selectedServerId, setPageLoading]);

  useEffect(() => {
    reload();
  }, [reload, ...deps]);

  return { data, loading, error, reload, setData, setError };
}