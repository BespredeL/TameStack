import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import { RotateCcw, AlertTriangle, CheckCircle2, ShieldAlert, Folder, Database } from 'lucide-react';
import { RestoreBackup, InspectBackupArchive } from '../../wailsjs/go/main/App';
import { db, backup as backupModel } from '../../wailsjs/go/models';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Button } from './ui';

interface RestoreBackupModalProps {
  serverID: string;
  backup: db.BackupRecord;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const RestoreBackupModal: React.FC<RestoreBackupModalProps> = ({
  serverID,
  backup,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [confirmed, setConfirmed] = useState(false);
  const [allowSystemPaths, setAllowSystemPaths] = useState(false);
  const [targetDirectory, setTargetDirectory] = useState(
    backup.backup_type === 'site' ? `/var/www/${backup.target_name}` : ''
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isInspecting, setIsInspecting] = useState(true);
  const [inspection, setInspection] = useState<backupModel.ArchiveInspectionResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;
    const runInspection = async () => {
      try {
        setIsInspecting(true);
        const res = await InspectBackupArchive(serverID, backup.file_path);
        if (isMounted) {
          setInspection(res);
          if (res?.suggested_root && !targetDirectory) {
            setTargetDirectory(res.suggested_root);
          }
        }
      } catch (err: unknown) {
        console.warn('Archive preflight inspection failed:', err);
      } finally {
        if (isMounted) setIsInspecting(false);
      }
    };

    runInspection();
    return () => {
      isMounted = false;
    };
  }, [serverID, backup.file_path]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!confirmed) {
      setError(t('backups.restoreModal.confirmOverwriteWarning'));
      return;
    }
    if (inspection?.has_system_paths && !allowSystemPaths) {
      setError(t('backups.restoreModal.systemPathsWarning'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const task = await RestoreBackup({
        server_id: serverID,
        backup_id: backup.id,
        file_path: backup.file_path,
        target_type: backup.backup_type,
        target_name: backup.target_name,
        target_directory: targetDirectory.trim(),
        allow_system_paths: allowSystemPaths,
        dry_run: false,
      });
      onTaskCreated(task as any);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
      setIsSubmitting(false);
    }
  };

  const hasDangerousPaths = !!inspection?.has_system_paths;
  const canSubmit = confirmed && (!hasDangerousPaths || allowSystemPaths) && !isSubmitting && !isInspecting;

  return (
    <ModalShell
      onClose={onClose}
      size="xl"
      className="max-h-[92vh]"
      icon={<RotateCcw className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0"
      title={t('backups.restoreModal.title')}
      subtitle={<span className="font-mono">{t('backups.restoreModal.targetInfo', { target: backup.target_name, type: backup.backup_type })}</span>}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4 overflow-y-auto min-h-0 flex-1">
        <div className="p-3.5 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-start gap-3">
          <AlertTriangle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
          <div className="text-xs text-rose-300 leading-relaxed">
            <span className="font-semibold">{t('backups.restoreModal.warningPrefix')}</span> {t('backups.restoreModal.warningText', { path: backup.file_path })}
          </div>
        </div>

        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        {/* Inspection Card */}
        <div className="p-3.5 bg-slate-950 border border-slate-800 rounded-xl space-y-2 text-xs">
          <div className="flex items-center justify-between">
            <span className="text-slate-400 font-medium">{t('backups.restoreModal.integrityCheck')}</span>
            {isInspecting ? (
              <div className="flex items-center gap-1.5 text-indigo-400">
                <Button variant="ghost" size="xs" loading className="h-auto p-0 text-indigo-400">
                  {t('backups.restoreModal.inspecting')}
                </Button>
              </div>
            ) : inspection?.integrity_passed ? (
              <span className="inline-flex items-center gap-1 text-emerald-400 font-semibold">
                <CheckCircle2 className="w-3.5 h-3.5" /> {t('backups.restoreModal.integrityConfirmed')}
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 text-rose-400 font-semibold">
                <AlertTriangle className="w-3.5 h-3.5" /> {t('backups.restoreModal.integrityError')}
              </span>
            )}
          </div>

          {inspection && (
            <div className="grid grid-cols-2 gap-2 pt-2 border-t border-slate-800/80 text-[11px] text-slate-300">
              <div>
                <span className="text-slate-500 block">{t('backups.restoreModal.filesInArchive')}</span>
                <span className="font-mono font-medium">{inspection.total_entries > 0 ? inspection.total_entries : t('backups.restoreModal.notDefined')}</span>
              </div>
              <div>
                <span className="text-slate-500 block">{t('backups.restoreModal.dbDump')}</span>
                <span className="font-medium">
                  {inspection.contains_db_dump ? (
                    <span className="text-emerald-400 flex items-center gap-1">
                      <Database className="w-3 h-3" /> {inspection.db_dump_name || t('backups.restoreModal.included')}
                    </span>
                  ) : t('backups.restoreModal.none')}
                </span>
              </div>
            </div>
          )}
        </div>

        {/* Dangerous System Paths Warning & Protection */}
        {hasDangerousPaths && (
          <div className="p-3.5 bg-amber-500/10 border border-amber-500/30 rounded-xl space-y-2">
            <div className="flex items-start gap-2 text-amber-300 text-xs font-semibold">
              <ShieldAlert className="w-4 h-4 shrink-0 text-amber-400 mt-0.5" />
              <span>{t('backups.restoreModal.systemPathsDetected')}</span>
            </div>
            <p className="text-[11px] text-slate-300 leading-relaxed">
              {t('backups.restoreModal.systemPathsDesc', { paths: inspection?.dangerous_paths?.slice(0, 3).join(', ') || '' })}
            </p>
            <label className="flex items-start gap-2 pt-1 cursor-pointer">
              <input
                type="checkbox"
                checked={allowSystemPaths}
                onChange={(e) => setAllowSystemPaths(e.target.checked)}
                className="rounded border-amber-500/40 text-amber-500 focus:ring-0 bg-slate-900 mt-0.5"
              />
              <span className="text-xs text-amber-200 font-medium">
                {t('backups.restoreModal.allowSystemPaths')}
              </span>
            </label>
          </div>
        )}

        {/* Target Directory for site restore */}
        {backup.backup_type === 'site' && (
          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-slate-300 flex items-center gap-1.5">
              <Folder className="w-3.5 h-3.5 text-indigo-400" />
              <span>{t('backups.restoreModal.docRootLabel')}</span>
            </label>
            <Input
              type="text"
              value={targetDirectory}
              onChange={(e) => setTargetDirectory(e.target.value)}
              placeholder="/var/www/example.com"
              className="font-mono"
            />
            <span className="text-[11px] text-slate-500 block">
              {t('backups.restoreModal.docRootDesc')}
            </span>
          </div>
        )}

        <div className="p-3 bg-slate-950/60 border border-slate-800 rounded-xl space-y-1.5 text-xs">
          <div className="flex justify-between">
            <span className="text-slate-400">{t('backups.restoreModal.archivePath')}</span>
            <span className="font-mono text-slate-200 truncate max-w-xs">{backup.file_path}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-slate-400">{t('backups.restoreModal.created')}</span>
            <span className="text-slate-200">{new Date(backup.created_at).toLocaleString()}</span>
          </div>
        </div>

        <label className="flex items-start gap-2.5 p-2 bg-slate-950/60 border border-slate-800 rounded-xl cursor-pointer">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            className="rounded border-slate-700 text-rose-600 focus:ring-0 bg-slate-950 mt-0.5"
          />
          <span className="text-xs text-slate-300">
            {t('backups.restoreModal.understandRisk')}
          </span>
        </label>

        <div className="pt-4 flex justify-end gap-3 border-t border-slate-800 shrink-0">
          <Button
            variant="secondary"
            size="md"
            onClick={onClose}
            disabled={isSubmitting}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="danger"
            size="md"
            loading={isSubmitting}
            disabled={!canSubmit}
            icon={<RotateCcw className="w-4 h-4" />}
          >
            {isSubmitting ? t('backups.restoreModal.restoring') : t('backups.restoreModal.startRestoreBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
