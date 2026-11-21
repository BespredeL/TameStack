import React, { useState, useEffect } from 'react';
import { Folder, File, Plus, AlertTriangle } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { CreateRemoteEntry } from '../../../wailsjs/go/main/App';
import { useTranslation } from '../../i18n/LanguageContext';
import { getErrorMessage } from '../../utils/errors';
import { Button, Input } from '../ui';

interface NewEntryModalProps {
  isOpen: boolean;
  isDir: boolean;
  currentPath: string;
  selectedServerID: string;
  onClose: () => void;
  onCreated: () => void;
}

export const NewEntryModal: React.FC<NewEntryModalProps> = ({
  isOpen,
  isDir,
  currentPath,
  selectedServerID,
  onClose,
  onCreated,
}) => {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setName('');
      setError(null);
      setIsSubmitting(false);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanName = name.trim();
    if (!cleanName) {
      setError(isDir ? t('filemanager.promptDirName') : t('filemanager.promptFileName'));
      return;
    }

    const fullPath = `${currentPath === '/' ? '' : currentPath}/${cleanName}`;
    setIsSubmitting(true);
    setError(null);

    try {
      await CreateRemoteEntry({
        server_id: selectedServerID,
        path: fullPath,
        is_dir: isDir,
      });
      onCreated();
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
      icon={
        isDir ? (
          <Folder className="w-5 h-5 text-amber-400" />
        ) : (
          <File className="w-5 h-5 text-indigo-400" />
        )
      }
      iconClassName={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 border ${
        isDir
          ? 'bg-amber-500/10 border-amber-500/20 text-amber-400'
          : 'bg-indigo-500/10 border-indigo-500/20 text-indigo-400'
      }`}
      title={isDir ? t('filemanager.newFolderTitle') : t('filemanager.newFileTitle')}
      subtitle={currentPath}
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
            {isDir ? t('filemanager.folderNameLabel') : t('filemanager.fileNameLabel')}
          </label>
          <Input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={isDir ? 'e.g. assets, cache, logs' : 'e.g. config.php, index.html, .env'}
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
            disabled={isSubmitting || !name.trim()}
            loading={isSubmitting}
            icon={<Plus className="w-4 h-4" />}
          >
            {isSubmitting ? t('filemanager.creating') : t('filemanager.createBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
