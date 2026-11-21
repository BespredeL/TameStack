import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Save, Loader2, FileCode, Check, Shield } from 'lucide-react';
import { ReadRemoteFile, SaveRemoteFile } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { CodeEditor } from './common/CodeEditor';
import { Button } from './ui';

interface FileEditorModalProps {
  serverID: string;
  filePath: string;
  onClose: () => void;
  onSaved?: () => void;
}

export const FileEditorModal: React.FC<FileEditorModalProps> = ({
  serverID,
  filePath,
  onClose,
  onSaved,
}) => {
  const { t } = useTranslation();
  const [content, setContent] = useState<string>('');
  const [permissions, setPermissions] = useState<string>('');
  const [createBackup, setCreateBackup] = useState<boolean>(true);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [isSavedSuccess, setIsSavedSuccess] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const fetchContent = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const res = await ReadRemoteFile(serverID, filePath);
      if (res.is_binary) {
        setError(t('filemanager.editor.binaryError'));
      } else {
        setContent(res.content);
        setPermissions(res.permissions);
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchContent();
  }, [serverID, filePath]);

  const handleSave = async () => {
    try {
      setIsSaving(true);
      setError(null);
      await SaveRemoteFile({
        server_id: serverID,
        path: filePath,
        content,
        create_backup: createBackup,
      });
      setIsSavedSuccess(true);
      setTimeout(() => setIsSavedSuccess(false), 2500);
      if (onSaved) onSaved();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh]"
      icon={<FileCode className="w-5 h-5" />}
      title={<span className="font-mono">{filePath}</span>}
      subtitle={permissions ? <span className="font-mono">{t('filemanager.editor.permissions', { perms: permissions })}</span> : undefined}
      headerActions={
        <>
          <label className="flex items-center gap-2 text-xs text-slate-400 bg-slate-950 border border-slate-800 px-3 py-1.5 rounded-xl cursor-pointer">
            <input
              type="checkbox"
              checked={createBackup}
              onChange={(e) => setCreateBackup(e.target.checked)}
              className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-900"
            />
            <Shield className="w-3.5 h-3.5 text-indigo-400" />
            {t('filemanager.editor.createBakCheckbox')}
          </label>

          <Button
            variant="primary"
            size="sm"
            onClick={handleSave}
            loading={isSaving}
            disabled={isLoading}
            icon={isSavedSuccess ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Save className="w-3.5 h-3.5" />}
          >
            {isSavedSuccess ? t('filemanager.editor.saved') : t('filemanager.editor.save')}
          </Button>
        </>
      }
    >
      {error && (
        <div className="px-6 py-2.5 bg-rose-500/15 border-b border-rose-500/30 text-xs text-rose-300 whitespace-pre-wrap shrink-0">
          {error}
        </div>
      )}

      {/* Editor Body */}
      <div className="flex-1 p-3 bg-slate-950 flex flex-col overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center h-full text-slate-500 gap-2">
            <Loader2 className="w-5 h-5 animate-spin" />
            {t('filemanager.editor.loading')}
          </div>
        ) : (
          <CodeEditor
            value={content}
            onChange={setContent}
            filePath={filePath}
            onSave={handleSave}
          />
        )}
      </div>
    </ModalShell>
  );
};
