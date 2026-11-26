import React, { useEffect, useState, useRef } from 'react';
import { CheckCircle2, AlertCircle, Clock, Loader2, Ban, Terminal } from 'lucide-react';
import { GetTaskLogs, CancelTask } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { TaskRecord, TaskLogRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { ModalShell } from './common/ModalShell';
import { localizeTaskTitle, localizeTaskStep } from '../utils/taskI18n';
import { Button, Badge } from './ui';

interface TaskModalProps {
  task: TaskRecord | null;
  onClose: () => void;
}

export const TaskModal: React.FC<TaskModalProps> = ({ task, onClose }) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const [logs, setLogs] = useState<TaskLogRecord[]>([]);
  const [currentTask, setCurrentTask] = useState<TaskRecord | null>(task);
  const [isCancelling, setIsCancelling] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const nextTempLogIdRef = useRef<number>(-1);

  const formatTimeSafe = (dateStr?: string | null) => {
    if (!dateStr || dateStr.startsWith('0001-01-01')) return '';
    try {
      const d = new Date(dateStr);
      return isNaN(d.getTime()) ? '' : d.toLocaleTimeString();
    } catch {
      return '';
    }
  };

  useEffect(() => {
    setCurrentTask(task);
    if (!task) return;

    // Load initial logs from DB
    GetTaskLogs(task.id)
      .then((l) => setLogs(l as TaskLogRecord[]))
      .catch(console.error);

    // Listen to real-time events for this task
    const cleanupLog = EventsOn('task:log', (data: any) => {
      if (data && data.task_id === task.id) {
        const tempId = nextTempLogIdRef.current--;
        setLogs((prev) => [
          ...prev,
          {
            id: tempId,
            task_id: data.task_id,
            level: data.level,
            message: data.message,
            created_at: data.time || new Date().toISOString(),
          },
        ]);
      }
    });

    const cleanupProgress = EventsOn('task:progress', (data: any) => {
      if (data && data.task_id === task.id) {
        setCurrentTask((prev) =>
          prev ? { ...prev, current_step: data.step, progress: data.progress } : null
        );
      }
    });

    const cleanupFinished = EventsOn('task:finished', (data: any) => {
      if (data && data.id === task.id) {
        setCurrentTask(data as TaskRecord);
      }
    });

    return () => {
      if (typeof cleanupLog === 'function') cleanupLog();
      if (typeof cleanupProgress === 'function') cleanupProgress();
      if (typeof cleanupFinished === 'function') cleanupFinished();
    };
  }, [task]);

  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs]);

  if (!currentTask) return null;

  const handleCancel = async () => {
    if (!currentTask) return;
    if (!(await confirm(t('tasks.modal.abortConfirm', { title: localizeTaskTitle(currentTask, t) }), { danger: true }))) {
      return;
    }
    setIsCancelling(true);
    try {
      await CancelTask(currentTask.id);
    } catch (err) {
      console.error(err);
    } finally {
      setIsCancelling(false);
    }
  };

  const getStatusBadge = () => {
    switch (currentTask.status) {
      case 'completed':
        return (
          <Badge variant="success" size="sm" className="inline-flex items-center gap-1.5">
            <CheckCircle2 className="w-3.5 h-3.5" /> {t('tasks.modal.statusCompleted')}
          </Badge>
        );
      case 'running':
        return (
          <Badge variant="primary" size="sm" className="inline-flex items-center gap-1.5 animate-pulse">
            <Loader2 className="w-3.5 h-3.5 animate-spin" /> {t('tasks.modal.statusRunning', { progress: currentTask.progress })}
          </Badge>
        );
      case 'failed':
        return (
          <Badge variant="danger" size="sm" className="inline-flex items-center gap-1.5">
            <AlertCircle className="w-3.5 h-3.5" /> {t('tasks.modal.statusFailed')}
          </Badge>
        );
      case 'cancelled':
        return (
          <Badge variant="neutral" size="sm" className="inline-flex items-center gap-1.5">
            <Ban className="w-3.5 h-3.5" /> {t('tasks.modal.statusCancelled')}
          </Badge>
        );
      default:
        return (
          <Badge variant="warning" size="sm" className="inline-flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5" /> {t('tasks.modal.statusQueued')}
          </Badge>
        );
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="3xl"
      className="max-h-[85vh]"
      icon={<Terminal className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={localizeTaskTitle(currentTask, t)}
      headerExtra={
        <Badge variant="neutral" size="sm" className="font-mono">
          {currentTask.operation_type}
        </Badge>
      }
      subtitle={t('tasks.modal.taskId', { id: currentTask.id })}
      headerActions={getStatusBadge()}
    >
      {/* Progress Bar & Current Step */}
      <div className="px-6 py-3 bg-slate-950/40 border-b border-slate-800 space-y-1.5">
        <div className="flex justify-between text-xs">
          <span className="text-slate-300 font-medium">{localizeTaskStep(currentTask.current_step, t) || t('tasks.modal.initializingStep')}</span>
          <span className="text-indigo-400 font-mono font-bold">{currentTask.progress}%</span>
        </div>
        <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-300 ${
              currentTask.status === 'failed'
                ? 'bg-rose-500'
                : currentTask.status === 'completed'
                ? 'bg-emerald-500'
                : 'bg-indigo-500'
            }`}
            style={{ width: `${currentTask.progress}%` }}
          />
        </div>
      </div>

      {/* Console Log Terminal Output */}
      <div
        ref={logContainerRef}
        className="flex-1 overflow-y-auto p-4 bg-slate-950 font-mono text-xs text-slate-300 space-y-1 select-text min-h-[300px] custom-scrollbar"
      >
        {currentTask.status === 'failed' && currentTask.error_message && (
          <div className="p-3 mb-3 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300">
            <div className="font-bold text-xs uppercase mb-1 flex items-center gap-1.5">
              <AlertCircle className="w-4 h-4 text-rose-400" />
              <span>{t('tasks.modal.execError')}</span>
            </div>
            <div className="text-[11px] leading-relaxed break-all whitespace-pre-wrap">{currentTask.error_message}</div>
          </div>
        )}

        {logs.length === 0 ? (
          <div className="text-slate-500 italic p-4 text-center">{t('tasks.modal.emptyLog')}</div>
        ) : (
          logs.map((log, idx) => (
            <div key={idx} className="flex items-start gap-2 leading-relaxed">
              <span className="text-slate-600 select-none text-[11px]">
                {formatTimeSafe(log.created_at)}
              </span>
              <span
                className={`px-1 py-0.2 rounded text-[10px] uppercase font-bold shrink-0 ${
                  log.level === 'error'
                    ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
                    : log.level === 'warn'
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                    : 'bg-indigo-500/10 text-indigo-300'
                }`}
              >
                {log.level}
              </span>
              <span className="break-all whitespace-pre-wrap">{log.message}</span>
            </div>
          ))
        )}
      </div>

      {/* Footer */}
      <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <div className="text-xs text-slate-400">
          {currentTask.started_at && formatTimeSafe(currentTask.started_at) ? t('tasks.modal.startedAt', { time: formatTimeSafe(currentTask.started_at) }) : ''}
          {currentTask.completed_at && formatTimeSafe(currentTask.completed_at) ? t('tasks.modal.completedAt', { time: formatTimeSafe(currentTask.completed_at) }) : ''}
        </div>

        <div className="flex items-center gap-3">
          {currentTask.status === 'running' && (
            <Button
              variant="danger"
              size="sm"
              onClick={handleCancel}
              loading={isCancelling}
              icon={<Ban className="w-3.5 h-3.5" />}
            >
              {t('tasks.modal.cancelBtn')}
            </Button>
          )}
          <Button
            variant="secondary"
            size="sm"
            onClick={onClose}
          >
            {t('common.close')}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
};
