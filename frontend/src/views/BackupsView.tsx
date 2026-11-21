import React, { useState } from 'react';
import {
  Archive,
  Plus,
  RefreshCw,
  Trash2,
  RotateCcw,
  Globe,
  Database,
  Settings,
  Shield,
  HardDrive,
  Calendar,
  Download,
  AlertTriangle,
  CheckCircle2,
  Radar,
} from 'lucide-react';
import { ListBackups, DeleteBackup, DownloadBackup, DiscoverBackups } from '../../wailsjs/go/main/App';
import { backup, db } from '../../wailsjs/go/models';
import {
  PageToolbar,
  Button,
  Badge,
  SearchInput,
  Tabs,
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
import { ServerRecord, TaskRecord } from '../types';
import { CreateBackupModal } from '../components/CreateBackupModal';
import { RestoreBackupModal } from '../components/RestoreBackupModal';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { getErrorMessage } from '../utils/errors';
import { useServer } from '../context/ServerContext';
import { useServerData } from '../hooks/useServerData';

interface BackupsViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const BackupsView: React.FC<BackupsViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
  } = useServer();
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const [filterType, setFilterType] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [downloadingId, setDownloadingId] = useState<string | null>(null);

  // Modals
  const [isCreateOpen, setIsCreateOpen] = useState<boolean>(false);
  const [restoreTarget, setRestoreTarget] = useState<db.BackupRecord | null>(null);

  const [isDiscovering, setIsDiscovering] = useState<boolean>(false);
  const [discoverResult, setDiscoverResult] = useState<backup.DiscoverBackupsResult | null>(null);
  const [discoverError, setDiscoverError] = useState<string | null>(null);

  const {
    data: backups,
    loading: isLoading,
    reload: fetchBackups,
  } = useServerData(
    (serverId) => ListBackups(serverId).then((r) => r || []),
    [],
    {
      initial: [] as db.BackupRecord[],
      loadingMessage: t('backups.loadingList'),
      onError: (msg) => console.error('Failed to list backups:', msg),
    },
  );

  const handleDiscover = async () => {
    setIsDiscovering(true);
    setDiscoverResult(null);
    setDiscoverError(null);
    try {
      const res = await DiscoverBackups(selectedServerID);
      setDiscoverResult(res);
      if ((res?.registered?.length ?? 0) > 0) {
        await fetchBackups();
      }
    } catch (err: unknown) {
      setDiscoverError(getErrorMessage(err));
    } finally {
      setIsDiscovering(false);
    }
  };

  const handleDownload = async (backupID: string) => {
    try {
      setDownloadingId(backupID);
      const savedPath = await DownloadBackup(selectedServerID, backupID);
      if (savedPath) {
        await fetchBackups();
      }
    } catch (err: unknown) {
      alert(t('backups.downloadFailed', { error: getErrorMessage(err) }));
    } finally {
      setDownloadingId(null);
    }
  };

  const handleDelete = async (backupID: string) => {
    if (!(await confirm(t('backups.deleteConfirm'), { danger: true, confirmLabel: t('common.delete') }))) return;
    try {
      await DeleteBackup(selectedServerID, backupID);
      fetchBackups();
    } catch (err: unknown) {
      alert(`${t('backups.deleteError')}: ${getErrorMessage(err)}`);
    }
  };

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'site':
        return <Globe className="w-4 h-4 text-indigo-400" />;
      case 'database':
        return <Database className="w-4 h-4 text-emerald-400" />;
      case 'config':
        return <Settings className="w-4 h-4 text-amber-400" />;
      default:
        return <Shield className="w-4 h-4 text-purple-400" />;
    }
  };



  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Archive}
        description={t('empty.backupsDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  const totalSize = backups.reduce((acc, b) => acc + (b.size_bytes || 0), 0);
  const filteredBackups = backups.filter((b) => {
    const matchesFilter = filterType === 'all' || b.backup_type === filterType;
    const matchesSearch =
      b.target_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      b.file_path.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesFilter && matchesSearch;
  });

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Archive className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('sites.targetServer')}
        actions={
          <>
            <Button
              variant="secondary"
              size="md"
              onClick={handleDiscover}
              disabled={isDiscovering || isLoading}
              icon={<Radar className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin text-indigo-400' : 'text-emerald-400'}`} />}
              title={t('backups.discoverTitle')}
            >
              {isDiscovering ? t('backups.discovering') : t('backups.discover')}
            </Button>
            <Button
              variant="secondary"
              size="md"
              onClick={fetchBackups}
              disabled={isLoading}
              icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
              title={t('backups.refreshList')}
            >
              {t('common.refresh')}
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => setIsCreateOpen(true)}
              icon={<Plus className="w-4 h-4" />}
            >
              {t('backups.createBackup')}
            </Button>
          </>
        }
      />

      {(discoverError || discoverResult) && (
        <div
          className={`flex items-start justify-between gap-3 p-3 rounded-lg border text-xs ${
            discoverError
              ? 'bg-rose-500/10 border-rose-500/30 text-rose-300'
              : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
          }`}
        >
          <div className="flex items-start gap-2 min-w-0">
            {discoverError ? (
              <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
            ) : (
              <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
            )}
            <span className="min-w-0 break-words">
              {discoverError
                ? t('backups.discoverFailed', { error: discoverError })
                : (discoverResult?.registered?.length ?? 0) > 0
                  ? t('backups.discoverRegistered', {
                      count: discoverResult?.registered?.length ?? 0,
                      dirs: discoverResult?.scanned_paths?.join(', ') ?? '',
                    })
                  : (discoverResult?.already_known ?? 0) > 0
                    ? t('backups.discoverAllKnown', { count: discoverResult?.already_known ?? 0 })
                    : t('backups.discoverNoneFound', { dirs: discoverResult?.scanned_paths?.join(', ') ?? '' })}
            </span>
          </div>
          <button
            onClick={() => {
              setDiscoverError(null);
              setDiscoverResult(null);
            }}
            className="shrink-0 text-slate-400 hover:text-white transition"
            aria-label={t('common.close')}
          >
            ✕
          </button>
        </div>
      )}

      {/* Metrics Row */}
      <div className="stat-grid">
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
            <Archive className="w-5 h-5" />
          </div>
          <div>
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
              {t('backups.totalArchives')}
            </span>
            <span className="text-lg font-bold text-white">{backups.length}</span>
          </div>
        </div>

        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
            <HardDrive className="w-5 h-5" />
          </div>
          <div>
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
              {t('backups.totalStorage')}
            </span>
            <span className="text-lg font-bold text-white">{formatBytes(totalSize)}</span>
          </div>
        </div>

        <div className="bg-surface-850 border border-slate-800 p-4 rounded-2xl shadow-sm flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-400">
            <Calendar className="w-5 h-5" />
          </div>
          <div>
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">
              {t('backups.latestBackup')}
            </span>
            <span className="text-xs font-semibold text-white">
              {backups.length > 0
                ? new Date(backups[0].created_at).toLocaleDateString()
                : t('backups.noBackupsYet')}
            </span>
          </div>
        </div>
      </div>

      {/* Main Table Container */}
      <TableContainer>
        {/* Filter bar */}
        <div className="px-4 py-3 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-950/40">
          <div className="flex flex-wrap items-center gap-3">
            <Tabs
              variant="pill"
              size="sm"
              activeTab={filterType}
              onChange={(id) => setFilterType(id as any)}
              tabs={[
                { id: 'all', label: t('backups.filterAll') },
                { id: 'site', label: t('backups.filterSites') },
                { id: 'database', label: t('backups.filterDatabases') },
                { id: 'config', label: t('backups.filterConfigs') },
                { id: 'full', label: t('backups.filterFull') },
              ]}
            />

            <div className="w-48 sm:w-64">
              <SearchInput
                placeholder={t('backups.searchPlaceholder')}
                value={searchQuery}
                onChangeValue={setSearchQuery}
                onClear={() => setSearchQuery('')}
                sizeVariant="sm"
              />
            </div>
          </div>
        </div>

        {/* Archives Table */}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">{t('backups.colTarget')}</TableHead>
              <TableHead>{t('backups.colStorage')}</TableHead>
              <TableHead>{t('backups.colFilePath')}</TableHead>
              <TableHead>{t('backups.colSize')}</TableHead>
              <TableHead>{t('backups.colCreated')}</TableHead>
              <TableHead className="text-right pr-4">{t('backups.colActions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableLoading colSpan={6} message={t('backups.readingSSH')} />
            ) : filteredBackups.length === 0 ? (
              <TableEmpty
                colSpan={6}
                icon={<Archive className="w-8 h-8 text-slate-600" />}
                message={t('backups.noBackupsFound')}
              />
            ) : (
              filteredBackups.map((b) => (
                <TableRow key={b.id}>
                  <TableCell className="pl-4 font-medium text-white">
                    <div className="flex items-center gap-2.5">
                      {getTypeIcon(b.backup_type)}
                      <div>
                        <span className="font-semibold block">{b.target_name}</span>
                        <span className="text-[10px] text-slate-500 uppercase block font-mono">
                          {b.backup_type}
                        </span>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="space-y-1">
                      {b.offsite ? (
                        <Badge
                          variant="success"
                          size="xs"
                          dot
                          title={t('backups.savedOnPcTooltip', { date: b.downloaded_at ? new Date(b.downloaded_at).toLocaleString() : '' })}
                        >
                          <Shield className="w-3 h-3 text-emerald-400" />
                          <span>{t('backups.savedOnPc')}</span>
                        </Badge>
                      ) : (
                        <Badge
                          variant="warning"
                          size="xs"
                          title={t('backups.serverOnlyTooltip')}
                        >
                          <AlertTriangle className="w-3 h-3 text-amber-400 shrink-0" />
                          <span>{t('backups.serverOnly')}</span>
                        </Badge>
                      )}
                      {b.verified && (
                        <div className="text-[10px] text-emerald-400/90 flex items-center gap-1 font-mono">
                          <CheckCircle2 className="w-2.5 h-2.5 text-emerald-400" />
                          <span>{t('backups.verifiedIntegrity')}</span>
                        </div>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-slate-400 max-w-xs truncate text-[11px]" title={b.file_path}>
                    {b.file_path}
                  </TableCell>
                  <TableCell className="font-mono text-slate-200">
                    {formatBytes(b.size_bytes)}
                  </TableCell>
                  <TableCell className="font-mono text-slate-400">
                    {new Date(b.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right pr-4">
                    <div className="flex items-center justify-end gap-1.5">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => handleDownload(b.id)}
                        disabled={downloadingId === b.id}
                        loading={downloadingId === b.id}
                        title={t('backups.downloadToPcTooltip')}
                        aria-label={t('backups.downloadToPcTooltip')}
                        icon={!downloadingId ? <Download className="w-3.5 h-3.5" /> : undefined}
                      />
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => setRestoreTarget(b)}
                        title={t('backups.restoreTooltip')}
                        aria-label={t('backups.restoreTooltip')}
                        icon={<RotateCcw className="w-3.5 h-3.5 text-amber-400" />}
                      />
                      <Button
                        variant="danger"
                        size="icon-sm"
                        onClick={() => handleDelete(b.id)}
                        title={t('backups.deleteTooltip')}
                        aria-label={t('backups.deleteTooltip')}
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

      {/* Modals */}
      {isCreateOpen && (
        <CreateBackupModal
          serverID={selectedServerID}
          servers={servers}
          onClose={() => setIsCreateOpen(false)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchBackups, 3000);
          }}
        />
      )}

      {restoreTarget && (
        <RestoreBackupModal
          serverID={selectedServerID}
          backup={restoreTarget}
          onClose={() => setRestoreTarget(null)}
          onTaskCreated={(t) => {
            onOpenTask(t);
            setTimeout(fetchBackups, 3000);
          }}
        />
      )}
    </div>
  );
};
