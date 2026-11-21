import React from 'react';
import { Camera, Sparkles, Loader2, GitCompare, Trash2 } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { security } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { Input, Button } from '../ui';

interface SnapshotsModalProps {
  onClose: () => void;
  basePath: string;
  setBasePath: React.Dispatch<React.SetStateAction<string>>;
  snapshotName: string;
  setSnapshotName: React.Dispatch<React.SetStateAction<string>>;
  isCreatingSnapshot: boolean;
  snapshots: security.FileSecuritySnapshot[];
  isLoadingSnapshots: boolean;
  isComparing: boolean;
  onCreateSnapshot: () => void;
  onCompare: (snapshotId: string) => void;
  onDeleteSnapshot: (snapshotId: string) => void;
}

export const SnapshotsModal: React.FC<SnapshotsModalProps> = ({
  onClose,
  basePath,
  setBasePath,
  snapshotName,
  setSnapshotName,
  isCreatingSnapshot,
  snapshots,
  isLoadingSnapshots,
  isComparing,
  onCreateSnapshot,
  onCompare,
  onDeleteSnapshot,
}) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="4xl"
      className="max-h-[85vh]"
      icon={<Camera className="w-5 h-5 text-indigo-400" />}
      iconClassName=""
      title={t('fileScanner.snapshotModalTitle')}
      subtitle={t('fileScanner.snapshotsSubtitle')}
    >
      <div className="p-6 overflow-y-auto space-y-6 flex-1">
        {/* Create snapshot box */}
        <div className="p-4 bg-slate-950/60 rounded-xl border border-slate-800 space-y-3">
          <h4 className="text-xs font-bold text-white flex items-center gap-1.5">
            <Sparkles className="w-4 h-4 text-indigo-400" />
            <span>{t('fileScanner.createNewSnapshotBtn')}</span>
          </h4>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.snapshotDirLabel')}</label>
              <Input
                type="text"
                sizeVariant="sm"
                value={basePath}
                onChange={(e) => setBasePath(e.target.value)}
                className="font-mono"
              />
            </div>
            <div>
              <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.snapshotNameLabel')}</label>
              <Input
                type="text"
                sizeVariant="sm"
                placeholder={t('fileScanner.snapshotNamePlaceholder')}
                value={snapshotName}
                onChange={(e) => setSnapshotName(e.target.value)}
              />
            </div>
          </div>
          <div className="flex items-center gap-3 pt-1">
            <Button
              variant="primary"
              size="sm"
              onClick={onCreateSnapshot}
              loading={isCreatingSnapshot}
              disabled={!basePath}
              icon={<Camera className="w-3.5 h-3.5" />}
            >
              {isCreatingSnapshot ? t('fileScanner.snapshotCreatingBtn') : t('fileScanner.snapshotCommitBtn')}
            </Button>

            {isCreatingSnapshot && (
              <Button
                variant="secondary"
                size="sm"
                onClick={onClose}
              >
                {t('fileScanner.minimizeToBackgroundBtn')}
              </Button>
            )}
          </div>
          <p className="text-[11px] text-slate-400 leading-relaxed">
            {t('fileScanner.snapshotNotice')}
          </p>
        </div>

        {/* Snapshots List */}
        <div className="space-y-3">
          <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">
            {t('fileScanner.savedSnapshotsHeader', { count: snapshots.length })}
          </h4>
          {isLoadingSnapshots ? (
            <div className="py-8 text-center text-slate-400 text-xs flex items-center justify-center gap-2">
              <Loader2 className="w-4 h-4 animate-spin text-indigo-500" />
              <span>{t('fileScanner.loadingSnapshots')}</span>
            </div>
          ) : snapshots.length === 0 ? (
            <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">
              {t('fileScanner.emptySnapshotsText')}
            </div>
          ) : (
            <div className="space-y-2.5">
              {snapshots.map((s) => (
                <div
                  key={s.id}
                  className="p-4 rounded-xl bg-slate-950 border border-slate-800 hover:border-slate-700 flex flex-col sm:flex-row sm:items-center justify-between gap-4 transition"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <Camera className="w-4 h-4 text-indigo-400 shrink-0" />
                      <span className="font-bold text-sm text-white">{s.name}</span>
                    </div>
                    <div className="text-xs text-slate-400 font-mono mt-1">
                      {t('fileScanner.snapshotDirText', { path: s.base_path })}
                    </div>
                    <div className="flex items-center gap-3 text-[11px] text-slate-500 mt-1">
                      <span>{t('fileScanner.snapshotFilesCount', { count: s.files_count })}</span>
                      <span>•</span>
                      <span>{t('fileScanner.snapshotCreatedAt', { date: s.created_at })}</span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => onCompare(s.id)}
                      disabled={isComparing}
                      icon={<GitCompare className="w-3.5 h-3.5 text-indigo-400" />}
                      className="bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border-indigo-500/30"
                      title={t('fileScanner.compareWithSnapshotTooltip')}
                    >
                      {t('fileScanner.compareWithSnapshotBtn')}
                    </Button>
                    <button
                      onClick={() => onDeleteSnapshot(s.id)}
                      className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition cursor-pointer"
                      title={t('fileScanner.deleteSnapshotTooltip')}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="px-6 py-3 border-t border-slate-800 bg-slate-950 flex items-center justify-between text-xs">
        <span className="text-slate-400">
          {isCreatingSnapshot ? (
            <span className="flex items-center gap-1.5 text-indigo-400">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
              <span>{t('fileScanner.indexingInBackground')}</span>
            </span>
          ) : (
            <span>{t('fileScanner.snapshotsCountFooter', { count: snapshots.length })}</span>
          )}
        </span>
        <Button
          variant="secondary"
          size="sm"
          onClick={onClose}
        >
          {t('fileScanner.closeWindowBtn')}
        </Button>
      </div>
    </ModalShell>
  );
};
