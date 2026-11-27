import React, { useState } from 'react';
import {
  ListTodo,
  Ban,
  Terminal,
  RefreshCw,
} from 'lucide-react';
import {
  Button,
  SearchInput,
  Badge,
  Tabs,
  PageToolbar,
  TableContainer,
  Table,
  TableHeader,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  TableEmpty,
} from '../components/ui';
import { TaskRecord, ServerRecord } from '../types';
import { CancelTask } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { usePrivacy } from '../context/PrivacyContext';
import { localizeTaskTitle, localizeTaskStep } from '../utils/taskI18n';

interface TasksViewProps {
  tasks: TaskRecord[];
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export const TasksView: React.FC<TasksViewProps> = ({
  tasks,
  servers,
  onOpenTask,
  onRefresh,
  isRefreshing,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const { maskServerName, maskText } = usePrivacy();
  const [filter, setFilter] = useState<'all' | 'running' | 'completed' | 'failed' | 'cancelled'>('all');
  const [search, setSearch] = useState('');

  const serverNameMap = new Map(servers.map((s) => [s.id, s.name]));

  const filtered = tasks.filter((task) => {
    const matchesFilter = filter === 'all' || task.status === filter;
    const q = search.toLowerCase();
    const matchesSearch =
      localizeTaskTitle(task, t).toLowerCase().includes(q) ||
      task.title.toLowerCase().includes(q) ||
      task.operation_type.toLowerCase().includes(q) ||
      task.id.toLowerCase().includes(q);
    return matchesFilter && matchesSearch;
  });

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'completed': return t('tasks.completed');
      case 'running': return t('tasks.running');
      case 'failed': return t('tasks.failed');
      case 'cancelled': return t('tasks.cancelled');
      case 'pending': return t('tasks.pending');
      default: return status;
    }
  };

  const handleCancel = async (e: React.MouseEvent, taskID: string) => {
    e.stopPropagation();
    if (!(await confirm(t('tasks.cancelConfirm'), { danger: true }))) {
      return;
    }
    try {
      await CancelTask(taskID);
      onRefresh();
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="page space-y-6">
      {/* Top Filter & Action Bar */}
      <PageToolbar
        icon={<ListTodo className="w-5 h-5 text-indigo-400" />}
        title={t('nav.tasks')}
        subtitle={`${tasks.length} ${t('nav.tasks').toLowerCase()}`}
        filters={
          <Tabs
            variant="pill"
            activeTab={filter}
            onChange={(id) => setFilter(id as any)}
            tabs={[
              { id: 'all', label: t('tasks.all') },
              { id: 'running', label: t('tasks.running') },
              { id: 'completed', label: t('tasks.completed') },
              { id: 'failed', label: t('tasks.failed') },
              { id: 'cancelled', label: t('tasks.cancelled') },
            ]}
          />
        }
        search={
          <SearchInput
            placeholder={t('tasks.searchPlaceholder')}
            value={search}
            onChangeValue={setSearch}
            sizeVariant="md"
          />
        }
        actions={
          <Button
            variant="secondary"
            size="icon-md"
            onClick={onRefresh}
            disabled={isRefreshing}
            title={t('tasks.refresh')}
            aria-label={t('tasks.refresh')}
          >
            <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-indigo-400' : ''}`} />
          </Button>
        }
      />

      {/* Task List Table */}
      <TableContainer>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('tasks.status')}</TableHead>
              <TableHead>{t('tasks.titleAndOp')}</TableHead>
              <TableHead>{t('tasks.server')}</TableHead>
              <TableHead>{t('tasks.stepAndProgress')}</TableHead>
              <TableHead>{t('tasks.startedAt')}</TableHead>
              <TableHead className="text-right">{t('tasks.actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length === 0 ? (
              <TableEmpty
                colSpan={6}
                icon={<ListTodo className="w-8 h-8 text-slate-600" />}
                message={t('tasks.noTasks')}
              />
            ) : (
              filtered.map((tRec) => {
                const srvName = tRec.server_id ? maskServerName(serverNameMap.get(tRec.server_id) || tRec.server_id) : t('tasks.systemGlobal');
                const badgeVariant =
                  tRec.status === 'completed'
                    ? 'success'
                    : tRec.status === 'running'
                    ? 'indigo'
                    : tRec.status === 'failed'
                    ? 'danger'
                    : 'neutral';

                return (
                  <TableRow
                    key={tRec.id}
                    onClick={() => onOpenTask(tRec)}
                    className="cursor-pointer"
                  >
                    <TableCell>
                      <Badge
                        variant={badgeVariant}
                        size="xs"
                        dot={tRec.status === 'running'}
                        pulse={tRec.status === 'running'}
                      >
                        {getStatusLabel(tRec.status)}
                      </Badge>
                    </TableCell>

                    <TableCell>
                      <div className="font-semibold text-white text-xs">{maskText(localizeTaskTitle(tRec, t))}</div>
                      <div className="font-mono text-[10px] text-slate-400">{tRec.operation_type}</div>
                    </TableCell>

                    <TableCell className="text-slate-300 font-medium">
                      {srvName}
                    </TableCell>

                    <TableCell className="min-w-[180px]">
                      <div className="flex justify-between text-[11px] mb-1">
                        <span className="text-slate-400 truncate max-w-[140px]">
                          {maskText(localizeTaskStep(tRec.current_step, t) || t('tasks.processing'))}
                        </span>
                        <span className="font-mono text-indigo-400 font-bold">{tRec.progress}%</span>
                      </div>
                      <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${
                            tRec.status === 'failed'
                              ? 'bg-rose-500'
                              : tRec.status === 'completed'
                              ? 'bg-emerald-500'
                              : 'bg-indigo-500'
                          }`}
                          style={{ width: `${tRec.progress}%` }}
                        />
                      </div>
                    </TableCell>

                    <TableCell className="text-slate-400 font-mono text-[11px]">
                      {tRec.started_at ? new Date(tRec.started_at).toLocaleTimeString() : new Date(tRec.created_at).toLocaleTimeString()}
                    </TableCell>

                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1.5" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => onOpenTask(tRec)}
                          title={t('tasks.inspectLogs')}
                          aria-label={t('tasks.inspectLogs')}
                          icon={<Terminal className="w-4 h-4" />}
                        />
                        {tRec.status === 'running' && (
                          <Button
                            variant="danger"
                            size="icon-sm"
                            onClick={(e) => handleCancel(e, tRec.id)}
                            title={t('tasks.cancelExecution')}
                            aria-label={t('tasks.cancelExecution')}
                            icon={<Ban className="w-4 h-4" />}
                          />
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </div>
  );
};

