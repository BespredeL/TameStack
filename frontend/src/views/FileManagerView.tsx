import React, { useEffect, useRef, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  FolderTree,
  Server,
  Folder,
  File,
  FileText,
  FileCode,
  Archive,
  Image as ImageIcon,
  ArrowUp,
  RefreshCw,
  Plus,
  Trash2,
  Edit2,
  Lock,
  Search,
  ShieldAlert,
  Terminal,
  PieChart,
  Copy,
  Check,
  Eye,
  EyeOff,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import {
  ListDirectory,
  DeleteRemoteEntry,
  SetServerSudoPassword,
} from '../../wailsjs/go/main/App';
import { files } from '../../wailsjs/go/models';
import { ServerRecord } from '../types';
import { FileEditorModal } from '../components/FileEditorModal';
import { ModalShell } from '../components/common/ModalShell';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { PageToolbar, Button, Tabs, SearchInput } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import { NewEntryModal } from '../components/files/NewEntryModal';
import { RenameEntryModal } from '../components/files/RenameEntryModal';
import { ChmodModal } from '../components/files/ChmodModal';
import { DiskUsageAnalyzer } from '../components/files/DiskUsageAnalyzer';

interface FileManagerViewProps {
  servers: ServerRecord[];
  onOpenAddServer: () => void;
  onOpenTask?: (task: any) => void;
}

export const FileManagerView: React.FC<FileManagerViewProps> = ({
  servers,
  onOpenAddServer,
  onOpenTask = () => { },
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
    setPageLoading,
  } = useServer();
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();

  // Tab: 'explorer' | 'analyzer'
  const [activeTab, setActiveTab] = useState<'explorer' | 'analyzer'>('explorer');

  const [currentPath, setCurrentPath] = useState<string>('/var/www');
  const [listing, setListing] = useState<files.DirectoryListing | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState<string>('');

  // Editable path input & copy
  const [isEditingPath, setIsEditingPath] = useState<boolean>(false);
  const [manualPathInput, setManualPathInput] = useState<string>('/var/www');
  const [isPathCopied, setIsPathCopied] = useState<boolean>(false);

  // Hidden files toggle
  const [showHidden, setShowHidden] = useState<boolean>(false);

  // Table sorting
  const [sortBy, setSortBy] = useState<'name' | 'size' | 'mtime'>('name');
  const [sortAsc, setSortAsc] = useState<boolean>(true);

  // Modals state
  const [newEntryModal, setNewEntryModal] = useState<{ isOpen: boolean; isDir: boolean }>({
    isOpen: false,
    isDir: false,
  });
  const [renameModal, setRenameModal] = useState<{
    isOpen: boolean;
    oldPath: string;
    oldName: string;
  }>({
    isOpen: false,
    oldPath: '',
    oldName: '',
  });
  const [chmodModal, setChmodModal] = useState<{
    isOpen: boolean;
    path: string;
    permissions: string;
  }>({
    isOpen: false,
    path: '',
    permissions: '0755',
  });

  // Editing file
  const [editingFile, setEditingFile] = useState<string | null>(null);
  const [isSudoPasswordModalOpen, setIsSudoPasswordModalOpen] = useState(false);
  const [newSudoPassword, setNewSudoPassword] = useState('');
  const [isSavingSudoPass, setIsSavingSudoPass] = useState(false);
  const [sudoError, setSudoError] = useState<string | null>(null);
  const listGen = useRef(0);

  const currentServer = servers.find((s) => s.id === selectedServerID);

  const saveSudoPassword = async () => {
    setIsSavingSudoPass(true);
    setSudoError(null);
    try {
      await SetServerSudoPassword(selectedServerID, newSudoPassword);
      setNewSudoPassword('');
      setIsSudoPasswordModalOpen(false);
      fetchDirectory(currentPath);
    } catch (err: unknown) {
      setSudoError(getErrorMessage(err));
    } finally {
      setIsSavingSudoPass(false);
    }
  };

  const fetchDirectory = async (pathTarget: string) => {
    if (!selectedServerID) return;
    const serverID = selectedServerID;
    const gen = ++listGen.current;
    setListing(null);
    try {
      setIsLoading(true);
      setPageLoading(true, t('filemanager.readingDir', { path: pathTarget }));
      setErrorMsg(null);
      const res = await ListDirectory(serverID, pathTarget);
      if (gen !== listGen.current) return;
      setListing(res);
      setCurrentPath(res.current_path);
      setManualPathInput(res.current_path);
    } catch (err: unknown) {
      if (gen !== listGen.current) return;
      console.error('Failed to list remote directory:', err);
      setErrorMsg(getErrorMessage(err));
    } finally {
      if (gen === listGen.current) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  useEffect(() => {
    fetchDirectory(currentPath);
  }, [selectedServerID]);

  const handleNavigate = (pathTarget: string) => {
    setIsEditingPath(false);
    fetchDirectory(pathTarget);
  };

  const handleManualPathSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (manualPathInput.trim()) {
      handleNavigate(manualPathInput.trim());
    }
  };

  const handleCopyPath = () => {
    navigator.clipboard.writeText(currentPath);
    setIsPathCopied(true);
    setTimeout(() => setIsPathCopied(false), 2000);
  };

  const handleDeleteEntry = async (entryPath: string, name: string) => {
    if (
      !(await confirm(t('filemanager.deleteConfirm', { name }), {
        danger: true,
        confirmLabel: t('common.delete'),
      }))
    ) {
      return;
    }
    try {
      await DeleteRemoteEntry({
        server_id: selectedServerID,
        path: entryPath,
      });
      fetchDirectory(currentPath);
    } catch (err: unknown) {
      alert(t('filemanager.deleteError', { error: getErrorMessage(err) }));
    }
  };

  const handleSort = (field: 'name' | 'size' | 'mtime') => {
    if (sortBy === field) {
      setSortAsc(!sortAsc);
    } else {
      setSortBy(field);
      setSortAsc(true);
    }
  };

  const getFileIcon = (entry: files.FileEntry) => {
    if (entry.is_dir) return <Folder className="w-4 h-4 text-amber-400 shrink-0" />;
    const ext = entry.extension;
    if (
      [
        '.php',
        '.js',
        '.ts',
        '.html',
        '.css',
        '.json',
        '.sh',
        '.py',
        '.go',
        '.yml',
        '.yaml',
        '.sql',
        '.conf',
        '.env',
        '.ini',
      ].includes(ext)
    ) {
      return <FileCode className="w-4 h-4 text-indigo-400 shrink-0" />;
    }
    if (['.log'].includes(ext)) {
      return <Terminal className="w-4 h-4 text-amber-400 shrink-0" />;
    }
    if (['.zip', '.tar', '.gz', '.bz2', '.7z'].includes(ext)) {
      return <Archive className="w-4 h-4 text-rose-400 shrink-0" />;
    }
    if (['.png', '.jpg', '.jpeg', '.svg', '.webp', '.gif', '.ico'].includes(ext)) {
      return <ImageIcon className="w-4 h-4 text-emerald-400 shrink-0" />;
    }
    return <FileText className="w-4 h-4 text-slate-400 shrink-0" />;
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={FolderTree}
        description={t('empty.filemanagerDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  // Filter and sort entries
  const rawEntries = listing?.entries || [];
  const visibleEntries = rawEntries.filter((e) => {
    if (!showHidden && e.name.startsWith('.') && e.name !== '..') {
      return false;
    }
    if (searchQuery.trim()) {
      return e.name.toLowerCase().includes(searchQuery.toLowerCase());
    }
    return true;
  });

  const sortedEntries = [...visibleEntries].sort((a, b) => {
    // Keep directories first always
    if (a.is_dir !== b.is_dir) {
      return a.is_dir ? -1 : 1;
    }
    let cmp = 0;
    if (sortBy === 'name') {
      cmp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    } else if (sortBy === 'size') {
      cmp = a.size - b.size;
    } else if (sortBy === 'mtime') {
      const timeA = new Date(a.mod_time).getTime();
      const timeB = new Date(b.mod_time).getTime();
      cmp = timeA - timeB;
    }
    return sortAsc ? cmp : -cmp;
  });

  // Calculate folder stats
  const totalFoldersCount = rawEntries.filter((e) => e.is_dir).length;
  const totalFilesCount = rawEntries.filter((e) => !e.is_dir).length;
  const totalFilesSizeBytes = rawEntries
    .filter((e) => !e.is_dir)
    .reduce((sum, e) => sum + e.size, 0);

  const formatByteSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    const kb = bytes / 1024;
    if (kb < 1024) return `${kb.toFixed(1)} KB`;
    const mb = kb / 1024;
    if (mb < 1024) return `${mb.toFixed(1)} MB`;
    const gb = mb / 1024;
    return `${gb.toFixed(1)} GB`;
  };

  // Breadcrumbs parsing
  const pathParts = currentPath.split('/').filter(Boolean);

  return (
    <div className="page space-y-6">
      {/* Top Bar: Server selector + Tabs */}
      <PageToolbar
        icon={<FolderTree className="w-5 h-5 text-indigo-400" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('filemanager.targetServer')}
        filters={
          <Tabs
            tabs={[
              { id: 'explorer', label: t('filemanager.tabExplorer'), icon: <Folder className="w-3.5 h-3.5" /> },
              { id: 'analyzer', label: t('filemanager.tabAnalyzer'), icon: <PieChart className="w-3.5 h-3.5" /> },
            ]}
            activeTab={activeTab}
            onChange={(tabId) => setActiveTab(tabId as any)}
            variant="pill"
            size="sm"
          />
        }
      />

      {/* DISK USAGE ANALYZER TAB */}
      {activeTab === 'analyzer' && (
        <DiskUsageAnalyzer
          serverID={selectedServerID}
          serverName={currentServer?.name || ''}
          initialPath={currentPath}
          onNavigateToFileManager={(target) => {
            setCurrentPath(target);
            setActiveTab('explorer');
            fetchDirectory(target);
          }}
          onOpenFileEditor={(filePath) => setEditingFile(filePath)}
          onOpenTask={onOpenTask}
        />
      )}

      {/* FILE EXPLORER TAB */}
      {activeTab === 'explorer' && (
        <>
          {/* Quick Location Shortcuts & Disk Analyzer Shortcut */}
          <div className="flex items-center justify-between gap-3 text-xs flex-wrap">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-slate-500 font-medium mr-1">
                {t('filemanager.quickJump')}
              </span>
              <button
                onClick={() => handleNavigate('/var/www')}
                className={`px-2.5 py-1 rounded-lg border transition ${currentPath.startsWith('/var/www')
                  ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40'
                  : 'bg-surface-900 text-slate-400 border-slate-800 hover:text-white'
                  }`}
              >
                {t('filemanager.quickSites')}
              </button>
              <button
                onClick={() => handleNavigate('/var/log')}
                className={`px-2.5 py-1 rounded-lg border transition ${currentPath.startsWith('/var/log')
                  ? 'bg-amber-600/20 text-amber-300 border-amber-500/40'
                  : 'bg-surface-900 text-slate-400 border-slate-800 hover:text-white'
                  }`}
              >
                {t('filemanager.quickLogs')}
              </button>
              <button
                onClick={() => handleNavigate('/etc')}
                className={`px-2.5 py-1 rounded-lg border transition ${currentPath.startsWith('/etc')
                  ? 'bg-purple-600/20 text-purple-300 border-purple-500/40'
                  : 'bg-surface-900 text-slate-400 border-slate-800 hover:text-white'
                  }`}
              >
                {t('filemanager.quickConfigs')}
              </button>
              <button
                onClick={() => handleNavigate('/home')}
                className={`px-2.5 py-1 rounded-lg border transition ${currentPath.startsWith('/home')
                  ? 'bg-emerald-600/20 text-emerald-300 border-emerald-500/40'
                  : 'bg-surface-900 text-slate-400 border-slate-800 hover:text-white'
                  }`}
              >
                {t('filemanager.quickHome')}
              </button>
            </div>

            <button
              type="button"
              onClick={() => setActiveTab('analyzer')}
              className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-surface-900 hover:bg-slate-800 border border-indigo-500/30 text-indigo-300 transition text-xs font-medium ml-auto"
            >
              <PieChart className="w-3.5 h-3.5 text-indigo-400" />
              <span>{t('filemanager.analyzeSpaceBtn')}</span>
            </button>
          </div>

          {/* Error alert banner */}
          {errorMsg && (
            <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between gap-4">
              <div className="flex items-center gap-2">
                <ShieldAlert className="w-4 h-4 text-rose-400 shrink-0" />
                <span>{t('filemanager.accessError', { error: errorMsg })}</span>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <button
                  onClick={() => {
                    setNewSudoPassword('');
                    setIsSudoPasswordModalOpen(true);
                  }}
                  className="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg font-semibold transition flex items-center gap-1.5"
                >
                  <Lock className="w-3 h-3 text-amber-400" />
                  <span>{t('filemanager.sudoPasswordBtn')}</span>
                </button>
              </div>
            </div>
          )}

          {/* Main File Explorer Container */}
          <div className="bg-surface-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
            {/* Address Bar & Actions Header */}
            <div className="px-6 py-3.5 border-b border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4 bg-slate-950/40">
              {/* Left: Level-up + Breadcrumbs or Editable Path */}
              <div className="flex items-center gap-2 flex-1 min-w-0">
                {listing?.parent_path && (
                  <button
                    onClick={() => handleNavigate(listing.parent_path)}
                    className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg transition-colors shrink-0"
                    title={t('filemanager.levelUpTooltip')}
                  >
                    <ArrowUp className="w-3.5 h-3.5" />
                  </button>
                )}

                {isEditingPath ? (
                  <form onSubmit={handleManualPathSubmit} className="flex items-center gap-2 flex-1 max-w-xl">
                    <input
                      type="text"
                      value={manualPathInput}
                      onChange={(e) => setManualPathInput(e.target.value)}
                      onBlur={() => setIsEditingPath(false)}
                      autoFocus
                      className="w-full px-3 py-1 bg-slate-950 border border-indigo-500 rounded-lg text-xs font-mono text-white focus:outline-none"
                    />
                  </form>
                ) : (
                  <div className="flex items-center gap-1.5 flex-wrap text-xs font-mono min-w-0">
                    <button
                      onClick={() => handleNavigate('/')}
                      className="text-slate-400 hover:text-white transition-colors"
                    >
                      /
                    </button>

                    {pathParts.map((part, idx) => {
                      const fullSubPath = '/' + pathParts.slice(0, idx + 1).join('/');
                      const isLast = idx === pathParts.length - 1;

                      return (
                        <React.Fragment key={fullSubPath}>
                          <span className="text-slate-600 font-mono">/</span>
                          <button
                            onClick={() => handleNavigate(fullSubPath)}
                            className={`transition-colors ${isLast
                              ? 'text-white font-semibold'
                              : 'text-indigo-400 hover:text-indigo-300'
                              }`}
                          >
                            {part}
                          </button>
                        </React.Fragment>
                      );
                    })}

                    <button
                      type="button"
                      onClick={() => setIsEditingPath(true)}
                      className="p-1 text-slate-500 hover:text-slate-300 transition rounded"
                      title={t('filemanager.renameTooltip')}
                    >
                      <Edit2 className="w-3 h-3" />
                    </button>

                    <button
                      type="button"
                      onClick={handleCopyPath}
                      className="p-1 text-slate-500 hover:text-slate-300 transition rounded"
                      title={t('filemanager.copyPath')}
                    >
                      {isPathCopied ? (
                        <Check className="w-3 h-3 text-emerald-400" />
                      ) : (
                        <Copy className="w-3 h-3" />
                      )}
                    </button>
                  </div>
                )}
              </div>

              {/* Right: Search, Hidden toggle, Create Folder / File, Refresh */}
              <div className="flex items-center gap-2 shrink-0 flex-wrap">
                <div className="w-48 sm:w-60 min-w-0">
                  <SearchInput
                    sizeVariant="sm"
                    placeholder={t('filemanager.searchPlaceholder')}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onClear={() => setSearchQuery('')}
                  />
                </div>

                <Button
                  variant={showHidden ? 'primary' : 'secondary'}
                  size="icon-sm"
                  onClick={() => setShowHidden(!showHidden)}
                  icon={showHidden ? <Eye className="w-3.5 h-3.5" /> : <EyeOff className="w-3.5 h-3.5 text-slate-400" />}
                  title={t('filemanager.showHiddenFiles')}
                  aria-label={t('filemanager.showHiddenFiles')}
                />

                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setNewEntryModal({ isOpen: true, isDir: true })}
                  icon={<Folder className="w-3.5 h-3.5 text-amber-400" />}
                >
                  {t('filemanager.folderBtn')}
                </Button>

                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setNewEntryModal({ isOpen: true, isDir: false })}
                  icon={<Plus className="w-3.5 h-3.5" />}
                >
                  {t('filemanager.fileBtn')}
                </Button>

                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => fetchDirectory(currentPath)}
                  disabled={isLoading}
                  icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />}
                  title={t('filemanager.refreshTooltip')}
                />
              </div>
            </div>

            {/* Directory Listing Table */}
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-slate-800/80 bg-slate-950/20 text-slate-400 font-semibold uppercase tracking-wider select-none">
                    <th
                      className="px-6 py-3.5 cursor-pointer hover:text-white transition"
                      onClick={() => handleSort('name')}
                    >
                      <div className="flex items-center gap-1.5">
                        <span>{t('filemanager.colName')}</span>
                        {sortBy === 'name' && (
                          sortAsc ? <ChevronUp className="w-3.5 h-3.5 text-indigo-400" /> : <ChevronDown className="w-3.5 h-3.5 text-indigo-400" />
                        )}
                      </div>
                    </th>
                    <th
                      className="px-6 py-3.5 cursor-pointer hover:text-white transition"
                      onClick={() => handleSort('size')}
                    >
                      <div className="flex items-center gap-1.5">
                        <span>{t('filemanager.colSize')}</span>
                        {sortBy === 'size' && (
                          sortAsc ? <ChevronUp className="w-3.5 h-3.5 text-indigo-400" /> : <ChevronDown className="w-3.5 h-3.5 text-indigo-400" />
                        )}
                      </div>
                    </th>
                    <th className="px-6 py-3.5">{t('filemanager.colPermissions')}</th>
                    <th
                      className="px-6 py-3.5 cursor-pointer hover:text-white transition"
                      onClick={() => handleSort('mtime')}
                    >
                      <div className="flex items-center gap-1.5">
                        <span>{t('filemanager.colModified')}</span>
                        {sortBy === 'mtime' && (
                          sortAsc ? <ChevronUp className="w-3.5 h-3.5 text-indigo-400" /> : <ChevronDown className="w-3.5 h-3.5 text-indigo-400" />
                        )}
                      </div>
                    </th>
                    <th className="px-6 py-3.5 text-right">{t('filemanager.colActions')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/50">
                  {isLoading ? (
                    <tr>
                      <td colSpan={5} className="px-6 py-12 text-center text-slate-400">
                        <div className="flex items-center justify-center gap-2">
                          <RefreshCw className="w-4 h-4 animate-spin text-indigo-400" />
                          <span>{t('filemanager.readingSFTP')}</span>
                        </div>
                      </td>
                    </tr>
                  ) : sortedEntries.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-6 py-12 text-center text-slate-500">
                        {t('filemanager.emptyDirectory')}
                      </td>
                    </tr>
                  ) : (
                    sortedEntries.map((entry) => (
                      <tr
                        key={entry.path}
                        className="hover:bg-slate-850/40 transition-colors group cursor-pointer"
                        onClick={() => {
                          if (entry.is_dir) {
                            handleNavigate(entry.path);
                          } else {
                            setEditingFile(entry.path);
                          }
                        }}
                      >
                        <td className="px-6 py-3 font-mono text-white font-medium flex items-center gap-2.5">
                          {getFileIcon(entry)}
                          <span className="hover:text-indigo-400 transition-colors truncate max-w-md">
                            {entry.name}
                          </span>
                        </td>
                        <td className="px-6 py-3 font-mono text-slate-400">
                          {entry.is_dir ? '—' : entry.human_size}
                        </td>
                        <td className="px-6 py-3 font-mono text-slate-500">
                          {entry.permissions}
                        </td>
                        <td className="px-6 py-3 font-mono text-slate-400">
                          {new Date(entry.mod_time).toLocaleString()}
                        </td>
                        <td className="px-6 py-3 text-right">
                          <div
                            className="flex items-center justify-end gap-1 opacity-60 group-hover:opacity-100 transition-opacity"
                            onClick={(e) => e.stopPropagation()}
                          >
                            {!entry.is_dir && (
                              <button
                                onClick={() => setEditingFile(entry.path)}
                                className="p-1.5 rounded-lg text-slate-400 hover:text-indigo-400 hover:bg-slate-800 transition-colors"
                                title={t('filemanager.editTooltip')}
                              >
                                <FileCode className="w-3.5 h-3.5" />
                              </button>
                            )}
                            <button
                              onClick={() =>
                                setRenameModal({
                                  isOpen: true,
                                  oldPath: entry.path,
                                  oldName: entry.name,
                                })
                              }
                              className="p-1.5 rounded-lg text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors"
                              title={t('filemanager.renameTooltip')}
                            >
                              <Edit2 className="w-3.5 h-3.5" />
                            </button>
                            <button
                              onClick={() =>
                                setChmodModal({
                                  isOpen: true,
                                  path: entry.path,
                                  permissions: entry.permissions,
                                })
                              }
                              className="p-1.5 rounded-lg text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors"
                              title={t('filemanager.chmodTooltip')}
                            >
                              <Lock className="w-3.5 h-3.5" />
                            </button>
                            <button
                              onClick={() => handleDeleteEntry(entry.path, entry.name)}
                              className="p-1.5 rounded-lg text-slate-400 hover:text-rose-500 hover:bg-slate-800 transition-colors"
                              title={t('filemanager.deleteTooltip')}
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Bottom Status Bar */}
            <div className="px-6 py-2.5 border-t border-slate-800 bg-slate-950/60 text-[11px] text-slate-400 flex items-center justify-between font-mono">
              <div className="flex items-center gap-4">
                <span>
                  <strong>{totalFoldersCount}</strong> {t('filemanager.statusBarFolders')}
                </span>
                <span>
                  <strong>{totalFilesCount}</strong> {t('filemanager.statusBarFiles')}
                </span>
                <span>
                  {t('filemanager.statusBarTotalSize')}:{' '}
                  <strong className="text-slate-200">{formatByteSize(totalFilesSizeBytes)}</strong>
                </span>
              </div>
              <span className="text-slate-500 truncate max-w-sm">{currentPath}</span>
            </div>
          </div>
        </>
      )}

      {/* New File / Folder Modal */}
      <NewEntryModal
        isOpen={newEntryModal.isOpen}
        isDir={newEntryModal.isDir}
        currentPath={currentPath}
        selectedServerID={selectedServerID}
        onClose={() => setNewEntryModal({ isOpen: false, isDir: false })}
        onCreated={() => fetchDirectory(currentPath)}
      />

      {/* Rename Entry Modal */}
      <RenameEntryModal
        isOpen={renameModal.isOpen}
        oldPath={renameModal.oldPath}
        oldName={renameModal.oldName}
        currentPath={currentPath}
        selectedServerID={selectedServerID}
        onClose={() => setRenameModal({ isOpen: false, oldPath: '', oldName: '' })}
        onRenamed={() => fetchDirectory(currentPath)}
      />

      {/* Chmod Modal */}
      <ChmodModal
        isOpen={chmodModal.isOpen}
        entryPath={chmodModal.path}
        currentPermissions={chmodModal.permissions}
        selectedServerID={selectedServerID}
        onClose={() => setChmodModal({ isOpen: false, path: '', permissions: '0755' })}
        onChmodSaved={() => fetchDirectory(currentPath)}
      />

      {/* Inline File Editor Modal */}
      {editingFile && (
        <FileEditorModal
          serverID={selectedServerID}
          filePath={editingFile}
          onClose={() => setEditingFile(null)}
          onSaved={() => fetchDirectory(currentPath)}
        />
      )}

      {/* Sudo Password Modal */}
      {isSudoPasswordModalOpen && (
        <ModalShell
          onClose={() => setIsSudoPasswordModalOpen(false)}
          size="md"
          icon={<ShieldAlert className="w-5 h-5 text-amber-400" />}
          iconClassName=""
          title={t('filemanager.sudoModalTitle')}
        >
          <div className="p-6 space-y-4">
            <p className="text-xs text-slate-400 leading-relaxed">
              {t('filemanager.sudoModalDesc')}
            </p>
            <input
              type="password"
              placeholder={t('filemanager.sudoPlaceholder')}
              value={newSudoPassword}
              onChange={(e) => setNewSudoPassword(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-700 rounded-lg text-sm text-white focus:outline-none focus:border-indigo-500"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newSudoPassword) {
                  void saveSudoPassword();
                }
              }}
            />
            {sudoError && <p className="text-xs text-rose-400">{sudoError}</p>}
            <div className="flex items-center justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setIsSudoPasswordModalOpen(false)}
                className="px-4 py-2 text-xs font-semibold text-slate-400 hover:text-white transition"
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                disabled={isSavingSudoPass || !newSudoPassword}
                onClick={() => void saveSudoPassword()}
                className="px-4 py-2 text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-lg transition shadow-md shadow-indigo-600/20"
              >
                {isSavingSudoPass ? t('filemanager.saving') : t('filemanager.saveAndApply')}
              </button>
            </div>
          </div>
        </ModalShell>
      )}
    </div>
  );
};
