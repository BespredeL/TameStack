import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../../utils/errors';

import { FileText, Trash2, CheckCircle2, AlertTriangle, Loader2, Save } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { CodeEditor } from '../common/CodeEditor';
import { security } from '../../../wailsjs/go/models';
import { ReadSecurityFile, SaveSecurityFile } from '../../../wailsjs/go/main/App';
import { useTranslation } from '../../i18n/LanguageContext';
import { Button } from '../ui';

interface FileViewerModalProps {
  file: security.ScannedFile;
  serverId: string;
  onClose: () => void;
  onDelete: (path: string) => void;
}

export const FileViewerModal: React.FC<FileViewerModalProps> = ({ file, serverId, onClose, onDelete }) => {
  const { t } = useTranslation();
  const [fileContent, setFileContent] = useState('');
  const [isFileLoading, setIsFileLoading] = useState(true);
  const [isSavingFile, setIsSavingFile] = useState(false);
  const [fileSaveSuccess, setFileSaveSuccess] = useState<string | null>(null);
  const [fileSaveError, setFileSaveError] = useState<string | null>(null);

  useEffect(() => {
    setIsFileLoading(true);
    setFileSaveSuccess(null);
    setFileSaveError(null);
    let cancelled = false;
    ReadSecurityFile(serverId, file.path)
      .then((content) => { if (!cancelled) setFileContent(content); })
      .catch((err: any) => {
        if (!cancelled) setFileSaveError(t('fileScanner.readFileError', { error: getErrorMessage(err) }));
      })
      .finally(() => { if (!cancelled) setIsFileLoading(false); });
    return () => { cancelled = true; };
  }, [file.path, serverId]);

  const handleSaveFile = async () => {
    setIsSavingFile(true);
    setFileSaveSuccess(null);
    setFileSaveError(null);
    try {
      await SaveSecurityFile(serverId, file.path, fileContent);
      setFileSaveSuccess(t('fileScanner.fileSavedSuccess'));
      setTimeout(() => setFileSaveSuccess(null), 4000);
    } catch (err: unknown) {
      setFileSaveError(t('fileScanner.fileSaveError', { error: getErrorMessage(err) }));
    } finally {
      setIsSavingFile(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh]"
      icon={<FileText className="w-5 h-5" />}
      iconClassName={`p-2 rounded-xl border shrink-0 ${
        file.threat_level === 'malicious'
          ? 'bg-rose-500/15 border-rose-500/30 text-rose-400'
          : file.threat_level === 'suspicious'
          ? 'bg-amber-500/15 border-amber-500/30 text-amber-400'
          : 'bg-indigo-500/15 border-indigo-500/30 text-indigo-400'
      }`}
      title={<span className="font-mono" title={file.path}>{file.path}</span>}
      subtitle={
        <span className="flex items-center gap-2">
          <span>{t('fileScanner.fileSize', { size: file.formatted_size })}</span>
          <span>•</span>
          <span>{t('fileScanner.fileModTime', { time: file.mod_time })}</span>
          <span>•</span>
          <span>{t('fileScanner.filePermissions', { perms: file.permissions })}</span>
        </span>
      }
      headerActions={
        <Button
          variant="danger"
          size="sm"
          onClick={() => onDelete(file.path)}
          icon={<Trash2 className="w-3.5 h-3.5" />}
          title={t('fileScanner.deleteMaliciousTooltip')}
        >
          {t('fileScanner.deleteBtn')}
        </Button>
      }
    >
      {/* Notification messages - edge-to-edge under header */}
      {fileSaveSuccess && (
        <div className="px-6 py-2.5 bg-emerald-500/10 border-b border-emerald-500/30 text-emerald-300 text-xs flex items-center gap-2 shrink-0">
          <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
          <span>{fileSaveSuccess}</span>
        </div>
      )}
      {fileSaveError && (
        <div className="px-6 py-2.5 bg-rose-500/15 border-b border-rose-500/30 text-rose-300 text-xs flex items-center gap-2 shrink-0">
          <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
          <span>{fileSaveError}</span>
        </div>
      )}

      {/* File Content Editor */}
      <div className="flex-1 min-h-0 flex flex-col bg-slate-950 overflow-hidden">
        {isFileLoading ? (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 space-y-2">
            <Loader2 className="w-8 h-8 animate-spin text-indigo-500" />
            <span className="text-xs">{t('fileScanner.readingFileLoading')}</span>
          </div>
        ) : (
          <CodeEditor
            value={fileContent}
            onChange={setFileContent}
            filePath={file.path}
            onSave={handleSaveFile}
            className="h-full border-t border-slate-800/80"
          />
        )}
      </div>

      {/* Modal Footer */}
      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-between">
        <span className="text-[11px] text-slate-500">
          {t('fileScanner.fileBackupNotice')}
        </span>

        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="md"
            onClick={onClose}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant="primary"
            size="md"
            onClick={handleSaveFile}
            loading={isSavingFile}
            disabled={isFileLoading}
            icon={<Save className="w-3.5 h-3.5" />}
          >
            {t('fileScanner.saveChangesBtn')}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
};
