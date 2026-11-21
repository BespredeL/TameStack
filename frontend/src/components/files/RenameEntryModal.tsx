import React, { useState, useEffect } from 'react';
import { Edit2, AlertTriangle, Check } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { RenameRemoteEntry } from '../../../wailsjs/go/main/App';
import { useTranslation } from '../../i18n/LanguageContext';
import { getErrorMessage } from '../../utils/errors';
import { Button, Input } from '../ui';

interface RenameEntryModalProps {
  isOpen: boolean;
  oldPath: string;
  oldName: string;
  currentPath: string;
  selectedServerID: string;
  onClose: () => void;
  onRenamed: () => void;
}

export const RenameEntryModal: React.FC<RenameEntryModalProps> = ({
  isOpen,
  oldPath,
  oldName,
  currentPath,
  selectedServerID,
  onClose,
  onRenamed,
}) => {
  const { t } = useTranslation();
  const [newName, setNewName] = useState(oldName);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setNewName(oldName);
      setError(null);
      setIsSubmitting(false);
    }
  }, [isOpen, oldName]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanName = newName.trim();
    if (!cleanName || cleanName === oldName) {
      onClose();
      return;
    }

    const newPath = `${currentPath === '/' ? '' : currentPath}/${cleanName}`;
    setIsSubmitting(true);
    setError(null);

    try {
      await RenameRemoteEntry({
        server_id: selectedServerID,
        old_path: oldPath,
        new_path: newPath,
      });
      onRenamed();
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Edit2 className="w-5 h-5 text-indigo-400" />}
      iconClassName="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0"
      title={t('filemanager.renameTitle')}
      subtitle={oldPath}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg flex items-start gap-2.5 text-xs text-rose-300">
            <AlertTriangle className="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1.5">
            {t('filemanager.newNameLabel')}
          </label>
          <Input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="font-mono"
            autoFocus
          />
        </div>

        <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
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
            disabled={isSubmitting || !newName.trim() || newName.trim() === oldName}
            loading={isSubmitting}
            icon={<Check className="w-4 h-4" />}
          >
            {isSubmitting ? t('filemanager.renaming') : t('filemanager.renameBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
