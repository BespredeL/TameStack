import React, { useState } from 'react';
import { php } from '../../../wailsjs/go/models';
import { UpdatePHPPool } from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { Button, Input, Select } from '../ui';
import { getErrorMessage } from '../../utils/errors';
import { useTranslation } from '../../i18n/LanguageContext';
import { Settings, AlertCircle, Shield, Cpu, Sliders } from 'lucide-react';

interface EditPoolModalProps {
  serverId: string;
  version: string;
  pool: php.FpmPoolConfig;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const EditPoolModal: React.FC<EditPoolModalProps> = ({
  serverId,
  version,
  pool,
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [user, setUser] = useState(pool.user || 'www-data');
  const [group, setGroup] = useState(pool.group || 'www-data');
  const [listen, setListen] = useState(pool.listen || '');
  const [listenOwner, setListenOwner] = useState(pool.listen_owner || pool.user || 'www-data');
  const [listenGroup, setListenGroup] = useState(pool.listen_group || pool.group || 'www-data');
  const [listenMode, setListenMode] = useState(pool.listen_mode || '0660');
  const [pmMode, setPmMode] = useState(pool.pm_mode || 'dynamic');
  const [maxChildren, setMaxChildren] = useState(pool.max_children || 10);
  const [startServers, setStartServers] = useState(pool.start_servers || 2);
  const [minSpare, setMinSpare] = useState(pool.min_spare_servers || 1);
  const [maxSpare, setMaxSpare] = useState(pool.max_spare_servers || 3);
  const [maxRequests, setMaxRequests] = useState(pool.pm_max_requests || 0);
  const [memoryLimit, setMemoryLimit] = useState(pool.memory_limit || '256M');
  const [maxExecTime, setMaxExecTime] = useState(pool.max_execution_time || 60);
  const [uploadMax, setUploadMax] = useState(pool.upload_max_filesize || '32M');
  const [postMax, setPostMax] = useState(pool.post_max_size || '32M');
  const [slowlog, setSlowlog] = useState(pool.slowlog || false);
  const [slowlogTimeout, setSlowlogTimeout] = useState(pool.slowlog_timeout || '5s');

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await UpdatePHPPool({
        server_id: serverId,
        version: version,
        pool_name: pool.pool_name,
        is_enabled: pool.is_enabled !== false,
        user: user.trim(),
        group: group.trim(),
        listen_socket: listen.trim(),
        listen_owner: listenOwner.trim(),
        listen_group: listenGroup.trim(),
        listen_mode: listenMode.trim(),
        pm_mode: pmMode,
        max_children: Number(maxChildren) || 5,
        start_servers: Number(startServers) || 2,
        min_spare_servers: Number(minSpare) || 1,
        max_spare_servers: Number(maxSpare) || 3,
        pm_max_requests: Number(maxRequests) || 0,
        process_idle_timeout: '10s',
        memory_limit: memoryLimit.trim(),
        max_execution_time: Number(maxExecTime) || 60,
        upload_max_filesize: uploadMax.trim(),
        post_max_size: postMax.trim(),
        slowlog: slowlog,
        slowlog_timeout: slowlogTimeout.trim(),
        access_log: '',
        php_values: {},
        php_admin_values: {},
      } as any);
      onSuccess();
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      title={`${t('php.editPoolTitle')}: ${pool.pool_name}`}
      subtitle={`PHP ${version}-FPM`}
      icon={<Settings className="w-5 h-5 text-indigo-400" />}
      size="2xl"
    >
      <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
        <div className="p-6 space-y-6 max-h-[70vh] overflow-y-auto flex-1">
          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/20 rounded-xl flex items-start gap-2.5 text-red-300 text-xs">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
              <span className="font-mono">{error}</span>
            </div>
          )}

          {/* SECTION 1: USER & LISTEN SOCKET */}
          <div className="space-y-4 bg-slate-950/60 p-4 rounded-xl border border-slate-800">
            <div className="flex items-center gap-2 pb-2 border-b border-slate-800/80 text-xs font-bold text-indigo-400 uppercase tracking-wider">
              <Shield className="w-4 h-4 text-indigo-400" />
              <span>{t('php.poolUserSection')}</span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.poolUser')}</label>
                <Input
                  type="text"
                  value={user}
                  onChange={(e) => setUser(e.target.value)}
                  placeholder="www-data"
                  className="font-mono"
                  required
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.poolGroup')}</label>
                <Input
                  type="text"
                  value={group}
                  onChange={(e) => setGroup(e.target.value)}
                  placeholder="www-data"
                  className="font-mono"
                  required
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.poolListen')}</label>
              <Input
                type="text"
                value={listen}
                onChange={(e) => setListen(e.target.value)}
                placeholder="/run/php/php8.1-fpm.sock or 127.0.0.1:9000"
                className="font-mono"
                required
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.listenOwner')}</label>
                <Input
                  type="text"
                  value={listenOwner}
                  onChange={(e) => setListenOwner(e.target.value)}
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.listenGroup')}</label>
                <Input
                  type="text"
                  value={listenGroup}
                  onChange={(e) => setListenGroup(e.target.value)}
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.listenMode')}</label>
                <Input
                  type="text"
                  value={listenMode}
                  onChange={(e) => setListenMode(e.target.value)}
                  placeholder="0660"
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
            </div>
          </div>

