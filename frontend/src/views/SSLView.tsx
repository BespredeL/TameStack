import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { useServerRequest } from '../hooks/useServerRequest';
import {
  ShieldCheck,
  RefreshCw,
  Plus,
  Lock,
  AlertTriangle,
  RotateCw,
  Trash2,
  CheckCircle2,
  Sparkles,
  Download,
  Settings,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import {
  ListSSLCertificates,
  GetSSLMetrics,
  RenewSSLCertificate,
  DeleteSSLCertificate,
  DiscoverSSLCertificates,
  GetCertbotStatus,
} from '../../wailsjs/go/main/App';
import { db, ssl } from '../../wailsjs/go/models';
import { PageToolbar, Button, Badge } from '../components/ui';
import { IssueSSLModal } from '../components/IssueSSLModal';
import { InstallCertbotModal } from '../components/InstallCertbotModal';
import { CertbotSettingsModal } from '../components/CertbotSettingsModal';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import { usePrivacy } from '../context/PrivacyContext';

interface SSLViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const SSLView: React.FC<SSLViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
    setPageLoading,
  } = useServer();
  const { t } = useTranslation();
  const confirm = useConfirm();
  const { maskDomain, maskServerName, maskText } = usePrivacy();
  const [certs, setCerts] = useState<db.SSLCertificateRecord[]>([]);
  const [metrics, setMetrics] = useState<ssl.SSLMetrics | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isDiscovering, setIsDiscovering] = useState(false);
  const [discoverMessage, setDiscoverMessage] = useState<string | null>(null);
  const [isIssueOpen, setIsIssueOpen] = useState(false);
  const [renewingID, setRenewingID] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [certbotStatus, setCertbotStatus] = useState<ssl.CertbotStatus | null>(null);
  const [isInstallCertbotOpen, setIsInstallCertbotOpen] = useState(false);
  const [isCertbotSettingsOpen, setIsCertbotSettingsOpen] = useState(false);
  const sslReq = useServerRequest();

  const selectedServer = servers.find((s) => s.id === selectedServerID) || servers[0];

  const fetchSSLData = async () => {
    if (!selectedServer) return;
    const serverID = selectedServer.id;
    const gen = sslReq.next();
    setCerts([]);
    setMetrics(null);
    setCertbotStatus(null);
    setIsLoading(true);
    setPageLoading(true, t('ssl.loadingServer', { server: selectedServer.name }));
    try {
      const certsData = await ListSSLCertificates(serverID);
      if (!sslReq.isCurrent(gen)) return;
      const metricsData = await GetSSLMetrics(serverID);
      if (!sslReq.isCurrent(gen)) return;
      setCerts(certsData || []);
      setMetrics(metricsData);
      try {
        const certbotData = await GetCertbotStatus(serverID);
        if (sslReq.isCurrent(gen)) setCertbotStatus(certbotData);
      } catch {
        if (sslReq.isCurrent(gen)) setCertbotStatus(null);
      }
    } catch (err: unknown) {
      if (!sslReq.isCurrent(gen)) return;
      setActionError(getErrorMessage(err));
    } finally {
      if (sslReq.isCurrent(gen)) {
        setIsLoading(false);
        setPageLoading(false);
      }
    }
  };

  const handleDiscoverCertificates = async () => {
    if (!selectedServer) return;
    setIsDiscovering(true);
    setPageLoading(true, t('ssl.scanningServer', { server: selectedServer.name }));
    setDiscoverMessage(null);
    try {
      const discovered = await DiscoverSSLCertificates(selectedServer.id);
      setCerts(discovered || []);
      const count = discovered?.length || 0;
      setDiscoverMessage(t('ssl.syncComplete', { count }));
      setTimeout(() => setDiscoverMessage(null), 5000);
      const metricsData = await GetSSLMetrics(selectedServer.id);
      setMetrics(metricsData);
    } catch (err: unknown) {
      console.error('Discover SSL error:', err);
      setDiscoverMessage(t('ssl.discoverError', { error: getErrorMessage(err) }));
    } finally {
      setIsDiscovering(false);
      setPageLoading(false);
    }
  };

  useEffect(() => {
    if (selectedServerID) {
      fetchSSLData();
    }
  }, [selectedServerID]);

  useEffect(() => {
    const cleanup = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        data.server_id === selectedServerID &&
        data.status === 'completed' &&
        (data.operation_type === 'ssl_install_certbot' ||
          data.operation_type === 'ssl_issue' ||
          data.operation_type === 'ssl_renew')
      ) {
        fetchSSLData();
      }
    });
    return () => {
      if (typeof cleanup === 'function') cleanup();
    };
  }, [selectedServerID, fetchSSLData]);

  const handleRenew = async (certID: string) => {
    setRenewingID(certID);
    try {
      const task = await RenewSSLCertificate(certID);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    } finally {
      setRenewingID(null);
    }
  };

  const handleDelete = async (certID: string, domain: string) => {
    if (!(await confirm(t('ssl.deleteConfirm', { domain }), { danger: true, confirmLabel: t('common.delete') }))) {
      return;
    }
    try {
      await DeleteSSLCertificate(certID);
      await fetchSSLData();
    } catch (err: unknown) {
      setActionError(getErrorMessage(err));
    }
  };

  const formatIssuer = (issuer: string) => {
    if (!issuer) return '—';
    const lower = issuer.toLowerCase();
    if (lower.includes("let's encrypt") || lower.includes("letsencrypt")) return "Let's Encrypt";
    if (lower.includes('zerossl')) return 'ZeroSSL';
    if (lower.includes('google trust')) return 'Google Trust Services';
    if (lower.includes('sectigo') || lower.includes('comodo')) return 'Sectigo';
    if (lower.includes('cloudflare')) return 'Cloudflare';
    if (lower.includes('digicert')) return 'DigiCert';
    return issuer;
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={ShieldCheck}
        description={t('empty.sslDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {actionError && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
          {actionError}
        </div>
      )}
      {/* Top Toolbar */}
      <PageToolbar
        icon={<ShieldCheck className="w-5 h-5 text-emerald-400" />}
        iconClassName="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center justify-center shrink-0"
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('ssl.targetServer')}
        actions={
          <>
            <Button
              variant="secondary"
              size="md"
              onClick={handleDiscoverCertificates}
              disabled={isDiscovering}
              icon={<Sparkles className={`w-3.5 h-3.5 text-amber-400 ${isDiscovering ? 'animate-spin' : ''}`} />}
              title={t('ssl.importCertificates')}
            >
              {isDiscovering ? t('ssl.searching') : t('ssl.importCertificates')}
            </Button>
            <Button
              variant="secondary"
              size="icon-md"
              onClick={fetchSSLData}
              disabled={isLoading}
              title={t('ssl.refreshCertificates')}
              aria-label={t('ssl.refreshCertificates')}
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin text-emerald-400' : ''}`} />
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => setIsIssueOpen(true)}
              icon={<Plus className="w-4 h-4" />}
            >
              {t('ssl.issueCertificate')}
            </Button>
          </>
        }
      />

      {certbotStatus && !certbotStatus.installed && (
        <div className="p-4 bg-amber-950/40 border border-amber-500/30 rounded-xl flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0">
              <AlertTriangle className="w-5 h-5" />
            </div>
            <div>
              <div className="text-sm font-semibold text-amber-200">{t('ssl.certbot.notInstalledTitle')}</div>
              <div className="text-xs text-slate-400 mt-0.5">{t('ssl.certbot.notInstalledDesc')}</div>
            </div>
          </div>
          <Button
            variant="success"
            size="md"
            onClick={() => setIsInstallCertbotOpen(true)}
            icon={<Download className="w-4 h-4" />}
            className="whitespace-nowrap"
          >
            {t('ssl.certbot.installBtn')}
          </Button>
        </div>
      )}

      {certbotStatus && certbotStatus.installed && (
        <div className="p-3 bg-surface-850 border border-slate-800 rounded-xl flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex items-center gap-2 text-emerald-400 font-semibold">
              <ShieldCheck className="w-4 h-4" />
              <span>{t('ssl.certbot.installedTitle')}</span>
            </div>
            <span className="text-slate-400 font-mono">{certbotStatus.version}</span>
            <Badge variant={certbotStatus.renew_timer_active ? 'success' : 'warning'}>
              {certbotStatus.renew_timer_active ? t('ssl.certbot.renewActive') : t('ssl.certbot.renewInactive')}
            </Badge>
          </div>

          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsCertbotSettingsOpen(true)}
            icon={<Settings className="w-3.5 h-3.5 text-indigo-400" />}
          >
            {t('ssl.certbot.settingsBtn')}
          </Button>
        </div>
      )}

      {isDiscovering && (
        <div className="p-4 bg-emerald-950/70 border border-emerald-500/40 rounded-xl text-xs text-emerald-200 flex items-center gap-3 shadow-md animate-pulse">
          <RefreshCw className="w-5 h-5 text-emerald-400 animate-spin shrink-0" />
          <div>
            <div className="font-bold text-white">{t('ssl.scanningBannerTitle')}</div>
            <div className="text-slate-400 font-mono text-[11px] mt-0.5">
              {t('ssl.scanningBannerDesc')}
            </div>
          </div>
        </div>
      )}

      {discoverMessage && !isDiscovering && (
        <div className="p-3 bg-emerald-950/60 border border-emerald-500/30 rounded-xl text-xs text-emerald-200 flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-amber-400 shrink-0" />
          <span>{discoverMessage}</span>
        </div>
      )}

      {/* Overview Metrics Cards */}
      <div className="stat-grid">
        <div className="bg-surface-850 border border-slate-800 p-4 rounded-xl shadow-lg">
          <span className="text-xs font-medium text-slate-400">{t('ssl.totalCertificates')}</span>
          <div className="mt-2 text-2xl font-bold text-white font-mono">{metrics?.total_certificates || 0}</div>
        </div>

        <div className="bg-surface-850 border border-slate-800 p-4 rounded-xl shadow-lg">
          <span className="text-xs font-medium text-emerald-400">{t('ssl.valid')}</span>
          <div className="mt-2 text-2xl font-bold text-emerald-400 font-mono">{metrics?.valid_count || 0}</div>
        </div>

        <div className="bg-surface-850 border border-slate-800 p-4 rounded-xl shadow-lg">
          <span className="text-xs font-medium text-amber-400">{t('ssl.expiringSoon')}</span>
          <div className="mt-2 text-2xl font-bold text-amber-400 font-mono">{metrics?.expiring_soon_count || 0}</div>
        </div>

        <div className="bg-surface-850 border border-slate-800 p-4 rounded-xl shadow-lg">
          <span className="text-xs font-medium text-rose-400">{t('ssl.expired')}</span>
          <div className="mt-2 text-2xl font-bold text-rose-400 font-mono">{metrics?.expired_count || 0}</div>
        </div>
      </div>

      {/* Certificates Table */}
      {isLoading ? (
        <LoadingState
          type="table"
          title={t('ssl.loadingTitle')}
          subtitle={t('ssl.loadingSubtitle', { server: maskServerName(selectedServer?.name || '') })}
        />
      ) : (
        <div className="bg-surface-850 border border-slate-800 rounded-2xl shadow-lg overflow-hidden">
          {certs.length === 0 ? (
          <div className="p-12 text-center text-slate-500 text-xs space-y-4">
            <Lock className="w-8 h-8 text-slate-600 mx-auto" />
            <div>
              <p className="text-white font-medium">{t('ssl.emptyTitle')}</p>
              <p className="text-slate-400 mt-1 max-w-md mx-auto">
                {t('ssl.emptyDesc')}
              </p>
            </div>
            <div className="flex items-center justify-center gap-3 pt-2">
              <button
                onClick={handleDiscoverCertificates}
                disabled={isDiscovering}
                className="flex items-center gap-1.5 px-4 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-200 rounded-lg text-xs font-semibold transition"
              >
                <Sparkles className="w-3.5 h-3.5 text-amber-400" />
                <span>{t('ssl.importFromServer')}</span>
              </button>
              <button
                onClick={() => setIsIssueOpen(true)}
                className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold shadow-md shadow-emerald-600/30"
              >
                {t('ssl.issueCertificate')}
              </button>
            </div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-900/60 text-slate-400 border-b border-slate-800 font-semibold">
                <tr>
                  <th className="py-3 px-4">{t('ssl.tableDomain')}</th>
                  <th className="py-3 px-4">{t('ssl.tableIssuer')}</th>
                  <th className="py-3 px-4">{t('ssl.tableDaysLeft')}</th>
                  <th className="py-3 px-4">{t('ssl.tableExpires')}</th>
                  <th className="py-3 px-4">{t('ssl.tableAutoRenew')}</th>
                  <th className="py-3 px-4 text-right">{t('ssl.tableActions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono">
                {certs.map((c) => {
                  const percent = Math.min(100, Math.max(0, Math.round((c.days_remaining / 90) * 100)));
                  const isExpiring = c.days_remaining <= 30;

                  return (
                    <tr key={c.id} className="hover:bg-slate-800/40 transition-colors">
                      <td className="py-3.5 px-4 font-sans">
                        <div className="font-bold text-white text-sm font-mono">{maskDomain(c.primary_domain)}</div>
                        {c.san_domains && c.san_domains.length > 0 && (
                          <div className="text-[11px] text-slate-400 font-mono mt-0.5">
                            SANs: {c.san_domains.map(maskDomain).join(', ')}
                          </div>
                        )}
                      </td>

                      <td className="py-3.5 px-4 max-w-[220px] font-sans">
                        <span
                          className="inline-block max-w-[200px] truncate whitespace-nowrap px-2.5 py-0.5 rounded-lg bg-slate-800 text-slate-300 border border-slate-700 text-[11px] font-medium align-middle"
                          title={maskText(c.issuer)}
                        >
                          {maskText(formatIssuer(c.issuer))}
                        </span>
                      </td>

                      <td className="py-3.5 px-4 min-w-[160px] font-sans">
                        <div className="flex justify-between text-[11px] mb-1">
                          <span className={isExpiring ? 'text-amber-400 font-bold' : 'text-emerald-400 font-bold'}>
                            {t('ssl.daysCount', { count: c.days_remaining })}
                          </span>
                          <span className="text-slate-500 font-mono">{percent}%</span>
                        </div>
                        <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                          <div
                            className={`h-full rounded-full ${isExpiring ? 'bg-amber-400' : 'bg-emerald-500'}`}
                            style={{ width: `${percent}%` }}
                          />
                        </div>
                      </td>

                      <td className="py-3.5 px-4 text-slate-400 font-mono text-[11px]">
                        {c.valid_to ? new Date(c.valid_to).toLocaleDateString() : '—'}
                      </td>

                      <td className="py-3.5 px-4 font-sans">
                        <span className="text-emerald-400 text-xs flex items-center gap-1 font-semibold">
                          <CheckCircle2 className="w-3.5 h-3.5" /> {t('ssl.autoRenewEnabled')}
                        </span>
                      </td>

                      <td className="py-3.5 px-4 text-right font-sans">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            onClick={() => handleRenew(c.id)}
                            disabled={renewingID === c.id}
                            className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold flex items-center gap-1 transition-colors disabled:opacity-50"
                            title={t('ssl.renewNow')}
                          >
                            <RotateCw className={`w-3 h-3 text-emerald-400 ${renewingID === c.id ? 'animate-spin' : ''}`} />
                            <span>{renewingID === c.id ? t('ssl.renewing') : t('ssl.renewNow')}</span>
                          </button>

                          <button
                            onClick={() => handleDelete(c.id, c.primary_domain)}
                            className="p-1 rounded text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
                            title={t('common.delete')}
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        </div>
      )}

      {/* Issue Modal */}
      {isIssueOpen && (
        <IssueSSLModal
          isOpen={true}
          servers={servers}
          onClose={() => {
            setIsIssueOpen(false);
            fetchSSLData();
          }}
          onTaskStarted={onOpenTask}
        />
      )}

      {/* Install Certbot Modal */}
      {isInstallCertbotOpen && selectedServer && (
        <InstallCertbotModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          onClose={() => setIsInstallCertbotOpen(false)}
          onTaskStarted={onOpenTask}
        />
      )}

      {/* Certbot Settings Modal */}
      {isCertbotSettingsOpen && selectedServer && certbotStatus && (
        <CertbotSettingsModal
          isOpen={true}
          serverID={selectedServer.id}
          serverName={selectedServer.name}
          version={certbotStatus.version}
          onClose={() => {
            setIsCertbotSettingsOpen(false);
            fetchSSLData();
          }}
          onConfigSaved={fetchSSLData}
        />
      )}
    </div>
  );
};

