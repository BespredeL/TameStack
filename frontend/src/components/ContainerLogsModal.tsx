import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { RefreshCw, Terminal, Copy, Check } from 'lucide-react';
import { GetDockerContainerLogs } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Select, Button } from './ui';

interface ContainerLogsModalProps {
  serverID: string;
  containerID: string;
  containerName: string;
  onClose: () => void;
}

export const ContainerLogsModal: React.FC<ContainerLogsModalProps> = ({
  serverID,
  containerID,
  containerName,
  onClose,
}) => {
  const { t } = useTranslation();
  const [logs, setLogs] = useState<string>('');
  const [lines, setLines] = useState<number>(200);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isCopied, setIsCopied] = useState<boolean>(false);
  const [autoRefresh, setAutoRefresh] = useState<boolean>(false);

  const fetchLogs = async () => {
    try {
      setIsLoading(true);
      const res = await GetDockerContainerLogs(serverID, containerID, lines);
      setLogs(res || t('docker.containerLogsModal.emptyLog'));
    } catch (err: unknown) {
      setLogs(t('docker.containerLogsModal.readError', { error: getErrorMessage(err) }));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
  }, [serverID, containerID, lines]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchLogs, 3000);
    return () => clearInterval(interval);
  }, [autoRefresh, serverID, containerID, lines]);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(logs);
    setIsCopied(true);
    setTimeout(() => setIsCopied(false), 2000);
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[80vh]"
      icon={<Terminal className="w-5 h-5" />}
      title={t('docker.containerLogsModal.title', { container: containerName })}
      subtitle={<span className="font-mono">ID: {containerID.slice(0, 12)}</span>}
      headerActions={
        <>
          <label className="flex items-center gap-2 text-xs text-slate-400 bg-slate-950 border border-slate-800 px-3 py-1.5 rounded-xl cursor-pointer">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-900"
            />
            {t('docker.containerLogsModal.autoRefresh')}
          </label>

          <div className="w-28">
            <Select
              sizeVariant="sm"
              value={lines}
              onChange={(e) => setLines(Number(e.target.value))}
            >
              <option value={100}>{t('docker.containerLogsModal.lines100')}</option>
              <option value={200}>{t('docker.containerLogsModal.lines200')}</option>
              <option value={500}>{t('docker.containerLogsModal.lines500')}</option>
              <option value={1000}>{t('docker.containerLogsModal.lines1000')}</option>
            </Select>
          </div>

          <Button
            variant="secondary"
            size="icon-sm"
            onClick={fetchLogs}
            loading={isLoading}
            icon={<RefreshCw className="w-4 h-4" />}
            title={t('docker.containerLogsModal.refreshTooltip')}
          />

          <Button
            variant="secondary"
            size="icon-sm"
            onClick={copyToClipboard}
            icon={isCopied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
            title={t('docker.containerLogsModal.copyTooltip')}
          />
        </>
      }
    >
      <div className="flex-1 p-4 bg-slate-950 overflow-y-auto font-mono text-xs text-slate-300 select-text leading-relaxed whitespace-pre-wrap custom-scrollbar">
        {logs}
      </div>
    </ModalShell>
  );
};
