import React, { useState, useEffect } from 'react';
import { ShieldAlert, ShieldCheck, Zap, Info, Clock, AlertCircle, FileText, Check } from 'lucide-react';
import { SaveFail2banJail } from '../../../wailsjs/go/main/App';
import { security } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { ModalShell } from '../common/ModalShell';
import { Input, Button, Badge } from '../ui';

interface ConfigureFail2banJailModalProps {
  isOpen: boolean;
  onClose: () => void;
  serverID: string;
  initialJail?: security.Fail2banJail | null;
  availableFilters?: string[];
  onSaved: () => void;
}

interface JailPreset {
  id: string;
  labelKey: string;
  name: string;
  port: string;
  filter: string;
  logPath: string;
  maxRetry: number;
  findTime: string;
  banTime: string;
  ignoreIP?: string;
}

const PRESETS: JailPreset[] = [
  {
    id: 'sshd',
    labelKey: 'security.fail2banModal.presetSSH',
    name: 'sshd',
    port: 'ssh',
    filter: 'sshd',
    logPath: '/var/log/auth.log',
    maxRetry: 5,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'nginx-http-auth',
    labelKey: 'security.fail2banModal.presetNginxAuth',
    name: 'nginx-http-auth',
    port: 'http,https',
    filter: 'nginx-http-auth',
    logPath: '/var/log/nginx/*error.log',
    maxRetry: 5,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'nginx-botsearch',
    labelKey: 'security.fail2banModal.presetNginxBotsearch',
    name: 'nginx-botsearch',
    port: 'http,https',
    filter: 'nginx-botsearch',
    logPath: '/var/log/nginx/*access.log',
    maxRetry: 2,
    findTime: '10m',
    banTime: '24h',
  },
  {
    id: 'nginx-bad-request',
    labelKey: 'security.fail2banModal.presetNginxBadrequest',
    name: 'nginx-bad-request',
    port: 'http,https',
    filter: 'nginx-bad-request',
    logPath: '/var/log/nginx/*access.log',
    maxRetry: 3,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'nginx-limit-req',
    labelKey: 'security.fail2banModal.presetNginxLimitReq',
    name: 'nginx-limit-req',
    port: 'http,https',
    filter: 'nginx-limit-req',
    logPath: '/var/log/nginx/*error.log',
    maxRetry: 10,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'apache-auth',
    labelKey: 'security.fail2banModal.presetApacheAuth',
    name: 'apache-auth',
    port: 'http,https',
    filter: 'apache-auth',
    logPath: '/var/log/apache2/*error.log',
    maxRetry: 5,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'apache-badbots',
    labelKey: 'security.fail2banModal.presetApacheBadbots',
    name: 'apache-badbots',
    port: 'http,https',
    filter: 'apache-badbots',
    logPath: '/var/log/apache2/*access.log',
    maxRetry: 2,
    findTime: '10m',
    banTime: '24h',
  },
  {
    id: 'mysqld-auth',
    labelKey: 'security.fail2banModal.presetMySQL',
    name: 'mysqld-auth',
    port: '3306',
    filter: 'mysqld-auth',
    logPath: '/var/log/mysql/error.log',
    maxRetry: 5,
    findTime: '10m',
    banTime: '1h',
  },
  {
    id: 'wordpress',
    labelKey: 'security.fail2banModal.presetWordPress',
    name: 'wordpress',
    port: 'http,https',
    filter: 'wordpress',
    logPath: '/var/log/nginx/*access.log',
    maxRetry: 5,
    findTime: '10m',
    banTime: '2h',
  },
  {
    id: 'custom',
    labelKey: 'security.fail2banModal.presetCustom',
    name: '',
    port: '',
    filter: '',
    logPath: '',
    maxRetry: 5,
    findTime: '10m',
    banTime: '1h',
  },
];

const COMMON_LOG_PATHS = [
  '/var/log/auth.log',
  '/var/log/nginx/*error.log',
  '/var/log/nginx/*access.log',
  '/var/log/apache2/*error.log',
  '/var/log/mysql/error.log',
];

