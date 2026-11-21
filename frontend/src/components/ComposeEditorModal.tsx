import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Play, Square, RefreshCw, FileCode } from 'lucide-react';
import { ReadDockerComposeFile, DockerComposeAction } from '../../wailsjs/go/main/App';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { ModalShell } from './common/ModalShell';
import { CodeEditor } from './common/CodeEditor';
import { Button } from './ui';

interface ComposeEditorModalProps {
  serverID: string;
  workingDir: string;
  projectName: string;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const ComposeEditorModal: React.FC<ComposeEditorModalProps> = ({
  serverID,
  workingDir,
  projectName,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const [content, setContent] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const fetchCompose = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const res = await ReadDockerComposeFile(serverID, workingDir);
      setContent(res || '');
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchCompose();
  }, [serverID, workingDir]);

  const handleAction = async (action: 'up' | 'down' | 'restart', saveCurrentContent: boolean = false) => {
    if (action === 'down' && !(await confirm(t('docker.composeEditorModal.downConfirm', { project: projectName }), { danger: true }))) {
      return;
    }
    try {
      setIsSaving(true);
      setError(null);
      const task = await DockerComposeAction({
        server_id: serverID,
        working_dir: workingDir,
        action,
        content: saveCurrentContent ? content : undefined,
      });
      onTaskCreated(task as any);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh]"
      icon={<FileCode className="w-5 h-5" />}
      title={t('docker.composeEditorModal.title', { project: projectName })}
      subtitle={<span className="font-mono">{workingDir}</span>}
      headerActions={
        <>
          <Button
            variant="success"
            size="sm"
            onClick={() => handleAction('up', true)}
            loading={isSaving}
            disabled={isLoading}
            icon={<Play className="w-3.5 h-3.5" />}
            title={t('docker.composeEditorModal.upTooltip')}
          >
            {t('docker.composeEditorModal.upBtn')}
          </Button>

          <Button
            variant="secondary"
            size="sm"
            onClick={() => handleAction('restart', false)}
            loading={isSaving}
            disabled={isLoading}
            icon={<RefreshCw className="w-3.5 h-3.5 text-indigo-400" />}
            title={t('docker.composeEditorModal.restartTooltip')}
          >
            {t('docker.composeEditorModal.restartBtn')}
          </Button>

          <Button
            variant="danger"
            size="sm"
            onClick={() => handleAction('down', false)}
            loading={isSaving}
            disabled={isLoading}
            icon={<Square className="w-3.5 h-3.5" />}
            title={t('docker.composeEditorModal.downTooltip')}
          >
            {t('docker.composeEditorModal.downBtn')}
          </Button>
        </>
      }
    >
      {error && (
        <div className="px-6 py-2.5 bg-rose-500/15 border-b border-rose-500/30 text-xs text-rose-300 whitespace-pre-wrap shrink-0">
          {error}
        </div>
      )}

      {/* Code editor body */}
      <div className="flex-1 p-3 bg-slate-950 flex flex-col overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center h-full text-slate-500 gap-2">
            <span>{t('docker.composeEditorModal.loadingConfig')}</span>
          </div>
        ) : (
          <CodeEditor
            value={content}
            onChange={setContent}
            language="yaml"
            filePath="docker-compose.yml"
          />
        )}
      </div>
    </ModalShell>
  );
};
