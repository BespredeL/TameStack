import { useEffect, useState } from 'react';
import { ServerRecord, DiagnosticsRecord, TaskRecord, DashboardStats, TaskProgressPayload } from '../types';
import { ListServers, GetDiagnostics, GetDashboardStats, ListTasks } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

export const useAppData = () => {
  const [servers, setServers] = useState<ServerRecord[]>([]);
  const [diagnosticsMap, setDiagnosticsMap] = useState<Record<string, DiagnosticsRecord>>({});
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [tasks, setTasks] = useState<TaskRecord[]>([]);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [serversStatus, setServersStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [serversError, setServersError] = useState<string | null>(null);

  const fetchData = async () => {
    setIsRefreshing(true);
    setServersError(null);
    try {
      const fetchedServers = await ListServers();
      const sList = (fetchedServers || []) as ServerRecord[];
      setServers(sList);
      setServersStatus('ready');

      try {
        const fetchedStats = await GetDashboardStats();
        setStats(fetchedStats as DashboardStats);
      } catch (err) {
        console.error('Failed to fetch dashboard stats:', err);
      }
      try {
        const fetchedTasks = await ListTasks(50);
        setTasks((fetchedTasks || []) as TaskRecord[]);
      } catch (err) {
        console.error('Failed to fetch tasks:', err);
      }

      // Fetch diagnostics for each server
      const diagMap: Record<string, DiagnosticsRecord> = {};
      await Promise.all(
        sList.map(async (s) => {
          try {
            const diag = await GetDiagnostics(s.id);
            if (diag && diag.server_id) {
              diagMap[s.id] = diag as DiagnosticsRecord;
            }
          } catch (e) {
            // No diagnostics yet
          }
        })
      );
      setDiagnosticsMap(diagMap);
    } catch (err) {
      console.error('Failed to fetch data:', err);
      setServersStatus('error');
      setServersError(err instanceof Error ? err.message : String(err));
    } finally {
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    const unCreated = EventsOn('task:created', (newTask: TaskRecord) => {
      setTasks((prev) => (prev.some((t) => t.id === newTask.id) ? prev : [newTask, ...prev]));
    });

    const unStarted = EventsOn('task:started', (startedTask: TaskRecord) => {
      setTasks((prev) =>
        prev.map((t) => (t.id === startedTask.id ? startedTask : t))
      );
    });

    const unProgress = EventsOn('task:progress', (data: TaskProgressPayload) => {
      setTasks((prev) =>
        prev.map((t) =>
          t.id === data.task_id ? { ...t, current_step: data.step, progress: data.progress } : t
        )
      );
    });

    const unFinished = EventsOn('task:finished', (finishedTask: TaskRecord) => {
      setTasks((prev) =>
        prev.map((t) => (t.id === finishedTask.id ? finishedTask : t))
      );
      // Re-fetch servers and stats to update online status & diagnostics
      fetchData();
    });

    return () => {
      if (typeof unCreated === 'function') unCreated();
      else EventsOff('task:created');

      if (typeof unStarted === 'function') unStarted();
      else EventsOff('task:started');

      if (typeof unProgress === 'function') unProgress();
      else EventsOff('task:progress');

      if (typeof unFinished === 'function') unFinished();
      else EventsOff('task:finished');
    };
  }, []);

  return {
    servers,
    diagnosticsMap,
    stats,
    tasks,
    isRefreshing,
    serversStatus,
    serversError,
    fetchData,
  };
};