export const ConfigureFail2banJailModal: React.FC<ConfigureFail2banJailModalProps> = ({
  isOpen,
  onClose,
  serverID,
  initialJail,
  availableFilters = [],
  onSaved,
}) => {
  const { t } = useTranslation();
  const isEditing = Boolean(initialJail);

  const [selectedPreset, setSelectedPreset] = useState<string>('custom');
  const [name, setName] = useState<string>('');
  const [enabled, setEnabled] = useState<boolean>(true);
  const [port, setPort] = useState<string>('ssh');
  const [filter, setFilter] = useState<string>('sshd');
  const [logPath, setLogPath] = useState<string>('/var/log/auth.log');
  const [maxRetry, setMaxRetry] = useState<number>(5);
  const [findTime, setFindTime] = useState<string>('10m');
  const [banTime, setBanTime] = useState<string>('1h');
  const [ignoreIP, setIgnoreIP] = useState<string>('');
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialJail) {
      setName(initialJail.name || '');
      setEnabled(initialJail.enabled ?? true);

      // Sanitize port from any runtime error leak
      let cleanPort = (initialJail.port || '').trim();
      if (!cleanPort || cleanPort.toLowerCase().includes('invalid') || cleanPort.includes('(')) {
        if (initialJail.name === 'sshd') cleanPort = 'ssh';
        else if (initialJail.name.startsWith('nginx') || initialJail.name.startsWith('apache')) cleanPort = 'http,https';
        else if (initialJail.name.startsWith('mysql')) cleanPort = '3306';
        else cleanPort = 'ssh';
      }
      setPort(cleanPort);

      // Sanitize filter
      let cleanFilter = (initialJail.filter || '').trim();
      if (!cleanFilter || cleanFilter.toLowerCase().includes('invalid')) {
        cleanFilter = initialJail.name || 'sshd';
      }
      setFilter(cleanFilter);

      // Sanitize logpath
      let cleanLogPath = (initialJail.log_path || '').trim();
      if (cleanLogPath.includes('Current monitored') || cleanLogPath.toLowerCase().includes('invalid')) {
        cleanLogPath = initialJail.name === 'sshd' ? '/var/log/auth.log' : '';
      }
      setLogPath(cleanLogPath);

      setMaxRetry(initialJail.max_retry > 0 ? initialJail.max_retry : 5);
      const cleanFind = (initialJail.find_time || '').trim();
      setFindTime(cleanFind && !cleanFind.toLowerCase().includes('invalid') ? cleanFind : '10m');

      const cleanBan = (initialJail.ban_time || '').trim();
      setBanTime(cleanBan && !cleanBan.toLowerCase().includes('invalid') ? cleanBan : '1h');

      const cleanIgnore = (initialJail.ignoreip || '').trim();
      setIgnoreIP(cleanIgnore && !cleanIgnore.toLowerCase().includes('invalid') ? cleanIgnore : '');

      setSelectedPreset('custom');
    } else {
      setSelectedPreset('sshd');
      const p = PRESETS.find((pr) => pr.id === 'sshd') || PRESETS[0];
      setName(p.name);
      setEnabled(true);
      setPort(p.port);
      setFilter(p.filter);
      setLogPath(p.logPath);
      setMaxRetry(p.maxRetry);
      setFindTime(p.findTime);
      setBanTime(p.banTime);
      setIgnoreIP('');
    }
    setError(null);
  }, [isOpen, initialJail]);

  const handlePresetChange = (presetId: string) => {
    setSelectedPreset(presetId);
    const p = PRESETS.find((pr) => pr.id === presetId);
    if (!p) return;
    if (p.id !== 'custom') {
      setName(p.name);
      setPort(p.port);
      setFilter(p.filter);
      setLogPath(p.logPath);
      setMaxRetry(p.maxRetry);
      setFindTime(p.findTime);
      setBanTime(p.banTime);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError(t('security.fail2banModal.nameRequired'));
      return;
    }
    if (!/^[a-zA-Z0-9_-]+$/.test(name.trim())) {
      setError(t('security.fail2banModal.nameInvalid'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);

      const req = new security.SaveFail2banJailRequest({
        server_id: serverID,
        name: name.trim(),
        enabled,
        port: port.trim() || 'ssh',
        filter: filter.trim() || name.trim(),
        log_path: logPath.trim(),
        max_retry: Number(maxRetry) || 5,
        find_time: findTime.trim() || '10m',
        ban_time: banTime.trim() || '1h',
        ignoreip: ignoreIP.trim(),
      });

      await SaveFail2banJail(req);
      onSaved();
      onClose();
    } catch (err: any) {
      setError(err?.toString() || 'Failed to save jail');
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      icon={<ShieldAlert className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0"
      title={
        isEditing
          ? t('security.fail2banModal.editTitle', { name })
          : t('security.fail2banModal.createTitle')
      }
      subtitle={
        <span className="text-slate-400 font-mono text-[11px]">
          {t('security.fail2banModal.configSaveHint', { name: name || 'jail' })}
        </span>
      }
    >
      <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
        {/* Form Body with standard padding */}
        <div className="p-6 space-y-5 overflow-y-auto max-h-[calc(85vh-130px)] custom-scrollbar">
          {error && (
            <div className="p-3.5 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center gap-3 text-rose-300 text-xs">
              <AlertCircle className="w-4 h-4 shrink-0 text-rose-400" />
              <span className="flex-1 font-mono">{error}</span>
            </div>
          )}

          {/* Preset Selector */}
          {!isEditing && (
            <div className="space-y-2">
              <label className="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
                <Zap className="w-3.5 h-3.5 text-indigo-400" />
                <span>{t('security.fail2banModal.presetLabel')}</span>
              </label>
              <div className="flex flex-wrap gap-1.5">
                {PRESETS.map((p) => {
                  const isSelected = selectedPreset === p.id;
                  return (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => handlePresetChange(p.id)}
                      className={`px-3 py-1.5 rounded-xl border text-xs font-medium transition cursor-pointer ${
                        isSelected
                          ? 'bg-indigo-600 border-indigo-500 text-white shadow-sm shadow-indigo-600/30'
                          : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                      }`}
                    >
                      {p.name ? p.name : t(p.labelKey)}
                    </button>
                  );
                })}
              </div>
              <p className="text-[11px] text-slate-400 flex items-center gap-1">
                <Info className="w-3 h-3 text-slate-500 shrink-0" />
                <span>{t('security.fail2banModal.presetDesc')}</span>
              </p>
            </div>
          )}

          {/* Status Toggle Card */}
          <div className="flex items-center justify-between p-3.5 bg-slate-950/60 border border-slate-800 rounded-xl">
            <div className="flex items-center gap-3">
              <div
                className={`w-9 h-9 rounded-xl flex items-center justify-center border transition ${
                  enabled
                    ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                    : 'bg-slate-800/50 border-slate-700/60 text-slate-500'
                }`}
              >
                <ShieldCheck className="w-4 h-4" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-slate-200">
                    {t('security.fail2banModal.statusLabel')}
                  </span>
                  <Badge variant={enabled ? 'success' : 'neutral'} size="sm">
                    <span
                      className={`w-1.5 h-1.5 rounded-full mr-1.5 ${
                        enabled ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                      }`}
                    />
                    {enabled ? t('security.fail2banModal.statusActive') : t('security.fail2banModal.statusDisabled')}
                  </Badge>
                </div>
                <div className="text-[11px] text-slate-400 mt-0.5">
                  {enabled
                    ? t('security.fail2banModal.statusActiveDesc')
                    : t('security.fail2banModal.statusDisabledDesc')}
                </div>
              </div>
            </div>
            <button
              type="button"
              onClick={() => setEnabled(!enabled)}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none cursor-pointer ${
                enabled ? 'bg-emerald-500' : 'bg-slate-700'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                  enabled ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* Jail Name */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-300 block">
              {t('security.fail2banModal.nameLabel')} <span className="text-rose-400">*</span>
            </label>
            <Input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isEditing}
              placeholder="e.g. sshd, nginx-botsearch"
              className="font-mono"
            />
          </div>

          {/* Port & Filter */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 items-end">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-300 block truncate" title={t('security.fail2banModal.portLabel')}>
                {t('security.fail2banModal.portLabel')}
              </label>
              <Input
                type="text"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="ssh, http,https, 80,443"
                className="font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-300 block truncate" title={t('security.fail2banModal.filterLabel')}>
                {t('security.fail2banModal.filterLabel')}
              </label>
              <Input
                type="text"
                list="fail2ban-available-filters"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder="e.g. sshd, nginx-botsearch"
                className="font-mono"
              />
              {availableFilters.length > 0 && (
                <datalist id="fail2ban-available-filters">
                  {availableFilters.map((f) => (
                    <option key={f} value={f} />
                  ))}
                </datalist>
              )}
            </div>
          </div>

          {/* Log Path */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-300 block">
              {t('security.fail2banModal.logPathLabel')}
            </label>
            <Input
              type="text"
              value={logPath}
              onChange={(e) => setLogPath(e.target.value)}
              placeholder="e.g. /var/log/auth.log or /var/log/nginx/*error.log"
              className="font-mono"
            />
            {/* Quick Log Suggestions */}
            <div className="flex flex-wrap items-center gap-1.5 pt-1 text-[11px]">
              <span className="text-slate-500 flex items-center gap-1">
                <FileText className="w-3 h-3" />
                {t('security.fail2banModal.quickLogs')}
              </span>
              {COMMON_LOG_PATHS.map((lp) => (
                <button
                  key={lp}
                  type="button"
                  onClick={() => setLogPath(lp)}
                  className={`px-2 py-0.5 rounded-lg border text-[10px] font-mono transition cursor-pointer ${
                    logPath === lp
                      ? 'bg-indigo-600/20 border-indigo-500/40 text-indigo-300'
                      : 'bg-slate-950 border-slate-800 text-slate-400 hover:text-slate-200 hover:border-slate-700'
                  }`}
                >
                  {lp}
                </button>
              ))}
            </div>
          </div>

          {/* Protection Rules Container (Max Retry, Find Time, Ban Time) */}
          <div className="p-4 bg-slate-950/60 border border-slate-800 rounded-xl space-y-3">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 items-end">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300 block truncate" title={t('security.fail2banModal.maxRetryLabel')}>
                  {t('security.fail2banModal.maxRetryLabel')}
                </label>
                <Input
                  type="number"
                  min={1}
                  max={100}
                  value={maxRetry}
                  onChange={(e) => setMaxRetry(parseInt(e.target.value, 10) || 5)}
                  className="font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300 block truncate" title={t('security.fail2banModal.findTimeLabel')}>
                  {t('security.fail2banModal.findTimeLabel')}
                </label>
                <Input
                  type="text"
                  value={findTime}
                  onChange={(e) => setFindTime(e.target.value)}
                  placeholder="10m, 1h"
                  className="font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-300 block truncate" title={t('security.fail2banModal.banTimeLabel')}>
                  {t('security.fail2banModal.banTimeLabel')}
                </label>
                <Input
                  type="text"
                  value={banTime}
                  onChange={(e) => setBanTime(e.target.value)}
                  placeholder="1h, 24h, 7d"
                  className="font-mono"
                />
              </div>
            </div>

            {/* Quick Ban Duration Pills */}
            <div className="flex flex-wrap items-center gap-1.5 pt-1 text-[11px] border-t border-slate-800/80">
              <span className="text-slate-400 mr-1 flex items-center gap-1">
                <Clock className="w-3 h-3 text-slate-500" />
                {t('security.fail2banModal.quickDurations')}:
              </span>
              {['10m', '1h', '24h', '7d', '30d', '-1'].map((dur) => (
                <button
                  key={dur}
                  type="button"
                  onClick={() => setBanTime(dur)}
                  className={`px-2.5 py-1 rounded-lg border text-[11px] font-mono transition cursor-pointer ${
                    banTime === dur
                      ? 'bg-indigo-600 border-indigo-500 text-white font-semibold shadow-sm shadow-indigo-600/30'
                      : 'bg-slate-900/80 border-slate-800 text-slate-400 hover:text-slate-200 hover:border-slate-700'
                  }`}
                >
                  {dur === '-1' ? '∞ ' + t('security.fail2banModal.permanent') : dur}
                </button>
              ))}
            </div>
          </div>

          {/* Ignore IP */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-300 block">
              {t('security.fail2banModal.ignoreIPLabel')}
            </label>
            <Input
              type="text"
              value={ignoreIP}
              onChange={(e) => setIgnoreIP(e.target.value)}
              placeholder="e.g. 192.168.1.100 10.0.0.0/8"
              className="font-mono"
            />
            {t('security.fail2banModal.ignoreIPHint') && (
              <p className="text-[11px] text-slate-500">
                {t('security.fail2banModal.ignoreIPHint')}
              </p>
            )}
          </div>
        </div>

        {/* Modal Sticky Footer */}
        <div className="px-6 py-4 bg-slate-950/60 border-t border-slate-800 flex items-center justify-between shrink-0">
          <span className="text-[11px] text-slate-500 font-mono hidden sm:inline">
            fail2ban-client v0.11+
          </span>

          <div className="flex items-center gap-2.5 ml-auto">
            <Button
              variant="secondary"
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
              loading={isSubmitting}
              icon={<Check className="w-3.5 h-3.5" />}
            >
              {isEditing ? t('common.save') : t('security.fail2banModal.saveBtn')}
            </Button>
          </div>
        </div>
      </form>
    </ModalShell>
  );
};
