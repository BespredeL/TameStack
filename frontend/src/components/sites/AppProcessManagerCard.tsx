import React, { useEffect, useState } from 'react';
import { Cpu, Play, Square, RotateCw, Trash2, Save, RefreshCw, Terminal } from 'lucide-react';
import { Button, Input, Badge, Select } from '../ui';
import { useTranslation } from '../../i18n/LanguageContext';
import { useConfirm } from '../../context/ConfirmContext';
import { getErrorMessage } from '../../utils/errors';
import {
  GetAppProcessStatus,
  ConfigureAppProcess,
  ControlAppProcess,
  DeleteAppProcess,
} from '../../../wailsjs/go/main/App';

interface AppProcessManagerCardProps {
  siteId: string;
  documentRoot: string;
}

export const AppProcessManagerCard: React.FC<AppProcessManagerCardProps> = ({
  siteId,
  documentRoot,
}) => {
  const { language } = useTranslation();
  const confirm = useConfirm();
  const isEN = language === 'en';

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<any>(null);

  const [runtime, setRuntime] = useState('nodejs');
  const [workingDir, setWorkingDir] = useState(documentRoot || '/var/www/');
  const [execStart, setExecStart] = useState('/usr/bin/node server.js');
  const [user, setUser] = useState('www-data');
  const [port, setPort] = useState(3000);
  const [envVars, setEnvVars] = useState('NODE_ENV=production\nPORT=3000');

  const loadStatus = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await GetAppProcessStatus(siteId);
      setStatus(res);
      if (res && res.configured) {
        if (res.runtime) setRuntime(res.runtime);
        if (res.working_dir) setWorkingDir(res.working_dir);
        if (res.exec_start) setExecStart(res.exec_start);
        if (res.user) setUser(res.user);
        if (res.port) setPort(res.port);
        if (res.env_vars !== undefined) setEnvVars(res.env_vars);
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadStatus();
  }, [siteId]);

  const handleSaveAndStart = async (e: React.FormEvent) => {
    e.preventDefault();
    const ok = await confirm(
      isEN
        ? 'Deploy systemd unit and restart this application process?'
        : 'Обновить systemd-юнит и перезапустить процесс приложения?'
    );
    if (!ok) return;
    setSaving(true);
    setError(null);
    try {
      await ConfigureAppProcess(siteId, {
        site_id: siteId,
        runtime,
        working_dir: workingDir.trim(),
        exec_start: execStart.trim(),
        user: user.trim() || 'www-data',
        port: Number(port) || 0,
        env_vars: envVars,
      });
      await loadStatus();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleControl = async (action: 'start' | 'stop' | 'restart') => {
    if (action === 'stop' || action === 'restart') {
      const ok = await confirm(
        action === 'stop'
          ? (isEN ? 'Stop this application process?' : 'Остановить процесс приложения?')
          : (isEN ? 'Restart this application process?' : 'Перезапустить процесс приложения?'),
        { danger: action === 'stop' }
      );
      if (!ok) return;
    }
    setSaving(true);
    setError(null);
    try {
      await ControlAppProcess(siteId, action);
      await loadStatus();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    const ok = await confirm(
      isEN
        ? 'Stop and remove the systemd service for this app?'
        : 'Остановить и удалить systemd-сервис этого приложения?',
      { danger: true }
    );
    if (!ok) return;
    setSaving(true);
    setError(null);
    try {
      await DeleteAppProcess(siteId);
      await loadStatus();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const isRunning = status?.status === 'active';

  return (
    <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <Cpu className="w-5 h-5 text-cyan-400" />
          <div>
            <h4 className="text-sm font-semibold text-slate-100">
              {isEN ? 'App Process Manager (Node.js / Python / Go)' : 'Менеджер процесса (Node.js / Python / Go)'}
            </h4>
            <p className="text-[11px] text-slate-400 font-mono">
              {status?.unit_name || 'tamestack-app.service'}
              {status?.pid ? ` • PID ${status.pid}` : ''}
              {status?.memory ? ` • ${status.memory}` : ''}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={isRunning ? 'success' : status?.configured ? 'warning' : 'neutral'} size="sm">
            {status?.status || 'not_found'}
          </Badge>
          {status?.configured && (
            <>
              <Button variant="ghost" size="sm" onClick={() => handleControl('start')} disabled={saving} title={isEN ? 'Start service' : 'Запустить сервис'} aria-label={isEN ? 'Start service' : 'Запустить сервис'}>
                <Play className="w-3.5 h-3.5 text-emerald-400" />
              </Button>
              <Button variant="ghost" size="sm" onClick={() => handleControl('restart')} disabled={saving} title={isEN ? 'Restart service' : 'Перезапустить сервис'} aria-label={isEN ? 'Restart service' : 'Перезапустить сервис'}>
                <RotateCw className="w-3.5 h-3.5 text-indigo-400" />
              </Button>
              <Button variant="ghost" size="sm" onClick={() => handleControl('stop')} disabled={saving} title={isEN ? 'Stop service' : 'Остановить сервис'} aria-label={isEN ? 'Stop service' : 'Остановить сервис'}>
                <Square className="w-3.5 h-3.5 text-amber-400" />
              </Button>
              <Button variant="ghost" size="sm" onClick={handleDelete} disabled={saving} title={isEN ? 'Delete systemd service' : 'Удалить systemd-сервис'} aria-label={isEN ? 'Delete systemd service' : 'Удалить systemd-сервис'}>
                <Trash2 className="w-3.5 h-3.5 text-rose-400" />
              </Button>
            </>
          )}
          <Button variant="ghost" size="sm" onClick={loadStatus} disabled={loading} title={isEN ? 'Refresh service status' : 'Обновить статус сервиса'} aria-label={isEN ? 'Refresh service status' : 'Обновить статус сервиса'}>
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>
      </div>

      {error && (
        <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          {error}
        </div>
      )}

      <form onSubmit={handleSaveAndStart} className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
        <div>
          <label className="block text-slate-400 mb-1">Runtime</label>
          <Select value={runtime} onChange={(e) => setRuntime(e.target.value)} sizeVariant="sm">
            <option value="nodejs">Node.js</option>
            <option value="python">Python (uvicorn / gunicorn)</option>
            <option value="go">Go Binary</option>
            <option value="custom">Custom</option>
          </Select>
        </div>
        <div>
          <label className="block text-slate-400 mb-1">{isEN ? 'Port (PORT env)' : 'Порт (PORT env)'}</label>
          <Input type="number" value={port} onChange={(e) => setPort(Number(e.target.value))} className="font-mono" />
        </div>
        <div className="md:col-span-2">
          <label className="block text-slate-400 mb-1">{isEN ? 'Start Command (ExecStart)' : 'Команда запуска (ExecStart)'}</label>
          <Input
            value={execStart}
            onChange={(e) => setExecStart(e.target.value)}
            placeholder="/usr/bin/node /var/www/app/server.js"
            className="font-mono"
            required
          />
        </div>
        <div>
          <label className="block text-slate-400 mb-1">{isEN ? 'Working Directory' : 'Рабочий каталог'}</label>
          <Input value={workingDir} onChange={(e) => setWorkingDir(e.target.value)} className="font-mono" required />
        </div>
        <div>
          <label className="block text-slate-400 mb-1">{isEN ? 'System User' : 'Пользователь'}</label>
          <Input value={user} onChange={(e) => setUser(e.target.value)} className="font-mono" />
        </div>
        <div className="md:col-span-2">
          <label className="block text-slate-400 mb-1">{isEN ? 'Environment Variables (.env)' : 'Переменные окружения (.env)'}</label>
          <textarea
            value={envVars}
            onChange={(e) => setEnvVars(e.target.value)}
            rows={3}
            className="w-full rounded-xl bg-slate-950 border border-slate-800 px-3 py-2 text-xs font-mono text-slate-200 focus:outline-none focus:border-indigo-500"
          />
        </div>
        <div className="md:col-span-2 flex justify-end">
          <Button type="submit" variant="primary" size="sm" loading={saving} disabled={saving}>
            <Save className="w-3.5 h-3.5 mr-1.5" />
            {isEN ? 'Deploy & Restart Service' : 'Применить и перезапустить'}
          </Button>
        </div>
      </form>

      {status?.recent_logs && (
        <div className="space-y-1.5">
          <div className="flex items-center gap-1.5 text-[11px] text-slate-400">
            <Terminal className="w-3.5 h-3.5" />
            <span>journalctl (recent)</span>
          </div>
          <pre className="p-3 rounded-xl bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 max-h-40 overflow-y-auto whitespace-pre-wrap">
            {status.recent_logs}
          </pre>
        </div>
      )}
    </div>
  );
};
