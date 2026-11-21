import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { useServerData } from '../hooks/useServerData';
import {
  Clock,
  Plus,
  RefreshCw,
  Trash2,
  Edit2,
  User,
} from 'lucide-react';
import { ListCronJobs, DeleteCronJob, ToggleCronJob } from '../../wailsjs/go/main/App';
import { cron } from '../../wailsjs/go/models';
import { ServerRecord } from '../types';
import {
  PageToolbar,
  Button,
  Badge,
  SearchInput,
  Select,
  TableContainer,
  Table,
  TableHeader,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  TableLoading,
  TableEmpty,
} from '../components/ui';
import { CreateCronJobModal } from '../components/CreateCronJobModal';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';

interface CronViewProps {
  servers: ServerRecord[];
  onOpenAddServer: () => void;
}

export const CronView: React.FC<CronViewProps> = ({
  servers,
  onOpenAddServer,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
  } = useServer();
  const { t } = useTranslation();
  const confirm = useConfirm();
  const [selectedUser, setSelectedUser] = useState<string>('root');
  const [searchQuery, setSearchQuery] = useState<string>('');

  // Modals
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [editingJob, setEditingJob] = useState<cron.CronJob | null>(null);

  const {
    data: jobs,
    loading: isLoading,
    error: actionError,
    setError: setActionError,
    reload: fetchJobs,
  } = useServerData(
    (serverId) => ListCronJobs(serverId, selectedUser).then((r) => r || []),
    [selectedUser],
    {
      initial: [] as cron.CronJob[],
      reset: [] as cron.CronJob[],
      loadingMessage: t('cron.loadingJobs', { user: selectedUser }),
      clearError: false,
    },
  );

  const handleToggle = async (job: cron.CronJob) => {
    if (job.enabled && !(await confirm(t('cron.pauseConfirm', { command: job.command })))) {
      return;
    }
    try {
      await ToggleCronJob({
        server_id: selectedServerID,
        job_id: job.id,
        user: selectedUser,
        enabled: !job.enabled,
      });
      fetchJobs();
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  const handleDelete = async (jobID: string) => {
    if (!(await confirm(t('cron.deleteConfirm'), { danger: true, confirmLabel: t('common.delete') }))) return;
    try {
      await DeleteCronJob({
        server_id: selectedServerID,
        job_id: jobID,
        user: selectedUser,
      });
      fetchJobs();
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };



  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Clock}
        description={t('empty.cronDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  const filteredJobs = jobs.filter((j) =>
    j.command.toLowerCase().includes(searchQuery.toLowerCase()) ||
    j.comment.toLowerCase().includes(searchQuery.toLowerCase()) ||
    j.expression.includes(searchQuery)
  );

  return (
    <div className="page space-y-6">
      {actionError && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          {actionError}
        </div>
      )}
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Clock className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('cron.targetServer')}
        filters={
          <div className="w-44">
            <span className="text-[10px] font-semibold text-slate-400 block uppercase tracking-wider mb-1">
              {t('cron.cronUser')}
            </span>
            <Select
              sizeVariant="sm"
              value={selectedUser}
              onChange={(e) => setSelectedUser(e.target.value)}
              icon={<User className="w-3.5 h-3.5" />}
            >
              <option value="root">root</option>
              <option value="www-data">www-data</option>
              <option value="all">{t('cron.allUsers')}</option>
            </Select>
          </div>
        }
        actions={
          <>
            <Button
              variant="secondary"
              size="md"
              onClick={fetchJobs}
              disabled={isLoading}
              icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
              title={t('cron.refreshTooltip')}
            >
              {t('cron.refreshBtn')}
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => {
                setEditingJob(null);
                setIsModalOpen(true);
              }}
              icon={<Plus className="w-4 h-4" />}
            >
              {t('cron.createJob')}
            </Button>
          </>
        }
      />

      {/* Main Jobs Table Container */}
      <TableContainer>
        {/* Search bar */}
        <div className="px-4 py-3 border-b border-slate-800 flex items-center justify-between gap-4 bg-slate-950/40">
          <div className="w-64">
            <SearchInput
              placeholder={t('cron.searchPlaceholder')}
              value={searchQuery}
              onChangeValue={setSearchQuery}
              onClear={() => setSearchQuery('')}
              sizeVariant="sm"
            />
          </div>
        </div>

        {/* Table */}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">{t('cron.colStatus')}</TableHead>
              <TableHead>{t('cron.colSchedule')}</TableHead>
              <TableHead>{t('cron.colCommand')}</TableHead>
              <TableHead>{t('cron.colDescription')}</TableHead>
              <TableHead className="text-right pr-4">{t('cron.colActions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableLoading colSpan={5} message={t('cron.readingSSH')} />
            ) : filteredJobs.length === 0 ? (
              <TableEmpty
                colSpan={5}
                icon={<Clock className="w-8 h-8 text-slate-600" />}
                message={t('cron.noJobsForUser', { user: selectedUser })}
              />
            ) : (
              filteredJobs.map((job) => (
                <TableRow key={job.id}>
                  <TableCell className="pl-4">
                    <button
                      type="button"
                      onClick={() => handleToggle(job)}
                      className="cursor-pointer"
                      title={job.enabled ? t('cron.clickToDisable') : t('cron.clickToEnable')}
                    >
                      <Badge
                        variant={job.enabled ? 'success' : 'neutral'}
                        size="xs"
                        dot={job.enabled}
                      >
                        {job.enabled ? t('cron.activeStatus') : t('cron.disabledStatus')}
                      </Badge>
                    </button>
                  </TableCell>
                  <TableCell className="font-mono text-indigo-400 font-semibold whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      <span>{job.expression}</span>
                      {job.source && job.source !== 'crontab' && (
                        <span
                          className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-slate-800 border border-slate-700 text-slate-300 font-normal"
                          title={job.source}
                        >
                          {job.source.replace('/etc/cron.d/', 'cron.d/')}
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-slate-200 text-[11px] max-w-md truncate" title={job.command}>
                    {selectedUser === 'all' && (
                      <span className="text-indigo-400/90 font-sans mr-2 font-semibold bg-indigo-500/10 px-1.5 py-0.5 rounded border border-indigo-500/20 text-[10px]">
                        {job.user}
                      </span>
                    )}
                    {job.command}
                  </TableCell>
                  <TableCell className="text-slate-400">
                    {job.comment || <span className="text-slate-600 italic">{t('cron.noDescription')}</span>}
                  </TableCell>
                  <TableCell className="text-right pr-4">
                    <div className="flex items-center justify-end gap-1.5">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => {
                          setEditingJob(job);
                          setIsModalOpen(true);
                        }}
                        title={t('cron.editTooltip')}
                        aria-label={t('cron.editTooltip')}
                        icon={<Edit2 className="w-3.5 h-3.5" />}
                      />
                      <Button
                        variant="danger"
                        size="icon-sm"
                        onClick={() => handleDelete(job.id)}
                        title={t('cron.deleteTooltip')}
                        aria-label={t('cron.deleteTooltip')}
                        icon={<Trash2 className="w-3.5 h-3.5" />}
                      />
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Modal */}
      {isModalOpen && (
        <CreateCronJobModal
          serverID={selectedServerID}
          initialJob={editingJob}
          onClose={() => {
            setIsModalOpen(false);
            setEditingJob(null);
          }}
          onSaved={fetchJobs}
        />
      )}
    </div>
  );
};

