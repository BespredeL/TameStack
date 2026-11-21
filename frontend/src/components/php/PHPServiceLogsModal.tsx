import React, { useState, useEffect } from 'react';
import { GetPHPFPMServiceLogs, CheckPHPFPMConfig } from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { getErrorMessage } from '../../utils/errors';
import { useTranslation } from '../../i18n/LanguageContext';
import { RefreshCw, CheckCircle2, Copy, Check, Terminal, AlertCircle } from 'lucide-react';
import { Button } from '../ui';

interface PHPServiceLogsModalProps {
  serverId: string;
  version: string;
  isOpen: boolean;
  onClose: () => void;
}

export const PHPServiceLogsModal: React.FC<PHPServiceLogsModalProps> = ({
  serverId,
  version,
  isOpen,
  onClose,
}) => {
  const { t } = useTranslation();
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lines, setLines] = useState(100);
  const [copied, setCopied] = useState(false);

  const [testingConfig, setTestingConfig] = useState(false);
  const [configResult, setConfigResult] = useState<string | null>(null);

  const fetchLogs = async () => {
    if (!serverId || !version) return;
    setLoading(true);
    setError(null);
    try {
      const data = await GetPHPFPMServiceLogs(serverId, version, lines);
      setLogs(data || '(No log records found)');
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const handleTestConfig = async () => {
    setTestingConfig(true);
    try {
      const res = await CheckPHPFPMConfig(serverId, version);
      setConfigResult(res);
    } catch (err: unknown) {
      setConfigResult(`Error: ${getErrorMessage(err)}`);
    } finally {
      setTestingConfig(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      fetchLogs();
      setConfigResult(null);
    }
  }, [isOpen, serverId, version, lines]);

  const handleCopy = () => {
    navigator.clipboard.writeText(logs);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (!isOpen) return null;

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh] flex flex-col"
      title={`${t('php.fpmLogsTitle')}: php${version}-fpm`}
      subtitle={<span className="font-mono">journalctl -u php{version}-fpm -n {lines}</span>}
      icon={<Terminal className="w-5 h-5 text-indigo-400" />}
      headerActions={
        <div className="flex items-center gap-2">
          {/* Line count selector */}
          <div className="flex items-center gap-1.5 bg-slate-900 px-2.5 py-1 rounded-xl border border-slate-800 text-xs">
            <span className="text-slate-400">{t('php.linesCount')}:</span>
            <select
              value={lines}
              onChange={(e) => setLines(Number(e.target.value))}
              className="bg-transparent text-white font-mono focus:outline-none cursor-pointer text-xs"
            >
              <option value={50} className="bg-slate-900">50</option>
              <option value={100} className="bg-slate-900">100</option>
              <option value={200} className="bg-slate-900">200</option>
              <option value={500} className="bg-slate-900">500</option>
            </select>
          </div>

          {/* Test config button */}
          <Button
            variant="secondary"
            size="sm"
            onClick={handleTestConfig}
            loading={testingConfig}
            icon={<CheckCircle2 className="w-3.5 h-3.5 text-indigo-400" />}
            className="bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border-indigo-500/30"
          >
            {testingConfig ? 'php-fpm -t...' : t('php.checkConfigBtn')}
          </Button>

          {/* Refresh button */}
          <Button
            variant="secondary"
            size="sm"
            onClick={fetchLogs}
            loading={loading}
            icon={<RefreshCw className="w-3.5 h-3.5" />}
          >
            {t('common.refresh')}
          </Button>

          {/* Copy button */}
          <Button
            variant="secondary"
            size="sm"
            onClick={handleCopy}
            icon={copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
          >
            {copied ? t('common.copied') : t('common.copy')}
          </Button>
        </div>
      }
    >
      <div className="flex-1 flex flex-col min-h-0 p-6 space-y-3">
        {/* Config check notification */}
        {configResult && (
          <div
            className={`p-3 rounded-xl border text-xs font-mono whitespace-pre-wrap shrink-0 ${
              configResult.includes('successful') || configResult.includes('OK') || configResult.includes('test is successful')
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
            }`}
          >
            {configResult}
          </div>
        )}

        {error && (
          <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2 shrink-0">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Terminal viewer */}
        <div className="flex-1 min-h-0 bg-slate-950 border border-slate-800 rounded-xl p-4 overflow-auto font-mono text-xs text-slate-300 custom-scrollbar select-text whitespace-pre-wrap">
          {logs}
        </div>
      </div>
    </ModalShell>
  );
};
