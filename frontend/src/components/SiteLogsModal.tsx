import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { RefreshCw, Terminal, Loader2, Copy, Check } from 'lucide-react';
import { GetSiteLogs } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button, Badge, Tabs } from './ui';

interface SiteLogsModalProps {
  isOpen: boolean;
  siteID: string;
  domain: string;
  webServer?: string;
  onClose: () => void;
}

export const SiteLogsModal: React.FC<SiteLogsModalProps> = ({
  isOpen,
  siteID,
  domain,
  webServer = 'nginx',
  onClose,
}) => {
  const { t } = useTranslation();
  const [logType, setLogType] = useState<'access' | 'error'>('access');
  const [serverType, setServerType] = useState<'auto' | 'apache' | 'nginx'>('auto');
  const [logs, setLogs] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const isApache = webServer.toLowerCase().includes('apache');
  const isNginx = webServer.toLowerCase().includes('nginx');

  useEffect(() => {
    if (isApache && !isNginx) {
      setServerType('apache');
    } else if (isNginx && !isApache) {
      setServerType('nginx');
    } else {
      setServerType('auto');
    }
  }, [webServer, domain]);

  const activeServer = serverType === 'auto' ? (isApache && !isNginx ? 'apache' : isNginx && !isApache ? 'nginx' : 'auto') : serverType;

  const defaultPath = activeServer === 'apache'
    ? `/var/log/apache2/${domain}.${logType}.log`
    : activeServer === 'nginx'
    ? `/var/log/nginx/${domain}.${logType}.log`
    : `/var/log/apache2/ & /var/log/nginx/`;

  const fetchLogs = () => {
    if (!siteID) return;
    setIsLoading(true);
    (GetSiteLogs as any)(siteID, logType, 100, serverType)
      .then((data: string) => setLogs(data))
      .catch((err: any) => setLogs('Failed to load logs: ' + (getErrorMessage(err))))
      .finally(() => setIsLoading(false));
  };

  useEffect(() => {
    if (isOpen) {
      fetchLogs();
    }
  }, [isOpen, siteID, logType, serverType]);

  const handleCopy = () => {
    if (!logs) return;
    navigator.clipboard.writeText(logs);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (!isOpen) return null;

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh]"
      icon={<Terminal className="w-5 h-5" />}
      iconClassName={`p-2 rounded-lg border ${
        isApache && !isNginx
          ? 'bg-amber-600/20 text-amber-400 border-amber-500/30'
          : 'bg-indigo-600/20 text-indigo-400 border-indigo-500/30'
      }`}
      title={t('manageSite.serverLogsTitle', { domain })}
      headerExtra={
        <Badge
          variant={isApache && isNginx ? 'primary' : isApache ? 'warning' : 'success'}
          size="sm"
          className="uppercase tracking-wider font-semibold"
        >
          {isApache && isNginx ? 'Nginx + Apache' : isApache ? 'Apache' : 'Nginx'}
        </Badge>
      }
      subtitle={<span className="font-mono">{defaultPath}</span>}
      headerActions={
        <>
          {/* Server Selector */}
          <Tabs
            variant="pill"
            size="sm"
            activeTab={serverType}
            onChange={(id) => setServerType(id as any)}
            tabs={[
              { id: 'auto', label: t('manageSite.autoDetect') },
              { id: 'apache', label: 'Apache' },
              { id: 'nginx', label: 'Nginx' },
            ]}
          />

          {/* Log Type Selector */}
          <Tabs
            variant="pill"
            size="sm"
            activeTab={logType}
            onChange={(id) => setLogType(id as any)}
            tabs={[
              { id: 'access', label: 'Access' },
              { id: 'error', label: 'Error' },
            ]}
          />

          <Button
            variant="secondary"
            size="icon-sm"
            onClick={fetchLogs}
            loading={isLoading}
            icon={<RefreshCw className="w-3.5 h-3.5" />}
            title={t('manageSite.refreshLogsTooltip')}
          />

          <Button
            variant="secondary"
            size="icon-sm"
            onClick={handleCopy}
            disabled={!logs}
            icon={copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            title={t('manageSite.copyLogsTooltip')}
          />
        </>
      }
    >
      {/* Log Viewer Content */}
      <div className="flex-1 p-4 bg-slate-950 overflow-y-auto font-mono text-xs text-slate-300 select-text leading-relaxed whitespace-pre-wrap custom-scrollbar">
        {isLoading && !logs ? (
          <div className="h-full flex items-center justify-center gap-2 text-slate-500 text-xs">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span>{t('manageSite.fetchingLogs')}</span>
          </div>
        ) : (
          logs || t('manageSite.emptyLogs')
        )}
      </div>
    </ModalShell>
  );
};
