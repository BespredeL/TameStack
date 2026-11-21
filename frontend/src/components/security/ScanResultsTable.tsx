import React from 'react';
import { AlertTriangle, ShieldAlert, CheckCircle2, Check, Copy, Edit3, Shield, Trash2 } from 'lucide-react';
import { security } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';

interface ScanResultsTableProps {
  scanResult: security.ScanFilesResult;
  displayedFiles: security.ScannedFile[];
  threatFilter: 'all' | 'threats' | 'clean';
  setThreatFilter: React.Dispatch<React.SetStateAction<'all' | 'threats' | 'clean'>>;
  tableSearch: string;
  setTableSearch: React.Dispatch<React.SetStateAction<string>>;
  hideExcluded: boolean;
  setHideExcluded: React.Dispatch<React.SetStateAction<boolean>>;
  copiedPath: string | null;
  onCopy: (text: string) => void;
  onView: (file: security.ScannedFile) => void;
  onQuickExclusion: (path: string) => void;
  onDelete: (path: string) => void;
}

export const ScanResultsTable: React.FC<ScanResultsTableProps> = ({
  scanResult,
  displayedFiles,
  threatFilter,
  setThreatFilter,
  tableSearch,
  setTableSearch,
  hideExcluded,
  setHideExcluded,
  copiedPath,
  onCopy,
  onView,
  onQuickExclusion,
  onDelete,
}) => {
  const { t } = useTranslation();

  return (
    <div className="bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm space-y-0">
      {/* Results Summary Bar */}
      <div className="p-4 border-b border-slate-800 bg-slate-900/50 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="text-xs text-slate-400">
            {t('fileScanner.filesFound')}{' '}
            <span className="text-white font-bold font-mono">{scanResult.scanned_count}</span>
          </span>
          <span className="text-slate-600">•</span>
          <span className="text-xs text-slate-400">
            {t('fileScanner.threatsFound')}{' '}
            <span
              className={`font-bold font-mono px-2 py-0.5 rounded ${
                scanResult.threat_count > 0
                  ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                  : 'bg-emerald-500/10 text-emerald-400'
              }`}
            >
              {scanResult.threat_count}
            </span>
          </span>
          <span className="text-slate-600">•</span>
          <span className="text-xs text-slate-500 font-mono">
            {t('fileScanner.scanDurationMs', { ms: scanResult.duration_ms })}
          </span>
        </div>

        {/* Filter controls */}
        <div className="flex items-center gap-2.5">
          <div className="flex items-center gap-1 bg-slate-950 p-0.5 rounded-lg border border-slate-800 text-xs">
            <button
              onClick={() => setThreatFilter('all')}
              className={`px-2.5 py-1 rounded text-xs font-semibold transition ${
                threatFilter === 'all' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'
              }`}
            >
              {t('fileScanner.filterAll', { count: (scanResult.files || []).length })}
            </button>
            <button
              onClick={() => setThreatFilter('threats')}
              className={`px-2.5 py-1 rounded text-xs font-semibold transition flex items-center gap-1 ${
                threatFilter === 'threats'
                  ? 'bg-rose-600 text-white'
                  : 'text-rose-400 hover:text-rose-300'
              }`}
            >
              <AlertTriangle className="w-3 h-3" />
              {t('fileScanner.filterThreats', { count: scanResult.threat_count })}
            </button>
            <button
              onClick={() => setThreatFilter('clean')}
              className={`px-2.5 py-1 rounded text-xs font-semibold transition ${
                threatFilter === 'clean' ? 'bg-emerald-600 text-white' : 'text-emerald-400 hover:text-emerald-300'
              }`}
            >
              {t('fileScanner.filterClean')}
            </button>
          </div>

          <input
            type="text"
            placeholder={t('fileScanner.pathFilter')}
            value={tableSearch}
            onChange={(e) => setTableSearch(e.target.value)}
            className="px-2.5 py-1 bg-slate-900 border border-slate-800 rounded-lg text-xs text-white font-mono focus:outline-none focus:border-indigo-500 w-36 sm:w-48"
          />

          <label className="flex items-center gap-1.5 text-xs text-slate-400 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={hideExcluded}
              onChange={(e) => setHideExcluded(e.target.checked)}
              className="rounded border-slate-700 bg-slate-900 text-indigo-600 focus:ring-0 cursor-pointer"
            />
            <span>{t('fileScanner.hideExcluded')}</span>
          </label>
        </div>
      </div>

      {/* Files Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse text-xs">
          <thead>
            <tr className="border-b border-slate-800 bg-slate-950/70 text-slate-400 font-semibold uppercase tracking-wider text-[11px]">
              <th className="py-3 px-4">{t('fileScanner.threatStatus')}</th>
              <th className="py-3 px-4">{t('fileScanner.fileAndPath')}</th>
              <th className="py-3 px-4">{t('fileScanner.sizeAndPerms')}</th>
              <th className="py-3 px-4">{t('fileScanner.modTime')}</th>
              <th className="py-3 px-4 text-right">{t('fileScanner.actions')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 font-sans">
            {displayedFiles.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-12 text-center text-slate-500 text-xs">
                  {(scanResult.files || []).length === 0
                    ? t('fileScanner.noFilesMatched')
                    : t('fileScanner.noFilterMatches')}
                </td>
              </tr>
            ) : (
              displayedFiles.map((file) => {
                const isMalicious = file.threat_level === 'malicious';
                const isSuspicious = file.threat_level === 'suspicious';

                return (
                  <tr
                    key={file.path}
                    className={`hover:bg-slate-800/30 transition-colors ${
                      isMalicious
                        ? 'bg-rose-500/5'
                        : isSuspicious
                        ? 'bg-amber-500/5'
                        : ''
                    }`}
                  >
                    <td className="py-3 px-4 whitespace-nowrap">
                      {isMalicious ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-rose-500/20 text-rose-300 border border-rose-500/40">
                          <ShieldAlert className="w-3.5 h-3.5 text-rose-400" />
                          {t('fileScanner.threatMalicious')}
                        </span>
                      ) : isSuspicious ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                          <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                          {t('fileScanner.threatSuspicious')}
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-slate-800 text-slate-400 border border-slate-700">
                          <CheckCircle2 className="w-3.5 h-3.5 text-slate-500" />
                          {t('fileScanner.threatClean')}
                        </span>
                      )}
                      {file.is_excluded && (
                        <span className="ml-2 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] bg-slate-800 text-slate-400 border border-slate-700">
                          {t('fileScanner.exclusionsBtn')}
                        </span>
                      )}
                    </td>

                    <td className="py-3 px-4 max-w-md">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-white text-xs font-semibold truncate" title={file.path}>
                          {file.path}
                        </span>
                        <button
                          onClick={() => onCopy(file.path)}
                          className="text-slate-500 hover:text-slate-300 transition shrink-0"
                          title={t('common.copy')}
                        >
                          {copiedPath === file.path ? (
                            <Check className="w-3 h-3 text-emerald-400" />
                          ) : (
                            <Copy className="w-3 h-3" />
                          )}
                        </button>
                      </div>

                      {/* Threat explanations & code snippet */}
                      {file.reasons && file.reasons.length > 0 && (
                        <div className="text-[11px] text-amber-300 font-medium mt-1 space-y-0.5">
                          {file.reasons.map((r, rIdx) => (
                            <div key={rIdx} className="flex items-center gap-1">
                              <span>⚠️</span>
                              <span>{r}</span>
                            </div>
                          ))}
                        </div>
                      )}

                      {file.snippet && (
                        <div className="mt-1.5 p-1.5 rounded bg-slate-950 border border-slate-800 font-mono text-[10px] text-rose-300 truncate max-w-lg">
                          <code>{file.snippet}</code>
                        </div>
                      )}
                    </td>

                    <td className="py-3 px-4 whitespace-nowrap font-mono text-xs">
                      <div className="text-slate-200 font-medium">{file.formatted_size}</div>
                      <div className="text-[11px] text-slate-500">{file.permissions} ({file.owner})</div>
                    </td>

                    <td className="py-3 px-4 whitespace-nowrap font-mono text-xs text-slate-400">
                      {file.mod_time}
                    </td>

                    <td className="py-3 px-4 text-right whitespace-nowrap">
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => onView(file)}
                          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 transition cursor-pointer"
                          title={t('fileScanner.viewBtn')}
                        >
                          <Edit3 className="w-3.5 h-3.5 text-indigo-400" />
                        </button>

                        {!file.is_excluded && (
                          <button
                            onClick={() => onQuickExclusion(file.path)}
                            className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                            title={t('fileScanner.whiteListAdd')}
                          >
                            <Shield className="w-3.5 h-3.5 text-emerald-400" />
                          </button>
                        )}

                        <button
                          onClick={() => onDelete(file.path)}
                          className="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/25 text-rose-400 border border-rose-500/30 transition cursor-pointer"
                          title={t('fileScanner.deleteBtn')}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};