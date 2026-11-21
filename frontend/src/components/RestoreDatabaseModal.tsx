import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';
import { RotateCcw, FileCode, AlertTriangle } from 'lucide-react';
import { RestoreDatabase } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { TaskRecord } from '../types';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';

interface RestoreDatabaseModalProps {
  serverID: string;
  engine: string;
  databaseName: string;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const RestoreDatabaseModal: React.FC<RestoreDatabaseModalProps> = ({
  serverID,
  engine,
  databaseName,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [dumpFilePath, setDumpFilePath] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dumpFilePath.trim()) {
      setError(t('databases.restoreModal.dumpPathRequired'));
      return;
    }
    if (!confirmed) {
      setError(t('databases.restoreModal.confirmOverwriteRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const task = await RestoreDatabase({
        server_id: serverID,
        engine: engine as any,
        database_name: databaseName,
        dump_file_path: dumpFilePath.trim(),
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
      size="lg"
      icon={<RotateCcw className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0"
      title={t('databases.restoreModal.title')}
      subtitle={<span className="font-mono">{t('databases.restoreModal.targetDbInfo', { db: databaseName, engine })}</span>}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        <div className="p-3.5 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-start gap-3">
          <AlertTriangle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
          <div className="text-xs text-rose-300 leading-relaxed">
            <span className="font-semibold">{t('databases.restoreModal.warningPrefix')}</span> {t('databases.restoreModal.warningText', { db: databaseName })}
          </div>
        </div>

        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
            <FileCode className="w-3.5 h-3.5 text-slate-400" />
            {t('databases.restoreModal.dumpPathLabel')}
          </label>
          <Input
            type="text"
            required
            placeholder="/var/backups/tamestack/mydb_20261113.sql.gz"
            value={dumpFilePath}
            onChange={(e) => setDumpFilePath(e.target.value)}
            className="font-mono text-xs"
          />
        </div>

        <label className="flex items-start gap-2.5 p-2 bg-slate-950/60 border border-slate-800 rounded-xl cursor-pointer">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            className="rounded border-slate-700 text-rose-600 focus:ring-0 bg-slate-950 mt-0.5"
          />
          <span className="text-xs text-slate-300 leading-relaxed select-none">
            {t('databases.restoreModal.confirmCheckbox')}
          </span>
        </label>

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
            variant="danger"
            size="md"
            disabled={isSubmitting || !confirmed}
            loading={isSubmitting}
          >
            {isSubmitting ? t('databases.restoreModal.restoring') : t('databases.restoreModal.confirmRestoreBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
