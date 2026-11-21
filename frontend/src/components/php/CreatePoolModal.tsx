import React, { useState } from 'react';
import { CreatePHPPool } from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { Button, Input, Select } from '../ui';
import { getErrorMessage } from '../../utils/errors';
import { useTranslation } from '../../i18n/LanguageContext';
import { Layers, AlertCircle, CheckCircle2 } from 'lucide-react';

interface CreatePoolModalProps {
  serverId: string;
  version: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (poolName: string) => void;
}

export const CreatePoolModal: React.FC<CreatePoolModalProps> = ({
  serverId,
  version,
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [poolName, setPoolName] = useState('');
  const [user, setUser] = useState('www-data');
  const [group, setGroup] = useState('www-data');
  const [pmMode, setPmMode] = useState('dynamic');
  const [maxChildren, setMaxChildren] = useState(10);
  const [memoryLimit, setMemoryLimit] = useState('256M');
  const [maxExecTime, setMaxExecTime] = useState(60);
  const [uploadMax, setUploadMax] = useState('32M');
  const [postMax, setPostMax] = useState('32M');
  const [slowlog, setSlowlog] = useState(false);

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!poolName.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await CreatePHPPool({
        server_id: serverId,
        version: version,
        pool_name: poolName.trim().toLowerCase(),
        user: user.trim(),
        group: group.trim(),
        pm_mode: pmMode,
        max_children: Number(maxChildren) || 5,
        start_servers: 2,
        min_spare_servers: 1,
        max_spare_servers: 3,
        pm_max_requests: 0,
        process_idle_timeout: '10s',
        memory_limit: memoryLimit.trim(),
        max_execution_time: Number(maxExecTime) || 60,
        upload_max_filesize: uploadMax.trim(),
        post_max_size: postMax.trim(),
        slowlog: slowlog,
        slowlog_timeout: '5s',
      } as any);
      onSuccess(poolName.trim());
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
      title={t('php.createPoolTitle', { version })}
      subtitle={t('php.createPoolSubtitle')}
      icon={<Layers className="w-5 h-5 text-indigo-400" />}
      size="lg"
    >
      <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
        <div className="p-6 space-y-4 max-h-[70vh] overflow-y-auto flex-1">
          {error && (
            <div className="p-3 bg-rose-500/10 border border-rose-500/30 rounded-xl text-xs text-rose-300 flex items-start gap-2">
              <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <pre className="font-mono whitespace-pre-wrap">{error}</pre>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('php.poolNameLabel', { version })}
            </label>
            <Input
              type="text"
              required
              pattern="^[a-zA-Z0-9_\-]+$"
              placeholder={t('php.poolNamePlaceholder')}
              value={poolName}
              onChange={(e) => setPoolName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
              className="font-mono"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.userLabel')}</label>
              <Input
                type="text"
                value={user}
                onChange={(e) => setUser(e.target.value)}
                className="font-mono"
              />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.groupLabel')}</label>
              <Input
                type="text"
                value={group}
                onChange={(e) => setGroup(e.target.value)}
                className="font-mono"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.pmLabel')}</label>
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
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.maxChildrenLabel')}</label>
              <Input
                type="number"
                min={1}
                max={256}
                value={maxChildren}
                onChange={(e) => setMaxChildren(Number(e.target.value) || 5)}
                className="font-mono"
              />
            </div>
          </div>

          <div className="pt-2 border-t border-slate-800 space-y-3">
            <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider block">
              {t('php.resourceLimitsTitle')}
            </span>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.memoryLimitLabel')}</label>
                <Input
                  type="text"
                  placeholder="256M"
                  value={memoryLimit}
                  onChange={(e) => setMemoryLimit(e.target.value)}
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.maxExecTimeLabel')}</label>
                <Input
                  type="number"
                  min={5}
                  max={600}
                  value={maxExecTime}
                  onChange={(e) => setMaxExecTime(Number(e.target.value) || 60)}
                  className="font-mono"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.uploadMaxLabel')}</label>
                <Input
                  type="text"
                  placeholder="32M"
                  value={uploadMax}
                  onChange={(e) => setUploadMax(e.target.value)}
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">{t('php.postMaxLabel')}</label>
                <Input
                  type="text"
                  placeholder="32M"
                  value={postMax}
                  onChange={(e) => setPostMax(e.target.value)}
                  className="font-mono"
                />
              </div>
            </div>

            <label className="flex items-center gap-2.5 pt-1 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={slowlog}
                onChange={(e) => setSlowlog(e.target.checked)}
                className="w-4 h-4 rounded border-slate-700 bg-slate-900 text-indigo-500 focus:ring-0 cursor-pointer"
              />
              <span className="text-xs text-slate-300">
                {t('php.slowlogLabel')}
              </span>
            </label>
          </div>
        </div>

        {/* Modal Footer */}
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
            disabled={saving || !poolName.trim()}
            loading={saving}
            icon={!saving ? <CheckCircle2 className="w-4 h-4" /> : undefined}
          >
            {saving ? t('php.creatingPool') : t('php.createAndActivatePool')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