          {/* SECTION 2: PROCESS MANAGER (PM) */}
          <div className="space-y-4 bg-slate-950/60 p-4 rounded-xl border border-slate-800">
            <div className="flex items-center gap-2 pb-2 border-b border-slate-800/80 text-xs font-bold text-indigo-400 uppercase tracking-wider">
              <Cpu className="w-4 h-4 text-indigo-400" />
              <span>{t('php.pmSettings')}</span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.pmMode')}</label>
                <Select
                  value={pmMode}
                  onChange={(e) => setPmMode(e.target.value)}
                >
                  <option value="dynamic">{t('php.pmDynamic')}</option>
                  <option value="ondemand">{t('php.pmOndemand')}</option>
                  <option value="static">{t('php.pmStatic')}</option>
                </Select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.maxChildren')}</label>
                <Input
                  type="number"
                  min={1}
                  max={512}
                  value={maxChildren}
                  onChange={(e) => setMaxChildren(parseInt(e.target.value) || 5)}
                  className="font-mono"
                />
              </div>
            </div>

            {pmMode === 'dynamic' && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.startServers')}</label>
                  <Input
                    type="number"
                    min={1}
                    value={startServers}
                    onChange={(e) => setStartServers(parseInt(e.target.value) || 2)}
                    className="font-mono"
                    sizeVariant="sm"
                  />
                </div>
                <div>
                  <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.minSpare')}</label>
                  <Input
                    type="number"
                    min={1}
                    value={minSpare}
                    onChange={(e) => setMinSpare(parseInt(e.target.value) || 1)}
                    className="font-mono"
                    sizeVariant="sm"
                  />
                </div>
                <div>
                  <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.maxSpare')}</label>
                  <Input
                    type="number"
                    min={1}
                    value={maxSpare}
                    onChange={(e) => setMaxSpare(parseInt(e.target.value) || 3)}
                    className="font-mono"
                    sizeVariant="sm"
                  />
                </div>
              </div>
            )}

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.maxRequests')}</label>
              <Input
                type="number"
                min={0}
                value={maxRequests}
                onChange={(e) => setMaxRequests(parseInt(e.target.value) || 0)}
                placeholder={t('php.maxRequestsPlaceholder')}
                className="font-mono"
              />
              <p className="text-[11px] text-slate-500 mt-1">
                {t('php.maxRequestsHelp')}
              </p>
            </div>
          </div>

          {/* SECTION 3: RESOURCE LIMITS */}
          <div className="space-y-4 bg-slate-950/60 p-4 rounded-xl border border-slate-800">
            <div className="flex items-center gap-2 pb-2 border-b border-slate-800/80 text-xs font-bold text-indigo-400 uppercase tracking-wider">
              <Sliders className="w-4 h-4 text-indigo-400" />
              <span>{t('php.poolLimits')}</span>
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.memoryLimit')}</label>
                <Input
                  type="text"
                  value={memoryLimit}
                  onChange={(e) => setMemoryLimit(e.target.value)}
                  placeholder="256M"
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.maxExecTime')}</label>
                <Input
                  type="number"
                  min={5}
                  max={3600}
                  value={maxExecTime}
                  onChange={(e) => setMaxExecTime(parseInt(e.target.value) || 60)}
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.uploadMax')}</label>
                <Input
                  type="text"
                  value={uploadMax}
                  onChange={(e) => setUploadMax(e.target.value)}
                  placeholder="32M"
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
              <div>
                <label className="block text-slate-400 mb-1 text-[11px] font-medium">{t('php.postMax')}</label>
                <Input
                  type="text"
                  value={postMax}
                  onChange={(e) => setPostMax(e.target.value)}
                  placeholder="32M"
                  className="font-mono"
                  sizeVariant="sm"
                />
              </div>
            </div>
          </div>

          {/* SECTION 4: SLOWLOG */}
          <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800 flex flex-wrap items-center justify-between gap-3">
            <label className="flex items-center gap-3 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={slowlog}
                onChange={(e) => setSlowlog(e.target.checked)}
                className="w-4 h-4 rounded border-slate-700 bg-slate-900 text-indigo-500 focus:ring-0 cursor-pointer"
              />
              <div>
                <span className="text-white text-xs font-semibold block">{t('php.enableSlowlog')}</span>
                <span className="text-[11px] text-slate-500">{t('php.slowlogDesc')}</span>
              </div>
            </label>
            {slowlog && (
              <div className="flex items-center gap-2">
                <span className="text-slate-400 text-xs font-mono">{t('php.slowlogTimeout')}:</span>
                <div className="w-24">
                  <Input
                    type="text"
                    value={slowlogTimeout}
                    onChange={(e) => setSlowlogTimeout(e.target.value)}
                    placeholder="5s"
                    className="font-mono"
                    sizeVariant="sm"
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        {/* PINNED MODAL FOOTER */}
        <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-end gap-3 shrink-0">
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={saving}
            loading={saving}
          >
            {saving ? t('common.saving') : t('common.save')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
