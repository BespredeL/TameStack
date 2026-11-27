import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';
import { EventsOn } from '../../wailsjs/runtime/runtime';

import {
  Globe,
  Plus,
  RefreshCw,
  ExternalLink,
  Terminal,
  Trash2,
  Folder,
  ShieldCheck,
  AlertTriangle,
  Loader2,
  Sparkles,
  Settings,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import { ListSites, DeleteSite, DiscoverSites, AcceptDiscoveredSite } from '../../wailsjs/go/main/App';
import { db } from '../../wailsjs/go/models';
import { PageToolbar, Button, SearchInput, Badge, EmptyState } from '../components/ui';
import { CreateSiteModal } from '../components/CreateSiteModal';
import { SiteLogsModal } from '../components/SiteLogsModal';
import { ManageSiteModal } from '../components/ManageSiteModal';
import { ModalShell } from '../components/common/ModalShell';
import { useServerData } from '../hooks/useServerData';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { LoadingState } from '../components/LoadingState';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import { usePrivacy } from '../context/PrivacyContext';

interface SitesViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
  onNavigateToAI?: (serverId?: string, prompt?: string) => void;
}

export const SitesView: React.FC<SitesViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
  onNavigateToAI,
}) => {
  const {
    selectedServerId: selectedServerID,
    setSelectedServerId: setSelectedServerID,
    setPageLoading,
  } = useServer();
  const { t, language } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const { maskDomain, maskPath } = usePrivacy();
  const [search, setSearch] = useState('');
  const [isDiscovering, setIsDiscovering] = useState(false);
  const [discoverMessage, setDiscoverMessage] = useState<string | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [logsSite, setLogsSite] = useState<db.SiteRecord | null>(null);
  const [manageSite, setManageSite] = useState<db.SiteRecord | null>(null);

  // Deletion modal state
  const [deleteCandidate, setDeleteCandidate] = useState<db.SiteRecord | null>(null);
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const selectedServer = servers.find((s) => s.id === selectedServerID) || servers[0];

  const {
    data: sites,
    setData: setSites,
    loading: isLoading,
    reload: fetchSites,
  } = useServerData(
    (serverId) => ListSites(serverId).then((r) => r || []),
    [],
    {
      initial: [] as db.SiteRecord[],
      reset: [] as db.SiteRecord[],
      loadingMessage: selectedServer ? t('sites.loadingSites', { server: selectedServer.name }) : undefined,
      onError: (msg) => console.error('Failed to fetch sites:', msg),
    },
  );

  useEffect(() => {
    const unregister = EventsOn('task:finished', (data: TaskRecord) => {
      if (
        data &&
        data.server_id === selectedServerID &&
        data.status === 'completed' &&
        (data.operation_type?.startsWith('site_') ||
          data.operation_type === 'ssl_issue' ||
          data.operation_type === 'user_create_site' ||
          data.operation_type === 'user_chown_site')
      ) {
        fetchSites();
      }
    });
    return () => {
      if (typeof unregister === 'function') unregister();
    };
  }, [selectedServerID, fetchSites]);

  const handleDiscoverSites = async () => {
    if (!selectedServer) return;
    setIsDiscovering(true);
    setPageLoading(true, t('sites.scanningVirtualHosts', { server: selectedServer.name }));
    setDiscoverMessage(null);
    try {
      const discovered = await DiscoverSites(selectedServer.id);
      setSites(discovered || []);
      const count = discovered?.length || 0;
      setDiscoverMessage(t('sites.syncComplete', { count }));
      setTimeout(() => setDiscoverMessage(null), 6000);
    } catch (err: unknown) {
      console.error('Discover sites error:', err);
      setDiscoverMessage(t('sites.discoverError', { error: getErrorMessage(err) }));
    } finally {
      setIsDiscovering(false);
      setPageLoading(false);
    }
  };

  const handleDeleteSite = async () => {
    if (!deleteCandidate) return;
    setIsDeleting(true);
    setDeleteError(null);
    try {
      await DeleteSite(deleteCandidate.id, deleteFiles);
      setDeleteCandidate(null);
      setDeleteFiles(false);
      await fetchSites();
    } catch (err: unknown) {
      setDeleteError(getErrorMessage(err));
    } finally {
      setIsDeleting(false);
    }
  };

  const handleAcceptDiscoveredSite = async (siteID: string) => {
    if (!(await confirm(t('sites.adoptConfirm')))) {
      return;
    }
    try {
      await AcceptDiscoveredSite(siteID);
      await fetchSites();
    } catch (err: unknown) {
      alert(t('sites.adoptError', { error: getErrorMessage(err) }));
    }
  };

  const filtered = sites.filter((s) => {
    const q = search.toLowerCase();
    return (
      s.domain.toLowerCase().includes(q) ||
      s.site_type.toLowerCase().includes(q) ||
      s.aliases?.some((a) => a.toLowerCase().includes(q))
    );
  });

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Globe}
        description={t('empty.sitesDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Globe className="w-5 h-5" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('sites.targetServer')}
        search={
          <SearchInput
            placeholder={t('sites.searchPlaceholder')}
            value={search}
            onChangeValue={setSearch}
          />
        }
        actions={
          <>
            <Button
              variant="secondary"
              size="md"
              onClick={handleDiscoverSites}
              disabled={isDiscovering}
              icon={<Sparkles className={`w-3.5 h-3.5 text-amber-400 ${isDiscovering ? 'animate-spin' : ''}`} />}
              title={t('sites.importTooltip')}
            >
              {isDiscovering ? t('sites.discovering') : t('sites.importSites')}
            </Button>
            <Button
              variant="secondary"
              size="icon-md"
              onClick={fetchSites}
              disabled={isLoading}
              title={t('sites.refreshTooltip')}
              aria-label={t('sites.refreshTooltip')}
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin text-indigo-400' : ''}`} />
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={() => setIsCreateOpen(true)}
              icon={<Plus className="w-4 h-4" />}
            >
              {t('sites.createSite')}
            </Button>
          </>
        }
      />

      {isDiscovering && (
        <div className="p-4 bg-indigo-950/70 border border-indigo-500/40 rounded-xl text-xs text-indigo-200 flex items-center gap-3 shadow-md animate-pulse">
          <Loader2 className="w-5 h-5 text-indigo-400 animate-spin shrink-0" />
          <div>
            <div className="font-bold text-white">{t('sites.scanningBannerTitle')}</div>
            <div className="text-slate-400 font-mono text-[11px] mt-0.5">
              {t('sites.scanningBannerDesc')}
            </div>
          </div>
        </div>
      )}

      {discoverMessage && !isDiscovering && (
        <div className="p-3 bg-indigo-950/60 border border-indigo-500/30 rounded-xl text-xs text-indigo-200 flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-amber-400 shrink-0" />
          <span>{discoverMessage}</span>
        </div>
      )}

      {/* Sites Grid */}
      {isLoading ? (
        <LoadingState
          type="cards"
          title={t('common.loadingShort')}
          subtitle={t('sites.loadingVhostContext', { name: selectedServer?.name || '', host: selectedServer?.host || '' })}
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={<Globe className="w-6 h-6 text-indigo-400" />}
          title={t('sites.emptySitesTitle')}
          description={t('sites.emptySitesDesc')}
          action={{
            label: t('sites.createSite'),
            onClick: () => setIsCreateOpen(true),
            icon: <Plus className="w-4 h-4" />,
            variant: 'primary',
          }}
        />
      ) : (
        <div className="card-grid">
          {filtered.map((s) => {
            const isHybrid = s.web_server?.toLowerCase().includes('nginx+apache') || (s.web_server?.toLowerCase().includes('nginx') && s.web_server?.toLowerCase().includes('apache'));
            const isApache = !isHybrid && s.web_server?.toLowerCase().includes('apache');
            return (
              <div
                key={s.id}
                className="site-card bg-surface-850 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 shadow-lg flex flex-col transition-colors group"
              >
                <div className="flex items-start gap-3 min-w-0">
                  <div className="w-10 h-10 rounded-lg bg-indigo-600/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0">
                    <Globe className="w-5 h-5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-start justify-between gap-2">
                      <h3 className="font-bold text-white text-sm tracking-wide truncate" title={maskDomain(s.domain)}>
                        {maskDomain(s.domain)}
                      </h3>
                      <div className="flex items-center gap-1.5 shrink-0">
                        <Badge
                          variant={s.origin === 'managed' ? 'success' : s.origin === 'discovered' ? 'warning' : 'neutral'}
                          size="xs"
                        >
                          {s.origin === 'managed' ? t('sites.originManagedLabel') : s.origin === 'discovered' ? t('sites.originDiscoveredLabel') : t('sites.originUnknownLabel')}
                        </Badge>
                        <Badge
                          variant={s.status === 'active' ? 'success' : 'neutral'}
                          dot
                          size="xs"
                        >
                          {s.status === 'active' ? 'Active' : 'Inactive'}
                        </Badge>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-1.5 mt-1.5">
                      <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-slate-800 text-indigo-300 font-bold border border-slate-700">
                        {s.site_type}
                      </span>
                      {s.php_version && (
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-300 font-bold border border-purple-500/20">
                          PHP {s.php_version}
                        </span>
                      )}
                      <span
                        className={`text-[10px] font-mono uppercase px-1.5 py-0.5 rounded font-bold border ${isHybrid
                          ? 'bg-cyan-500/10 text-cyan-300 border-cyan-500/20'
                          : isApache
                            ? 'bg-amber-500/10 text-amber-300 border-amber-500/20'
                            : 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20'
                          }`}
                      >
                        {isHybrid ? 'Nginx + Apache' : isApache ? 'Apache' : 'Nginx'}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="mt-3 p-2.5 rounded-lg bg-slate-900 border border-slate-800/80 text-xs space-y-1 min-w-0">
                  <div className="text-slate-500 text-[10px] flex items-center gap-1">
                    <Folder className="w-3 h-3 text-indigo-400 shrink-0" />
                    <span className="uppercase">{t('sites.docRoot')}</span>
                  </div>
                  <div className="font-mono text-slate-300 truncate text-[11px]" title={maskPath(s.document_root)}>
                    {maskPath(s.document_root)}
                  </div>
                  <div className="font-mono text-slate-500 truncate text-[11px] min-h-[1rem]" title={s.aliases?.map(maskDomain).join(', ')}>
                    {s.aliases && s.aliases.length > 0 ? `Aliases: ${s.aliases.map(maskDomain).join(', ')}` : '\u00a0'}
                  </div>
                </div>

                <div className="pt-3 mt-4 border-t border-slate-800 flex items-center justify-between gap-2 text-xs">
                  <div className="flex flex-wrap items-center gap-1.5 min-w-0">
                    {s.origin === 'discovered' && (
                      <Button
                        variant="secondary"
                        size="xs"
                        onClick={() => handleAcceptDiscoveredSite(s.id)}
                        title={t('sites.adoptSiteTooltip')}
                        icon={<ShieldCheck className="w-3.5 h-3.5 text-amber-400 shrink-0" />}
                      >
                        {t('sites.adoptBtn')}
                      </Button>
                    )}

                    <a
                      href={`http://${s.domain}`}
                      target="_blank"
                      rel="noreferrer"
                      className="px-2.5 py-1.5 rounded-lg bg-surface-800 hover:bg-surface-700 text-slate-300 hover:text-white transition-colors inline-flex items-center justify-center gap-1.5 font-medium whitespace-nowrap text-xs border border-slate-700/60"
                      title={t('sites.openInBrowser')}
                    >
                      <ExternalLink className="w-3.5 h-3.5 shrink-0" />
                      <span>{t('sites.visit')}</span>
                    </a>

                    <Button
                      variant="primary"
                      size="xs"
                      onClick={() => setManageSite(s)}
                      title={t('sites.manageSite')}
                      icon={<Settings className="w-3.5 h-3.5 shrink-0" />}
                    >
                      {t('sites.manageSite')}
                    </Button>

                    <Button
                      variant="secondary"
                      size="xs"
                      onClick={() => setLogsSite(s)}
                      title={t('manageSite.tabs.logs')}
                      icon={<Terminal className="w-3.5 h-3.5 shrink-0" />}
                    >
                      {t('manageSite.tabs.logs')}
                    </Button>

                    <Button
                      variant="secondary"
                      size="xs"
                      onClick={() => onNavigateToAI?.(selectedServer?.id, `Diagnose site ${s.domain} on server ${selectedServer?.name || ''}`)}
                      title={t('sites.aiDiagnosis')}
                      icon={<Sparkles className="w-3.5 h-3.5 text-indigo-400 shrink-0" />}
                    >
                      AI
                    </Button>
                  </div>

                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => setDeleteCandidate(s)}
                    className="text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 shrink-0"
                    title={t('sites.deleteSite')}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create Site Wizard Modal */}
      {isCreateOpen && (
        <CreateSiteModal
          isOpen={true}
          servers={servers}
          onClose={() => {
            setIsCreateOpen(false);
            fetchSites();
          }}
          onTaskStarted={onOpenTask}
        />
      )}

      {/* Logs Modal */}
      {logsSite && (
        <SiteLogsModal
          isOpen={true}
          siteID={logsSite.id}
          domain={logsSite.domain}
          webServer={logsSite.web_server}
          onClose={() => setLogsSite(null)}
        />
      )}

      {/* Manage Site Modal */}
      {manageSite && (
        <ManageSiteModal
          isOpen={true}
          site={manageSite}
          serverID={selectedServerID}
          onClose={() => setManageSite(null)}
          onSiteUpdated={() => {
            fetchSites();
          }}
          onOpenTask={onOpenTask}
        />
      )}

      {/* Safe Delete Confirmation Dialog */}
      {deleteCandidate && (
        <ModalShell
          onClose={() => setDeleteCandidate(null)}
          size="md"
          icon={<Trash2 className="w-5 h-5" />}
          iconClassName="p-2.5 rounded-xl bg-rose-500/10 text-rose-400 border border-rose-500/30 shrink-0"
          title={deleteCandidate.origin === 'discovered' ? t('sites.deleteDiscoveredTitle') : t('sites.deleteSiteTitle')}
          subtitle={deleteCandidate.domain}
        >
          <div className="p-6 space-y-4">
            {deleteCandidate.origin === 'discovered' ? (
              <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2.5">
                <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <div className="font-semibold text-amber-200">{t('sites.discoveredWarningTitle')}</div>
                  <p className="text-[11px] text-amber-300/90 leading-relaxed">{t('sites.discoveredWarningDesc')}</p>
                </div>
              </div>
            ) : (
              <p className="text-xs text-slate-300 leading-relaxed">
                {t('sites.deleteVhostNotice', { domain: deleteCandidate.domain, webServer: deleteCandidate.web_server || 'web' })}
              </p>
            )}

            {deleteError && <p className="text-xs text-rose-400">{deleteError}</p>}

            <label className="flex items-start gap-2.5 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={deleteFiles}
                onChange={(e) => setDeleteFiles(e.target.checked)}
                className="mt-0.5 rounded border-slate-700 text-rose-600 focus:ring-0"
              />
              <span className="text-xs text-rose-300 font-medium">
                {deleteCandidate.origin === 'discovered'
                  ? t('sites.deleteDiscoveredFilesCheckbox', { docRoot: deleteCandidate.document_root })
                  : t('sites.deleteManagedFilesCheckbox', { docRoot: deleteCandidate.document_root })}
              </span>
            </label>

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => setDeleteCandidate(null)}
                className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium"
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                onClick={handleDeleteSite}
                disabled={isDeleting}
                className="px-5 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold flex items-center gap-2 disabled:opacity-50"
              >
                {isDeleting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                <span>{t('common.delete')}</span>
              </button>
            </div>
          </div>
        </ModalShell>
      )}
    </div>
  );
};

