import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import { RefreshCw, Copy, Check } from 'lucide-react';
import { db } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { Button, Select } from './ui';
import { GetSiteLogs } from '../../wailsjs/go/main/App';
import { highlightCode } from '../utils/prismSetup';

interface SiteLogsTabProps {
  site: db.SiteRecord;
}

export const SiteLogsTab: React.FC<SiteLogsTabProps> = ({ site }) => {
  const { t } = useTranslation();
  const siteWebServer = (site.web_server || 'nginx').toLowerCase();
  const defaultEngine = siteWebServer.includes('apache') && !siteWebServer.includes('nginx') ? 'apache' : 'nginx';
  const [logType, setLogType] = useState<'access' | 'error'>('error');
  const [logLines, setLogLines] = useState<number>(50);
  const [logsOutput, setLogsOutput] = useState<string>('');
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);
  const [isCopied, setIsCopied] = useState(false);
  const [logServer, setLogServer] = useState<'auto' | 'nginx' | 'apache'>(
    siteWebServer.includes('nginx') && siteWebServer.includes('apache') ? 'auto' : defaultEngine === 'apache' ? 'apache' : 'nginx'
  );

  const loadLogs = async () => {
    setIsLoadingLogs(true);
    try {
      const preferred =
        logServer === 'auto'
          ? site.web_server || 'nginx'
          : logServer;
      const out = await GetSiteLogs(site.id, logType, logLines, preferred);
      setLogsOutput(out || t('manageSite.emptyLogs'));
    } catch (err: unknown) {
      setLogsOutput(t('manageSite.readLogsError', { error: getErrorMessage(err) }));
    } finally {
      setIsLoadingLogs(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, [site.id]);

  const copyLogs = () => {
    if (!logsOutput) return;
    navigator.clipboard.writeText(logsOutput);
    setIsCopied(true);
    setTimeout(() => setIsCopied(false), 2000);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 bg-slate-950 p-1 rounded-xl border border-slate-800 text-xs font-semibold">
          <button
            onClick={() => {
              setLogType('error');
              loadLogs();
            }}
            className={`px-3 py-1 rounded-lg transition ${
              logType === 'error' ? 'bg-rose-500/20 text-rose-300' : 'text-slate-400 hover:text-white'
            }`}
          >
            {t('manageSite.logTabError')}
          </button>
          <button
            onClick={() => {
              setLogType('access');
              loadLogs();
            }}
            className={`px-3 py-1 rounded-lg transition ${
              logType === 'access' ? 'bg-indigo-500/20 text-indigo-300' : 'text-slate-400 hover:text-white'
            }`}
          >
            {t('manageSite.logTabAccess')}
          </button>
        </div>

        <div className="flex items-center gap-3">
          <div className="w-36">
            <Select
              sizeVariant="sm"
              value={logServer}
              onChange={(e) => {
                setLogServer(e.target.value as 'auto' | 'nginx' | 'apache');
                setTimeout(() => loadLogs(), 0);
              }}
              title={t('manageSite.logStackTooltip')}
            >
              <option value="auto">{t('manageSite.autoStackOption', { server: site.web_server || 'detect' })}</option>
              <option value="nginx">Nginx</option>
              <option value="apache">Apache</option>
            </Select>
          </div>
          <div className="w-24">
            <Select
              sizeVariant="sm"
              value={logLines}
              onChange={(e) => {
                setLogLines(Number(e.target.value));
                loadLogs();
              }}
            >
              <option value={50}>{t('manageSite.linesCountOption', { count: 50 })}</option>
              <option value={100}>{t('manageSite.linesCountOption', { count: 100 })}</option>
              <option value={200}>{t('manageSite.linesCountOption', { count: 200 })}</option>
            </Select>
          </div>

          <Button
            size="sm"
            variant="secondary"
            onClick={loadLogs}
            disabled={isLoadingLogs}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${isLoadingLogs ? 'animate-spin' : ''}`} />}
            title={t('common.refresh')}
          />
        </div>
      </div>

      <div className="bg-slate-950 border border-slate-800 rounded-xl overflow-hidden shadow-inner">
        <div className="px-3 py-1.5 bg-slate-900/70 border-b border-slate-800 flex items-center justify-between text-xs text-slate-400">
          <span className="font-mono text-[11px] text-slate-400">
            {site.domain} • {logType === 'error' ? 'error.log' : 'access.log'} ({logLines} lines)
          </span>
          <Button
            size="sm"
            variant="ghost"
            onClick={copyLogs}
            icon={isCopied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            title={t('common.copy')}
            className="h-7 text-[11px]"
          >
            {isCopied ? t('common.copied') : t('common.copy')}
          </Button>
        </div>
        <div className="p-4 overflow-x-auto max-h-[440px] font-mono text-[11px] leading-relaxed select-text custom-scrollbar">
          <pre className="prism-tamestack m-0 p-0 bg-transparent">
            <code
              className="language-log"
              dangerouslySetInnerHTML={{ __html: highlightCode(logsOutput, 'log') }}
            />
          </pre>
        </div>
      </div>
    </div>
  );
};
