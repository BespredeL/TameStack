import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';
import { Archive, Folder } from 'lucide-react';
import { BackupDatabase } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { TaskRecord } from '../types';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';

interface BackupDatabaseModalProps {
  serverID: string;
  engine: string;
  databaseName: string;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const BackupDatabaseModal: React.FC<BackupDatabaseModalProps> = ({
  serverID,
  engine,
  databaseName,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [targetPath, setTargetPath] = useState('/var/backups/tamestack');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setIsSubmitting(true);
      setError(null);
      const task = await BackupDatabase({
        server_id: serverID,
        engine: engine as any,
        database_name: databaseName,
        target_path: targetPath,
      });
      onTaskCreated(task as any);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Archive className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400 shrink-0"
      title={t('databases.backupModal.title')}
      subtitle={<span className="font-mono">{databaseName} ({engine})</span>}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
            <Folder className="w-3.5 h-3.5 text-slate-400" />
            {t('databases.backupModal.targetDirLabel')}
          </label>
          <Input
            type="text"
            required
            value={targetPath}
            onChange={(e) => setTargetPath(e.target.value)}
            className="font-mono text-xs"
          />
          <p className="text-[11px] text-slate-500 mt-1">
            {t('databases.backupModal.compressionNotice', { file: `${databaseName}_YYYYMMDD_HHMMSS.sql.gz` })}
          </p>
        </div>

        <div className="pt-4 flex justify-end gap-3 border-t border-slate-800">
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={onClose}
            disabled={isSubmitting}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={isSubmitting}
            loading={isSubmitting}
            className="bg-amber-600 hover:bg-amber-500 text-white shadow-amber-600/30"
          >
            {isSubmitting ? t('databases.backupModal.creating') : t('databases.backupModal.createBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
