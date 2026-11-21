import React, { useState, useEffect } from 'react';
import { db, site, php } from '../../wailsjs/go/models';
import {
  InspectPHPVersions,
  ListPHPPools,
  PlanSiteRuntimeAssignment,
  AssignSiteRuntime,
} from '../../wailsjs/go/main/App';
import { getErrorMessage } from '../utils/errors';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { Button, Input, Select } from './ui';
import {
  Code2,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  ShieldCheck,
  Zap,
} from 'lucide-react';

interface SiteRuntimeTabProps {
  site: db.SiteRecord;
  serverID: string;
  onSiteUpdated: () => void;
}

export const SiteRuntimeTab: React.FC<SiteRuntimeTabProps> = ({
  site,
  serverID,
  onSiteUpdated,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();

  const [versions, setVersions] = useState<php.PHPVersionInfo[]>([]);
  const [pools, setPools] = useState<php.FpmPoolConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingPools, setLoadingPools] = useState(false);

  // Form
  const [selectedVersion, setSelectedVersion] = useState(site.php_version || '8.3');
  const [selectedHandler, setSelectedHandler] = useState('fpm');
  const [selectedPool, setSelectedPool] = useState('www');
  const [createDedicatedPool, setCreateDedicatedPool] = useState(false);
  const [dedicatedUser, setDedicatedUser] = useState((site as any).system_user || 'www-data');
  const [dedicatedMemory, setDedicatedMemory] = useState('256M');

  // Plan & Execution
  const [planning, setPlanning] = useState(false);
  const [plan, setPlan] = useState<site.SiteRuntimePlan | null>(null);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Load versions
  useEffect(() => {
    if (!serverID) return;
    setLoading(true);
    InspectPHPVersions(serverID)
      .then((data) => {
        const installed = (data || []).filter((v) => v.is_installed);
        setVersions(installed);
        if (installed.length > 0 && !installed.some((v) => v.version === selectedVersion)) {
          setSelectedVersion(installed[0].version);
        }
      })
      .catch((err) => setError(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, [serverID]);

  // Load pools for selected version
  useEffect(() => {
    if (!serverID || !selectedVersion) return;
    setLoadingPools(true);
    ListPHPPools(serverID, selectedVersion)
      .then((data) => {
        setPools(data || []);
        if (data && data.length > 0) {
          const matching = data.find((p) => p.pool_name === site.domain?.replace(/[^a-z0-9]/gi, '_'));
          if (matching) {
            setSelectedPool(matching.pool_name);
          } else {
            setSelectedPool(data[0].pool_name);
          }
        }
      })
      .catch(() => setPools([]))
      .finally(() => setLoadingPools(false));
  }, [serverID, selectedVersion]);

  const handlePlan = async () => {
    setPlanning(true);
    setError(null);
    setPlan(null);
    try {
      const p = await PlanSiteRuntimeAssignment({
        site_id: site.id,
        php_version: selectedVersion,
        handler: selectedHandler,
        pool_name: createDedicatedPool ? site.domain.replace(/[^a-z0-9]/gi, '_') : selectedPool,
        create_pool: createDedicatedPool,
        pool_user: dedicatedUser.trim() || 'www-data',
        pool_memory_limit: dedicatedMemory.trim() || '256M',
      } as any);
      setPlan(p);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setPlanning(false);
    }
  };

  const handleApply = async () => {
    if (!(await confirm(t('siteRuntime.confirmApply')))) {
      return;
    }
    setApplying(true);
    setError(null);
    setSuccess(null);
    try {
      await AssignSiteRuntime({
        site_id: site.id,
        php_version: selectedVersion,
        handler: selectedHandler,
        pool_name: createDedicatedPool ? site.domain.replace(/[^a-z0-9]/gi, '_') : selectedPool,
        create_pool: createDedicatedPool,
        pool_user: dedicatedUser.trim() || 'www-data',
        pool_memory_limit: dedicatedMemory.trim() || '256M',
      } as any);
      setSuccess(t('siteRuntime.applySuccess'));
      setPlan(null);
      onSiteUpdated();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setApplying(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Current Runtime Banner */}
      <div className="bg-slate-900/60 p-4 rounded-xl border border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
            <Code2 className="w-5 h-5" />
          </div>
          <div>
            <span className="text-[11px] font-medium text-slate-400 block uppercase tracking-wider">
              {t('siteRuntime.currentRuntime')}
            </span>
            <div className="flex items-center gap-2 mt-0.5">
              <span className="text-white font-bold text-sm font-mono">
                PHP {site.php_version || 'None / Static'}
              </span>
              <span className="text-slate-500">•</span>
              <span className="text-slate-300 text-xs font-mono uppercase">
                {site.web_server || 'nginx'}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2 text-xs text-slate-400 font-mono">
          <span className="text-emerald-400 flex items-center gap-1 font-sans font-medium">
            <ShieldCheck className="w-4 h-4" /> ConfigGuardian Active
          </span>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {success && (
        <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center gap-3">
          <CheckCircle2 className="w-5 h-5 shrink-0" />
          <span>{success}</span>
        </div>
      )}

      {/* Runtime Configuration Form */}
      <div className="bg-surface-850 p-6 rounded-2xl border border-slate-800 space-y-6">
        <h4 className="text-sm font-bold text-white flex items-center gap-2">
          <span>{t('siteRuntime.configureTitle')}</span>
        </h4>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* PHP Version */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('siteRuntime.phpVersion')}
            </label>
            {loading ? (
              <div className="text-xs text-slate-500 flex items-center gap-2 py-2">
                <Loader2 className="w-3.5 h-3.5 animate-spin" /> {t('common.loading.defaultTitle')}
              </div>
            ) : (
              <Select
                value={selectedVersion}
                onChange={(e) => setSelectedVersion(e.target.value)}
                className="font-mono"
              >
                {versions.map((v) => (
                  <option key={v.version} value={v.version}>
                    PHP {v.version} ({v.fpm_status === 'active' ? 'FPM Active' : 'Stopped'})
                  </option>
                ))}
              </Select>
            )}
            <p className="text-[11px] text-slate-500 mt-1">
              {t('siteRuntime.versionNotice')}
            </p>
          </div>

          {/* Handler */}
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('siteRuntime.handler')}
            </label>
            <Select
              value={selectedHandler}
              onChange={(e) => setSelectedHandler(e.target.value)}
            >
              <option value="fpm">PHP-FPM (FastCGI Unix Domain Socket) — Recommended</option>
              <option value="proxy_fcgi">Apache mod_proxy_fcgi + FPM</option>
              <option value="mod_php">Apache mod_php (Legacy prefork)</option>
            </Select>
            <p className="text-[11px] text-slate-500 mt-1">
              {t('siteRuntime.handlerNotice')}
            </p>
          </div>
        </div>

        {/* Pool Selection */}
        {selectedHandler !== 'mod_php' && (
          <div className="pt-4 border-t border-slate-800 space-y-4">
            <div className="flex items-center justify-between">
              <label className="block text-xs font-semibold text-slate-300">
                {t('siteRuntime.poolConfig')}
              </label>

              <label className="flex items-center gap-2 cursor-pointer text-xs text-indigo-400 select-none">
                <input
                  type="checkbox"
                  checked={createDedicatedPool}
                  onChange={(e) => setCreateDedicatedPool(e.target.checked)}
                  className="w-4 h-4 rounded border-slate-700 bg-slate-900 text-indigo-500 focus:ring-0 cursor-pointer"
                />
                <span>{t('siteRuntime.createDedicatedPool')}</span>
              </label>
            </div>

            {createDedicatedPool ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-4 rounded-xl bg-slate-950/60 border border-slate-800">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                    {t('siteRuntime.poolUser')}
                  </label>
                  <Input
                    type="text"
                    value={dedicatedUser}
                    onChange={(e) => setDedicatedUser(e.target.value)}
                    placeholder="www-data"
                    className="font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                    {t('siteRuntime.poolMemory')}
                  </label>
                  <Input
                    type="text"
                    value={dedicatedMemory}
                    onChange={(e) => setDedicatedMemory(e.target.value)}
                    placeholder="256M"
                    className="font-mono"
                  />
                </div>
              </div>
            ) : (
              <div>
                <Select
                  value={selectedPool}
                  onChange={(e) => setSelectedPool(e.target.value)}
                  disabled={loadingPools}
                  className="font-mono"
                >
                  {pools.map((p) => (
                    <option key={p.pool_name} value={p.pool_name}>
                      Pool: {p.pool_name} ({p.listen || 'socket'}) [user: {p.user}]
                    </option>
                  ))}
                </Select>
              </div>
            )}
          </div>
        )}

        {/* Pre-flight Plan Display */}
        {plan && (
          <div className="p-4 rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-xs space-y-3">
            <h5 className="font-bold text-indigo-300 flex items-center gap-2">
              <Zap className="w-4 h-4" />
              <span>{t('siteRuntime.planReady')}</span>
            </h5>
            <div className="space-y-1 font-mono text-[11px] text-slate-300">
              <div>• Target Socket: <span className="text-white">{plan.target_socket}</span></div>
              <div>• Target Handler: <span className="text-cyan-300">{plan.handler} (PHP {plan.target_php_version})</span></div>
              <div>• Pool: <span className="text-cyan-300">{plan.pool_name}</span></div>
              <div>• VirtualHost File: <span className="text-emerald-300">{plan.vhost_file}</span></div>
            </div>
            <p className="text-[11px] text-slate-400">
              {t('siteRuntime.guardianNotice')}
            </p>
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
          <Button
            type="button"
            variant="secondary"
            size="md"
            onClick={handlePlan}
            disabled={planning || applying}
            loading={planning}
          >
            {planning ? '...' : t('siteRuntime.previewPlanBtn')}
          </Button>

          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={handleApply}
            disabled={applying}
            loading={applying}
          >
            {applying ? '...' : t('siteRuntime.applyBtn')}
          </Button>
        </div>
      </div>
    </div>
  );
};
