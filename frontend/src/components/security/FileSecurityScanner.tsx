import React, { useState, useEffect, useCallback } from 'react';
import { getErrorMessage } from '../../utils/errors';

import {
  Search,
  Shield,
  AlertTriangle,
  CheckCircle2,
  Trash2,
  Eye,
  FileCode,
  Folder,
  Clock,
  Sparkles,
  Camera,
  GitCompare,
  Loader2,
  Sliders,
} from 'lucide-react';
import {
  ScanSecurityFiles,
  DeleteSecurityFile,
  AddFileExclusion,
  ListFileExclusions,
  DeleteFileExclusion,
  CreateFileSnapshot,
  ListFileSnapshots,
  DeleteFileSnapshot,
  CompareFileSnapshot,
  ListFileSecuritySignatures,
  SaveFileSecuritySignature,
  DeleteFileSecuritySignature,
  ToggleFileSecuritySignature,
  ListSites,
} from '../../../wailsjs/go/main/App';
import { security, db } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { useAlert } from '../../context/AlertContext';
import { useConfirm } from '../../context/ConfirmContext';
import { FileViewerModal } from './FileViewerModal';
import { ExclusionsModal } from './ExclusionsModal';
import { SnapshotsModal } from './SnapshotsModal';
import { SignaturesModal } from './SignaturesModal';
import { ScanResultsTable } from './ScanResultsTable';
import { Button, Input, Select } from '../ui';

