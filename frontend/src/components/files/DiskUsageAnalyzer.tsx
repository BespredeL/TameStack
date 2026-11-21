import React, { useState, useEffect } from 'react';
import {
  HardDrive,
  Folder,
  FileText,
  Trash2,
  RefreshCw,
  ArrowRight,
  Sparkles,
  AlertTriangle,
  FolderOpen,
  Edit2,
} from 'lucide-react';
import { Button, Input } from '../ui';
import {
  AnalyzeDiskUsage,
  CleanDiskCategory,
  DeleteRemoteEntry,
} from '../../../wailsjs/go/main/App';
import { files } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { useConfirm } from '../../context/ConfirmContext';
import { getErrorMessage } from '../../utils/errors';
import { EventsOn } from '../../../wailsjs/runtime/runtime';

interface DiskUsageAnalyzerProps {
  serverID: string;
  serverName: string;
  initialPath?: string;
  onNavigateToFileManager: (path: string) => void;
  onOpenFileEditor?: (filePath: string) => void;
  onOpenTask: (task: any) => void;
}

export const DiskUsageAnalyzer: React.FC<DiskUsageAnalyzerProps> = ({
  serverID,
  serverName,
  initialPath = '/',
  onNavigateToFileManager,
  onOpenFileEditor,
  onOpenTask,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();

  const [currentPath, setCurrentPath] = useState(initialPath);
  const [inputPath, setInputPath] = useState(initialPath);
  const [report, setReport] = useState<files.DiskAnalysisReport | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cleaningCat, setCleaningCat] = useState<string | null>(null);

  const fetchAnalysis = async (pathTarget: string) => {
    if (!serverID) return;
    setIsLoading(true);
    setError(null);
    try {
      const res = await AnalyzeDiskUsage(serverID, pathTarget);
      setReport(res);
      setCurrentPath(res.analyzed_path);
      setInputPath(res.analyzed_path);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchAnalysis(currentPath);
  }, [serverID]);

  // Listen for cleanup task completion
  useEffect(() => {
    const cleanup = EventsOn('task:finished', (data: any) => {
      if (
        data &&
        data.server_id === serverID &&
        data.operation_type === 'files_disk_cleanup' &&
        data.status === 'completed'
      ) {
        fetchAnalysis(currentPath);
      }
    });
    return () => {
      if (typeof cleanup === 'function') cleanup();
    };
  }, [serverID, currentPath]);

  const handleCleanCategory = async (category: files.CleanupCategory) => {
    const localizedTitle = t(`filemanager.cleanupCategories.${category.id}.title`) || category.title;
    const localizedSize = category.human_size === 'Оценка по факту' ? t('filemanager.evaluatedOnRun') : category.human_size;
    if (
      !(await confirm(
        t('filemanager.cleanConfirm', {
          title: localizedTitle,
          size: localizedSize,
        }),
        { danger: true, confirmLabel: t('common.clean') }
      ))
    ) {
      return;
    }

    setCleaningCat(category.id);
    try {
      const task = await CleanDiskCategory(serverID, category.id);
      onOpenTask(task as any);
      setTimeout(() => fetchAnalysis(currentPath), 2000);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setCleaningCat(null);
    }
  };

  const handleDeleteFile = async (filePath: string, name: string) => {
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
        server_id: serverID,
        path: filePath,
      });
      fetchAnalysis(currentPath);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handlePathSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (inputPath.trim()) {
      fetchAnalysis(inputPath.trim());
    }
  };

  const quickJumpPaths = ['/', '/var', '/var/log', '/var/www', '/home', '/tmp'];

  return (
    <div className="space-y-6">
      {/* Partition Overview Cards */}
      {report?.partitions && report.partitions.length > 0 && (
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-3 flex items-center gap-2">
            <HardDrive className="w-4 h-4 text-indigo-400" />
            <span>{t('filemanager.diskPartitionsTitle')}</span>
          </h4>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {report.partitions.map((part) => {
              const isHigh = part.used_percent > 85;
              const isMed = part.used_percent > 70 && !isHigh;

              return (
                <div
                  key={part.mount_point}
                  className={`bg-surface-850 border rounded-xl p-4 transition shadow-sm ${
                    isHigh
                      ? 'border-rose-500/40 bg-rose-500/5'
                      : isMed
                      ? 'border-amber-500/40 bg-amber-500/5'
                      : 'border-slate-800'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <HardDrive
                        className={`w-4 h-4 ${
                          isHigh
                            ? 'text-rose-400'
                            : isMed
                            ? 'text-amber-400'
                            : 'text-indigo-400'
                        }`}
                      />
                      <span className="font-mono text-sm font-bold text-white">
                        {part.mount_point}
                      </span>
                    </div>
                    <span
                      className={`text-xs font-bold font-mono px-2 py-0.5 rounded-full ${
                        isHigh
                          ? 'bg-rose-500/20 text-rose-300'
                          : isMed
                          ? 'bg-amber-500/20 text-amber-300'
                          : 'bg-emerald-500/20 text-emerald-300'
                      }`}
                    >
                      {part.used_percent}%
                    </span>
                  </div>

                  {/* Progress bar */}
                  <div className="w-full h-2 bg-slate-900 rounded-full overflow-hidden mb-2">
                    <div
                      className={`h-full transition-all duration-500 ${
                        isHigh
                          ? 'bg-rose-500'
                          : isMed
                          ? 'bg-amber-500'
                          : 'bg-indigo-500'
                      }`}
                      style={{ width: `${Math.min(part.used_percent, 100)}%` }}
                    />
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono">
                    <span>
                      {t('filemanager.used')}:{' '}
                      <strong className="text-slate-200">{part.human_used}</strong>
                    </span>
                    <span>
                      {t('filemanager.free')}:{' '}
                      <strong className="text-slate-200">{part.human_free}</strong> / {part.human_total}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Directory Selector Bar */}
      <div className="bg-surface-850 p-4 rounded-xl border border-slate-800 shadow-sm flex flex-col sm:flex-row items-center justify-between gap-4">
        <form onSubmit={handlePathSubmit} className="flex items-center gap-2 w-full sm:w-auto flex-1">
          <span className="text-xs text-slate-400 shrink-0 font-medium">
            {t('filemanager.analyzedFolder')}:
          </span>
          <div className="flex-1 max-w-lg">
            <Input
              type="text"
              value={inputPath}
              onChange={(e) => setInputPath(e.target.value)}
              sizeVariant="sm"
              className="font-mono"
            />
          </div>
          <Button
            type="submit"
            disabled={isLoading}
            loading={isLoading}
            variant="primary"
            size="sm"
            icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} />}
          >
            {t('filemanager.scanBtn')}
          </Button>
        </form>

        {/* Quick jump chips */}
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] text-slate-500">{t('filemanager.quickSelect')}:</span>
          {quickJumpPaths.map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => fetchAnalysis(p)}
              className={`px-2 py-1 text-[11px] font-mono rounded-lg border transition ${
                currentPath === p
                  ? 'bg-indigo-500/20 text-indigo-300 border-indigo-500/40 font-bold'
                  : 'bg-slate-900 border-slate-800 text-slate-400 hover:text-white'
              }`}
            >
              {p}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <div className="p-3.5 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center gap-2.5 text-xs text-rose-300">
          <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Quick Safe Cleanup Section */}
      {report?.cleanup_options && report.cleanup_options.length > 0 && (
        <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                <Sparkles className="w-4 h-4" />
              </div>
              <div>
                <h4 className="text-sm font-bold text-white flex items-center gap-2">
                  <span>{t('filemanager.cleanupHeading')}</span>
                </h4>
                <p className="text-xs text-slate-400">
                  {t('filemanager.cleanupSubheading')}
                </p>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {report.cleanup_options.map((cat) => {
              const titleKey = `filemanager.cleanupCategories.${cat.id}.title`;
              const descKey = `filemanager.cleanupCategories.${cat.id}.desc`;
              const translatedTitle = t(titleKey);
              const translatedDesc = t(descKey);
              const title = translatedTitle !== titleKey ? translatedTitle : cat.title;
              const desc = translatedDesc !== descKey ? translatedDesc : cat.description;
              const size = cat.human_size === 'Оценка по факту' ? t('filemanager.evaluatedOnRun') : cat.human_size;
              return (
                <div
                  key={cat.id}
                  className="bg-slate-950/60 border border-slate-800 rounded-xl p-3.5 flex flex-col justify-between gap-3"
                >
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-xs font-semibold text-white">{title}</span>
                      <span className="text-xs font-mono font-bold text-emerald-400">
                        {size}
                      </span>
                    </div>
                    <p className="text-[11px] text-slate-400 leading-relaxed">
                      {desc}
                    </p>
                  </div>

                  <button
                    type="button"
                    disabled={!cat.available || cleaningCat === cat.id}
                    onClick={() => handleCleanCategory(cat)}
                    className="w-full py-1.5 px-3 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 disabled:hover:bg-slate-800 text-slate-200 rounded-lg text-xs font-semibold transition border border-slate-700/60 flex items-center justify-center gap-1.5"
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                    <span>
                      {cleaningCat === cat.id
                        ? t('filemanager.cleaning')
                        : t('filemanager.cleanCategoryBtn')}
                    </span>
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Main Grid: Left = Top Directories, Right = Largest Files */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* TOP DIRECTORIES */}
        <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm flex flex-col">
          <div className="px-5 py-3.5 border-b border-slate-800 bg-slate-950/40 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Folder className="w-4 h-4 text-amber-400" />
              <h4 className="text-xs font-bold text-white uppercase tracking-wider">
                {t('filemanager.subdirectoriesTitle')}
              </h4>
            </div>
            <span className="text-xs font-mono text-slate-400">
              {t('filemanager.totalInPath')}:{' '}
              <strong className="text-white">{report?.human_total_path || '—'}</strong>
            </span>
          </div>

          <div className="p-2 flex-1 overflow-x-auto">
            {isLoading ? (
              <div className="py-12 text-center text-slate-400 flex items-center justify-center gap-2">
                <RefreshCw className="w-4 h-4 animate-spin text-indigo-400" />
                <span>{t('filemanager.calculatingSizes')}</span>
              </div>
            ) : !report?.top_directories || report.top_directories.length === 0 ? (
              <div className="py-12 text-center text-slate-500 text-xs">
                {t('filemanager.noSubdirsFound')}
              </div>
            ) : (
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="text-slate-500 border-b border-slate-800/60 pb-2">
                    <th className="py-2 px-3 font-medium">{t('filemanager.colDirectory')}</th>
                    <th className="py-2 px-3 font-medium">{t('filemanager.colShare')}</th>
                    <th className="py-2 px-3 font-medium text-right">{t('filemanager.colSize')}</th>
                    <th className="py-2 px-3 text-right"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/40 font-mono">
                  {report.top_directories.map((dir) => (
                    <tr key={dir.path} className="hover:bg-slate-900/40 transition">
                      <td className="py-2 px-3 font-medium text-white flex items-center gap-2 max-w-[200px] truncate">
                        <Folder className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                        <span className="truncate" title={dir.path}>
                          {dir.name}
                        </span>
                      </td>
                      <td className="py-2 px-3 w-32">
                        <div className="flex items-center gap-2">
                          <div className="w-16 h-1.5 bg-slate-900 rounded-full overflow-hidden">
                            <div
                              className="h-full bg-amber-400 rounded-full"
                              style={{ width: `${Math.min(dir.percent, 100)}%` }}
                            />
                          </div>
                          <span className="text-[10px] text-slate-400">{dir.percent}%</span>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-right font-bold text-slate-200">
                        {dir.human_size}
                      </td>
                      <td className="py-2 px-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => fetchAnalysis(dir.path)}
                            className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-indigo-400 transition"
                            title={t('filemanager.drillDownTooltip')}
                          >
                            <ArrowRight className="w-3.5 h-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => onNavigateToFileManager(dir.path)}
                            className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-white transition"
                            title={t('filemanager.openInExplorerTooltip')}
                          >
                            <FolderOpen className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* TOP LARGEST FILES */}
        <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm flex flex-col">
          <div className="px-5 py-3.5 border-b border-slate-800 bg-slate-950/40 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <FileText className="w-4 h-4 text-indigo-400" />
              <h4 className="text-xs font-bold text-white uppercase tracking-wider">
                {t('filemanager.largestFilesTitle')}
              </h4>
            </div>
            <span className="text-xs font-mono text-slate-400">
              {report?.largest_files?.length || 0} {t('filemanager.filesCount')}
            </span>
          </div>

          <div className="p-2 flex-1 overflow-x-auto">
            {isLoading ? (
              <div className="py-12 text-center text-slate-400 flex items-center justify-center gap-2">
                <RefreshCw className="w-4 h-4 animate-spin text-indigo-400" />
                <span>{t('filemanager.scanningLargeFiles')}</span>
              </div>
            ) : !report?.largest_files || report.largest_files.length === 0 ? (
              <div className="py-12 text-center text-slate-500 text-xs">
                {t('filemanager.noLargeFilesFound')}
              </div>
            ) : (
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="text-slate-500 border-b border-slate-800/60 pb-2">
                    <th className="py-2 px-3 font-medium">{t('filemanager.colFile')}</th>
                    <th className="py-2 px-3 font-medium text-right">{t('filemanager.colSize')}</th>
                    <th className="py-2 px-3 text-right"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/40 font-mono">
                  {report.largest_files.map((file) => (
                    <tr key={file.path} className="hover:bg-slate-900/40 transition">
                      <td className="py-2 px-3 max-w-[220px]">
                        <div className="flex items-center gap-2">
                          <FileText className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                          <div className="truncate">
                            <span className="text-white font-medium block truncate" title={file.path}>
                              {file.name}
                            </span>
                            <span className="text-[10px] text-slate-500 block truncate" title={file.path}>
                              {file.path}
                            </span>
                          </div>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-right font-bold text-rose-300">
                        {file.human_size}
                      </td>
                      <td className="py-2 px-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          {onOpenFileEditor && (
                            <button
                              type="button"
                              onClick={() => onOpenFileEditor(file.path)}
                              className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-indigo-400 transition"
                              title={t('filemanager.editTooltip')}
                            >
                              <Edit2 className="w-3.5 h-3.5" />
                            </button>
                          )}
                          <button
                            type="button"
                            onClick={() => {
                              const parent = file.path.substring(0, file.path.lastIndexOf('/')) || '/';
                              onNavigateToFileManager(parent);
                            }}
                            className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-white transition"
                            title={t('filemanager.openInExplorerTooltip')}
                          >
                            <FolderOpen className="w-3.5 h-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDeleteFile(file.path, file.name)}
                            className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-rose-400 transition"
                            title={t('filemanager.deleteTooltip')}
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