export function matchFileExclusion(filePath: string, rawPattern: string): boolean {
  if (!filePath || !rawPattern) return false;
  const pattern = rawPattern.trim();
  if (!pattern) return false;

  // Exact match
  if (filePath === pattern) return true;

  // No wildcards: check exact, prefix or directory component
  if (!pattern.includes('*') && !pattern.includes('?') && !pattern.includes('[')) {
    const clean = pattern.endsWith('/') ? pattern.slice(0, -1) : pattern;
    if (
      filePath === clean ||
      filePath.startsWith(clean + '/') ||
      filePath.includes('/' + clean.replace(/^\//, '') + '/') ||
      filePath.endsWith('/' + clean.replace(/^\//, '')) ||
      filePath.includes(pattern)
    ) {
      return true;
    }
    return false;
  }

  // Trailing directory wildcard: e.g. /var/www/site/cache/* or cache/*
  if (pattern.endsWith('/*')) {
    const dir = pattern.slice(0, -2);
    if (filePath === dir || filePath.startsWith(dir + '/')) return true;
    const cleanDir = dir.replace(/^\//, '');
    if (filePath.includes('/' + cleanDir + '/') || filePath.endsWith('/' + cleanDir)) return true;
  }

  // Infix directory wildcard: e.g. */cache/*
  if (pattern.startsWith('*/') && pattern.endsWith('/*')) {
    const mid = pattern.slice(2, -2);
    if (filePath.includes('/' + mid + '/') || filePath.endsWith('/' + mid)) return true;
  }

  // Extension wildcard: e.g. *.log or *.php
  if (pattern.startsWith('*.') && !pattern.slice(2).includes('/')) {
    const ext = pattern.slice(1);
    if (filePath.endsWith(ext)) return true;
  }

  // General glob matching
  try {
    const escaped = pattern
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*\*/g, '§§GLOBSTAR§§')
      .replace(/\*/g, '[^/]*')
      .replace(/\?/g, '[^/]')
      .replace(/§§GLOBSTAR§§/g, '.*');

    const re = new RegExp('(^|/)' + escaped + '($|/)');
    if (re.test(filePath)) return true;

    const fullRe = new RegExp('^' + escaped + '$');
    if (fullRe.test(filePath)) return true;
  } catch {
    const stripped = pattern.replace(/[*?[\]]/g, '');
    if (stripped && filePath.includes(stripped)) return true;
  }

  return false;
}

interface FileSecurityScannerProps {
  serverId: string;
}

export const FileSecurityScanner: React.FC<FileSecurityScannerProps> = ({ serverId }) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  // Search parameters
  const [basePath, setBasePath] = useState('/var/www');
  const [timeRange, setTimeRange] = useState<'1h' | '6h' | '24h' | '3d' | '7d' | '30d' | 'all'>('24h');
  const [fileTypes, setFileTypes] = useState<'scripts' | 'configs' | 'all'>('scripts');
  const [detectMalware, setDetectMalware] = useState(true);

  // Available sites on server for quick directory pick
  const [serverSites, setServerSites] = useState<db.SiteRecord[]>([]);

  // Scan state
  const [isScanning, setIsScanning] = useState(false);
  const [scanResult, setScanResult] = useState<security.ScanFilesResult | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);

  // Table filter & search
  const [threatFilter, setThreatFilter] = useState<'all' | 'threats' | 'clean'>('all');
  const [tableSearch, setTableSearch] = useState('');
  const [hideExcluded, setHideExcluded] = useState(true);

  // File Viewer / Editor Modal
  const [viewingFile, setViewingFile] = useState<security.ScannedFile | null>(null);

  // Exclusions (Whitelist) Modal
  const [isExclusionsOpen, setIsExclusionsOpen] = useState(false);
  const [exclusions, setExclusions] = useState<security.FileSecurityExclusion[]>([]);
  const [newExclusionPattern, setNewExclusionPattern] = useState('');
  const [newExclusionReason, setNewExclusionReason] = useState('');
  const [isAddingExclusion, setIsAddingExclusion] = useState(false);

  // Snapshots (File Integrity) Modal
  const [isSnapshotsOpen, setIsSnapshotsOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<security.FileSecuritySnapshot[]>([]);
  const [isLoadingSnapshots, setIsLoadingSnapshots] = useState(false);
  const [isCreatingSnapshot, setIsCreatingSnapshot] = useState(false);
  const [snapshotName, setSnapshotName] = useState('');
  const [diffResult, setDiffResult] = useState<security.SnapshotDiffResult | null>(null);
  const [isComparing, setIsComparing] = useState(false);

  // Heuristic Signatures Modal
  const [isSignaturesOpen, setIsSignaturesOpen] = useState(false);
  const [signatures, setSignatures] = useState<security.FileSecuritySignature[]>([]);
  const [isLoadingSignatures, setIsLoadingSignatures] = useState(false);

  // Status Notification Toast
  const [statusToast, setStatusToast] = useState<{ type: 'success' | 'error' | 'info'; message: string } | null>(null);

  const showToast = (type: 'success' | 'error' | 'info', message: string) => {
    setStatusToast({ type, message });
    if (type !== 'info') {
      setTimeout(() => {
        setStatusToast((curr) => (curr?.message === message ? null : curr));
      }, 6000);
    }
  };

  const [copiedPath, setCopiedPath] = useState<string | null>(null);

  useEffect(() => {
    if (serverId) {
      loadSites();
      loadExclusions();
      loadSnapshots();
      loadSignatures();
    }
  }, [serverId]);

  const loadSignatures = async () => {
    setIsLoadingSignatures(true);
    try {
      const res = await ListFileSecuritySignatures(serverId);
      setSignatures(res || []);
    } catch (err: unknown) {
      console.error('Failed to load signatures', err);
    } finally {
      setIsLoadingSignatures(false);
    }
  };

  const loadSites = async () => {
    try {
      const sites = await ListSites(serverId);
      setServerSites(sites || []);
      if (sites && sites.length > 0 && sites[0].document_root) {
        setBasePath(sites[0].document_root);
      }
    } catch {
      // ignore
    }
  };

  const loadExclusions = async () => {
    try {
      const res = await ListFileExclusions(serverId);
      setExclusions(res || []);
    } catch (err: unknown) {
      console.error('Failed to load exclusions', err);
    }
  };

  const loadSnapshots = async () => {
    setIsLoadingSnapshots(true);
    try {
      const res = await ListFileSnapshots(serverId);
      setSnapshots(res || []);
    } catch (err: unknown) {
      console.error('Failed to load snapshots', err);
    } finally {
      setIsLoadingSnapshots(false);
    }
  };

  const handleScan = async () => {
    if (!serverId) return;
    setIsScanning(true);
    setScanError(null);
    setScanResult(null);
    setDiffResult(null);

    try {
      const req = new security.ScanFilesRequest({
        server_id: serverId,
        base_path: basePath,
        time_range: timeRange,
        file_types: fileTypes,
        detect_malware: detectMalware,
        max_results: 500,
      });
      const res = await ScanSecurityFiles(req);
      setScanResult(res);
    } catch (err: unknown) {
      setScanError(getErrorMessage(err));
    } finally {
      setIsScanning(false);
    }
  };

  const handleDeleteFile = async (filePath: string) => {
    if (!(await confirm(t('fileScanner.deleteConfirm', { path: filePath }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }

    try {
      await DeleteSecurityFile(serverId, filePath);
      // Remove from scan result
      if (scanResult) {
        setScanResult(new security.ScanFilesResult({
          ...scanResult,
          files: (scanResult.files || []).filter((f) => f.path !== filePath),
          scanned_count: Math.max(0, (scanResult.scanned_count || 1) - 1),
        }));
      }
      // Remove from diff result if open
      if (diffResult) {
        setDiffResult(new security.SnapshotDiffResult({
          ...diffResult,
          added_files: (diffResult.added_files || []).filter((f) => f.path !== filePath),
          modified_files: (diffResult.modified_files || []).filter((f) => f.path !== filePath),
        }));
      }
      if (viewingFile && viewingFile.path === filePath) {
        setViewingFile(null);
      }
    } catch (err: unknown) {
      alert(t('fileScanner.deleteFileError', { error: getErrorMessage(err) }));
    }
  };

  const isFileExcluded = useCallback(
    (filePath: string, backendFlag?: boolean) => {
      if (backendFlag) return true;
      if (!exclusions || exclusions.length === 0) return false;
      return exclusions.some((exc) => matchFileExclusion(filePath, exc.pattern));
    },
    [exclusions]
  );

  const handleAddQuickExclusion = async (filePath: string) => {
    const pattern = filePath;
    const reason = t('fileScanner.addExclusionReasonDefault');
    try {
      await AddFileExclusion(serverId, pattern, reason);
      await loadExclusions();
      // Mark as excluded locally
      if (scanResult) {
        setScanResult(new security.ScanFilesResult({
          ...scanResult,
          files: (scanResult.files || []).map((f) => (f.path === filePath ? { ...f, is_excluded: true } : f)),
        }));
      }
    } catch (err: unknown) {
      alert(t('fileScanner.addExclusionError', { error: getErrorMessage(err) }));
    }
  };

  const handleAddCustomExclusion = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newExclusionPattern.trim()) return;
    setIsAddingExclusion(true);
    const pat = newExclusionPattern.trim();
    try {
      await AddFileExclusion(serverId, pat, newExclusionReason.trim());
      setNewExclusionPattern('');
      setNewExclusionReason('');
      await loadExclusions();
      if (scanResult) {
        setScanResult(
          new security.ScanFilesResult({
            ...scanResult,
            files: (scanResult.files || []).map((f) =>
              matchFileExclusion(f.path, pat) ? { ...f, is_excluded: true } : f
            ),
          })
        );
      }
      showToast('success', t('fileScanner.exclusionAddedToast'));
    } catch (err: unknown) {
      showToast('error', t('fileScanner.addExclusionError', { error: getErrorMessage(err) }));
    } finally {
      setIsAddingExclusion(false);
    }
  };

  const handleDeleteExclusion = async (id: string) => {
    try {
      await DeleteFileExclusion(id);
      const remaining = await ListFileExclusions(serverId);
      setExclusions(remaining || []);
      if (scanResult) {
        setScanResult(
          new security.ScanFilesResult({
            ...scanResult,
            files: (scanResult.files || []).map((f) => ({
              ...f,
              is_excluded: (remaining || []).some((exc) => matchFileExclusion(f.path, exc.pattern)),
            })),
          })
        );
      }
      showToast('success', t('fileScanner.exclusionRemovedToast'));
    } catch (err: unknown) {
      showToast('error', t('fileScanner.addExclusionError', { error: getErrorMessage(err) }));
    }
  };

  const handleCreateSnapshot = async () => {
    if (isCreatingSnapshot) return;
    setIsCreatingSnapshot(true);
    const targetPath = basePath;
    const name = snapshotName.trim();
    showToast('info', t('fileScanner.snapshotIndexingToast', { path: targetPath }));
    try {
      const snap = await CreateFileSnapshot(serverId, '', name, targetPath);
      setSnapshotName('');
      await loadSnapshots();
      showToast('success', t('fileScanner.snapshotCreatedToast', { name: snap.name, count: snap.files_count }));
    } catch (err: unknown) {
      showToast('error', t('fileScanner.snapshotCreateError', { error: getErrorMessage(err) }));
    } finally {
      setIsCreatingSnapshot(false);
    }
  };

  const handleCompareSnapshot = async (snapshotId: string) => {
    if (isComparing) return;
    setIsComparing(true);
    setScanResult(null);
    setDiffResult(null);
    setIsSnapshotsOpen(false);
    showToast('info', t('fileScanner.snapshotComparingToast'));
    try {
      const res = await CompareFileSnapshot(snapshotId);
      setDiffResult(res);
      showToast('success', t('fileScanner.snapshotComparedToast', { added: res.total_added, modified: res.total_modified, deleted: res.total_deleted }));
    } catch (err: unknown) {
      showToast('error', t('fileScanner.snapshotCompareError', { error: getErrorMessage(err) }));
    } finally {
      setIsComparing(false);
    }
  };

  const handleDeleteSnapshot = async (snapshotId: string) => {
    if (!(await confirm(t('fileScanner.deleteSnapshotConfirm'), { danger: true, confirmLabel: t('common.delete') }))) return;
    try {
      await DeleteFileSnapshot(snapshotId);
      await loadSnapshots();
      showToast('success', t('fileScanner.snapshotDeletedToast'));
    } catch (err: unknown) {
      showToast('error', t('fileScanner.addExclusionError', { error: getErrorMessage(err) }));
    }
  };

  const handleSaveSignature = async (sig: security.FileSecuritySignature) => {
    try {
      await SaveFileSecuritySignature(sig);
      await loadSignatures();
      showToast('success', t('fileScanner.signatureSavedToast'));
    } catch (err: unknown) {
      showToast('error', getErrorMessage(err));
      throw err;
    }
  };

  const handleDeleteSignature = async (id: string) => {
    if (!(await confirm(t('fileScanner.deleteSignatureConfirm'), { danger: true, confirmLabel: t('common.delete') }))) return;
    try {
      await DeleteFileSecuritySignature(id);
      await loadSignatures();
      showToast('success', t('fileScanner.signatureDeletedToast'));
    } catch (err: unknown) {
      showToast('error', getErrorMessage(err));
    }
  };

  const handleToggleSignature = async (id: string, enabled: boolean) => {
    try {
      await ToggleFileSecuritySignature(id, enabled);
      await loadSignatures();
      showToast('success', t('fileScanner.signatureToggledToast'));
    } catch (err: unknown) {
      showToast('error', getErrorMessage(err));
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedPath(text);
    setTimeout(() => setCopiedPath(null), 2000);
  };

  // Filtered files for display
  const displayedFiles = (scanResult?.files || []).map((f) => {
    const excluded = isFileExcluded(f.path, f.is_excluded);
    if (f.is_excluded !== excluded) {
      return { ...f, is_excluded: excluded };
    }
    return f;
  }).filter((f) => {
    if (hideExcluded && f.is_excluded) return false;
    if (threatFilter === 'threats' && f.threat_level === 'clean') return false;
    if (threatFilter === 'clean' && f.threat_level !== 'clean') return false;
    if (tableSearch) {
      const q = tableSearch.toLowerCase();
      return (
        f.path.toLowerCase().includes(q) ||
        (f.reasons && f.reasons.some((r) => r.toLowerCase().includes(q))) ||
        (f.snippet && f.snippet.toLowerCase().includes(q))
      );
    }
    return true;
  });

  return (
    <div className="space-y-6">
      {/* Background Operation Banner */}
      {(isCreatingSnapshot || isComparing) && (
        <div className="p-3.5 bg-indigo-950/40 border border-indigo-500/30 rounded-2xl text-xs flex items-center justify-between text-indigo-200">
          <div className="flex items-center gap-3">
            <Loader2 className="w-4 h-4 animate-spin text-indigo-400 shrink-0" />
            <div>
              <span className="font-semibold block">
                {isCreatingSnapshot ? t('fileScanner.backgroundBannerCreating', { path: basePath }) : t('fileScanner.backgroundBannerComparing')}
              </span>
              <span className="text-[11px] text-slate-400">
                {t('fileScanner.backgroundBannerDesc')}
              </span>
            </div>
          </div>
          <button
            onClick={() => setIsSnapshotsOpen(true)}
            className="px-3 py-1.5 bg-indigo-600/30 hover:bg-indigo-600/50 text-indigo-300 border border-indigo-500/30 rounded-lg text-xs font-semibold transition cursor-pointer shrink-0"
          >
            {t('fileScanner.backgroundBannerOpenBtn')}
          </button>
        </div>
      )}

      {/* Non-blocking Status Toast */}
      {statusToast && (
        <div
          className={`p-3.5 rounded-2xl border text-xs flex items-center justify-between transition-all ${
            statusToast.type === 'success'
              ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
              : statusToast.type === 'error'
              ? 'bg-rose-950/40 border-rose-500/40 text-rose-200'
              : 'bg-indigo-950/40 border-indigo-500/40 text-indigo-200'
          }`}
        >
          <div className="flex items-center gap-2.5">
            {statusToast.type === 'success' ? (
              <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
            ) : statusToast.type === 'error' ? (
              <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
            ) : (
              <Loader2 className="w-4 h-4 text-indigo-400 animate-spin shrink-0" />
            )}
            <span>{statusToast.message}</span>
          </div>
          <button
            onClick={() => setStatusToast(null)}
            className="text-slate-400 hover:text-white p-1 text-xs"
          >
            ✕
          </button>
        </div>
      )}
      {/* Search & Control Parameters Card */}
      <div className="bg-surface-850 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-4">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 pb-4 border-b border-slate-800/80">
          <div>
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              <Search className="w-4 h-4 text-indigo-400" />
              <span>{t('fileScanner.title')}</span>
            </h3>
            <p className="text-xs text-slate-400 mt-0.5">
              {t('fileScanner.subtitle')}
            </p>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setIsSnapshotsOpen(true)}
              icon={<Camera className="w-3.5 h-3.5 text-indigo-400" />}
              title={t('fileScanner.snapshotsBtn')}
            >
              {t('fileScanner.snapshotsBtn')} ({snapshots.length})
            </Button>

            <Button
              variant="secondary"
              size="sm"
              onClick={() => setIsExclusionsOpen(true)}
              icon={<Shield className="w-3.5 h-3.5 text-emerald-400" />}
              title={t('fileScanner.exclusionsBtn')}
            >
              {t('fileScanner.exclusionsBtn')} ({exclusions.length})
            </Button>

            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setIsSignaturesOpen(true);
                loadSignatures();
              }}
              icon={<Sliders className="w-3.5 h-3.5 text-indigo-400" />}
              title={t('fileScanner.signaturesBtn')}
            >
              {t('fileScanner.signaturesBtn')} ({signatures.length})
            </Button>
          </div>
        </div>

        {/* Input Parameters Grid */}
        <div className="grid grid-cols-1 md:grid-cols-12 gap-3 items-end">
          {/* Base Path & Quick Site Selector */}
          <div className="md:col-span-5 space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
                <Folder className="w-3.5 h-3.5 text-indigo-400" />
                <span>{t('fileScanner.pathLabel')}</span>
              </label>

              {serverSites.length > 0 && (
                <div className="flex items-center gap-1 text-[11px] text-slate-400">
                  <span>{t('fileScanner.siteSelect')}</span>
                  <select
                    onChange={(e) => {
                      if (e.target.value) setBasePath(e.target.value);
                    }}
                    className="bg-slate-900 border border-slate-800 rounded text-[11px] text-slate-300 px-1.5 py-0.5 focus:outline-none focus:border-indigo-500 cursor-pointer"
                  >
                    <option value="">{t('fileScanner.selectPlaceholder')}</option>
                    {serverSites.map((s) => (
                      <option key={s.id} value={s.document_root}>
                        {s.domain}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
            <Input
              type="text"
              value={basePath}
              onChange={(e) => setBasePath(e.target.value)}
              placeholder="/var/www/my-site/public"
              className="font-mono"
            />
          </div>

          {/* Time Range Selector */}
          <div className="md:col-span-2 space-y-1.5">
            <label className="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Clock className="w-3.5 h-3.5 text-amber-400" />
              <span>{t('fileScanner.timeRange')}</span>
            </label>
            <Select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as '1h' | '6h' | '24h' | '3d' | '7d' | '30d' | 'all')}
            >
              <option value="1h">{t('fileScanner.timeRange1h')}</option>
              <option value="6h">{t('fileScanner.timeRange6h')}</option>
              <option value="24h">{t('fileScanner.timeRange24h')}</option>
              <option value="3d">{t('fileScanner.timeRange3d')}</option>
              <option value="7d">{t('fileScanner.timeRange7d')}</option>
              <option value="14d">{t('fileScanner.timeRange14d')}</option>
              <option value="30d">{t('fileScanner.timeRange30d')}</option>
              <option value="all">{t('fileScanner.timeRangeAll')}</option>
            </Select>
          </div>

          {/* File Types Selector */}
          <div className="md:col-span-2 space-y-1.5">
            <label className="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <FileCode className="w-3.5 h-3.5 text-purple-400" />
              <span>{t('fileScanner.fileTypes')}</span>
            </label>
            <Select
              value={fileTypes}
              onChange={(e) => setFileTypes(e.target.value as 'scripts' | 'configs' | 'all')}
            >
              <option value="scripts">{t('fileScanner.fileTypesScripts')}</option>
              <option value="configs">{t('fileScanner.fileTypesConfigs')}</option>
              <option value="all">{t('fileScanner.fileTypesAll')}</option>
            </Select>
          </div>

          {/* Malware checkbox & Submit Button */}
          <div className="md:col-span-3 flex items-center gap-2">
            <Button
              onClick={handleScan}
              disabled={isScanning || !basePath}
              loading={isScanning}
              variant="primary"
              size="md"
              icon={!isScanning ? <Search className="w-4 h-4" /> : undefined}
              className="w-full"
            >
              {t('fileScanner.findFiles')}
            </Button>
          </div>
        </div>

        {/* Options Row */}
        <div className="flex flex-wrap items-center justify-between gap-3 pt-2 text-xs text-slate-400 border-t border-slate-800/60">
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={detectMalware}
              onChange={(e) => setDetectMalware(e.target.checked)}
              className="rounded border-slate-700 bg-slate-900 text-indigo-600 focus:ring-0 focus:ring-offset-0 cursor-pointer"
            />
            <span className="text-slate-300 font-medium flex items-center gap-1">
              <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
              {t('fileScanner.detectMalware')}
            </span>
          </label>

          <span className="text-[11px] text-slate-500">
            {t('fileScanner.pruneHint')}
          </span>
        </div>
      </div>

      {scanError && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/25 text-rose-300 text-xs flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
            <span>{scanError}</span>
          </div>
          <button onClick={() => setScanError(null)} className="text-rose-400 hover:text-white">
            ✕
          </button>
        </div>
      )}

      {/* Snapshot Diff Results (if comparing) */}
      {diffResult && (() => {
        const diffAdded = diffResult.added_files || [];
        const diffModified = diffResult.modified_files || [];
        const diffDeleted = diffResult.deleted_files || [];
        const changedList = [...diffAdded, ...diffModified];
        const hasNoChanges = changedList.length === 0 && diffDeleted.length === 0;

        return (
          <div className="bg-surface-850 border border-indigo-500/40 rounded-2xl p-5 shadow-lg space-y-4 animate-fade-in">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
              <div className="flex items-center gap-2.5">
                <div className="p-2 rounded-xl bg-indigo-500/15 border border-indigo-500/30 text-indigo-400">
                  <GitCompare className="w-5 h-5" />
                </div>
                <div>
                  <h4 className="text-sm font-bold text-white">
                    {t('fileScanner.diffReportTitle')} <span className="text-indigo-300 font-mono">{diffResult.snapshot_name}</span>
                  </h4>
                  <p className="text-xs text-slate-400 font-mono mt-0.5">
                    {t('fileScanner.diffBasePath')} {diffResult.base_path} ({t('fileScanner.diffCheckedIn', { ms: diffResult.duration_ms })})
                  </p>
                </div>
              </div>

              <button
                onClick={() => setDiffResult(null)}
                className="text-xs text-slate-400 hover:text-white px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 transition"
              >
                {t('fileScanner.closeReport')}
              </button>
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="p-3 rounded-xl bg-slate-900 border border-slate-800 flex items-center justify-between">
                <span className="text-xs text-slate-400">{t('fileScanner.addedFiles')}</span>
                <span className={`text-sm font-bold font-mono ${(diffResult.total_added || 0) > 0 ? 'text-rose-400' : 'text-slate-300'}`}>
                  +{(diffResult.total_added || 0)}
                </span>
              </div>
              <div className="p-3 rounded-xl bg-slate-900 border border-slate-800 flex items-center justify-between">
                <span className="text-xs text-slate-400">{t('fileScanner.modifiedFiles')}</span>
                <span className={`text-sm font-bold font-mono ${(diffResult.total_modified || 0) > 0 ? 'text-amber-400' : 'text-slate-300'}`}>
                  {(diffResult.total_modified || 0)}
                </span>
              </div>
              <div className="p-3 rounded-xl bg-slate-900 border border-slate-800 flex items-center justify-between">
                <span className="text-xs text-slate-400">{t('fileScanner.deletedFiles')}</span>
                <span className="text-sm font-bold font-mono text-slate-400">
                  -{(diffResult.total_deleted || 0)}
                </span>
              </div>
            </div>

            {hasNoChanges && (
              <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/25 text-emerald-300 text-xs flex items-center gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                <span>{t('fileScanner.noChangesDetected')}</span>
              </div>
            )}

            {/* Added & Modified Files Table */}
            {changedList.length > 0 && (
              <div className="space-y-3 pt-2">
                <h5 className="text-xs font-bold text-slate-200 uppercase tracking-wider">
                  {t('fileScanner.detectedChanges', { count: changedList.length })}
                </h5>
                <div className="divide-y divide-slate-800/80 rounded-xl border border-slate-800 bg-slate-950 overflow-hidden">
                  {changedList.map((f) => (
                    <div key={f.path} className="p-3 flex items-center justify-between gap-4 hover:bg-slate-900/60 transition">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${
                              f.threat_level === 'malicious'
                                ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                                : f.threat_level === 'suspicious'
                                ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                                : 'bg-indigo-500/15 text-indigo-300 border border-indigo-500/25'
                            }`}
                          >
                            {f.threat_level === 'malicious'
                              ? t('fileScanner.threatMalicious')
                              : f.threat_level === 'suspicious'
                              ? t('fileScanner.threatSuspicious')
                              : t('fileScanner.modifiedFiles')}
                          </span>
                          <span className="font-mono text-xs text-white truncate" title={f.path}>
                            {f.path}
                          </span>
                        </div>
                        {f.reasons && f.reasons.length > 0 && (
                          <div className="text-[11px] text-amber-300/90 mt-1 flex items-center gap-1.5">
                            <AlertTriangle className="w-3.5 h-3.5 shrink-0" />
                            <span>{f.reasons.join(' • ')}</span>
                          </div>
                        )}
                      </div>

                      <div className="flex items-center gap-2 shrink-0">
                        <button
                          onClick={() => setViewingFile(f)}
                          className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium flex items-center gap-1 transition"
                          title={t('fileScanner.viewFileTooltip')}
                        >
                          <Eye className="w-3.5 h-3.5 text-indigo-400" />
                          <span>{t('fileScanner.viewBtn')}</span>
                        </button>
                        <button
                          onClick={() => handleDeleteFile(f.path)}
                          className="p-1.5 rounded bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 transition"
                          title={t('fileScanner.deleteFileTooltip')}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Deleted Files List */}
            {diffDeleted.length > 0 && (
              <div className="space-y-2 pt-2">
                <h5 className="text-xs font-bold text-slate-300 uppercase tracking-wider">
                  {t('fileScanner.deletedFilesHeader', { count: diffDeleted.length })}
                </h5>
                <div className="divide-y divide-slate-800/80 rounded-xl border border-slate-800 bg-slate-950 p-2 space-y-1">
                  {diffDeleted.map((p) => (
                    <div key={p} className="p-2 flex items-center gap-2 text-xs font-mono text-slate-400">
                      <span className="px-1.5 py-0.5 rounded text-[10px] font-bold uppercase bg-slate-800 text-slate-400">
                        {t('fileScanner.deletedStatus')}
                      </span>
                      <span className="truncate">{p}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        );
      })()}

      {/* Results Workspace */}
      {scanResult && (
        <ScanResultsTable
          scanResult={scanResult}
          displayedFiles={displayedFiles}
          threatFilter={threatFilter}
          setThreatFilter={setThreatFilter}
          tableSearch={tableSearch}
          setTableSearch={setTableSearch}
          hideExcluded={hideExcluded}
          setHideExcluded={setHideExcluded}
          copiedPath={copiedPath}
          onCopy={copyToClipboard}
          onView={setViewingFile}
          onQuickExclusion={handleAddQuickExclusion}
          onDelete={handleDeleteFile}
        />
      )}

      {/* File Editor / Viewer Modal */}
      {viewingFile && (
        <FileViewerModal
          file={viewingFile}
          serverId={serverId}
          onClose={() => setViewingFile(null)}
          onDelete={handleDeleteFile}
        />
      )}

      {/* Exclusions (Whitelist) Modal */}
      {isExclusionsOpen && (
        <ExclusionsModal
          onClose={() => setIsExclusionsOpen(false)}
          exclusions={exclusions}
          newExclusionPattern={newExclusionPattern}
          setNewExclusionPattern={setNewExclusionPattern}
          newExclusionReason={newExclusionReason}
          setNewExclusionReason={setNewExclusionReason}
          isAddingExclusion={isAddingExclusion}
          onAddCustom={handleAddCustomExclusion}
          onDeleteExclusion={handleDeleteExclusion}
        />
      )}

      {/* Snapshots (File Integrity) Modal */}
      {isSnapshotsOpen && (
        <SnapshotsModal
          onClose={() => setIsSnapshotsOpen(false)}
          basePath={basePath}
          setBasePath={setBasePath}
          snapshotName={snapshotName}
          setSnapshotName={setSnapshotName}
          isCreatingSnapshot={isCreatingSnapshot}
          snapshots={snapshots}
          isLoadingSnapshots={isLoadingSnapshots}
          isComparing={isComparing}
          onCreateSnapshot={handleCreateSnapshot}
          onCompare={handleCompareSnapshot}
          onDeleteSnapshot={handleDeleteSnapshot}
        />
      )}

      {/* Heuristic Signatures Modal */}
      {isSignaturesOpen && (
        <SignaturesModal
          onClose={() => setIsSignaturesOpen(false)}
          signatures={signatures}
          isLoading={isLoadingSignatures}
          onSaveSignature={handleSaveSignature}
          onDeleteSignature={handleDeleteSignature}
          onToggleSignature={handleToggleSignature}
        />
      )}
    </div>
  );
};
