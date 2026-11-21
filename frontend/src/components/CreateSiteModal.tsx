import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Globe,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  Loader2,
  ShieldCheck,
  Folder,
} from 'lucide-react';
import { ServerRecord } from '../types';
import { CheckSiteDNS, PlanSiteCreation, CreateSite, InspectWebServers, GetDiagnostics } from '../../wailsjs/go/main/App';
import { site } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Select, Button } from './ui';

interface CreateSiteModalProps {
  isOpen: boolean;
  servers: ServerRecord[];
  onClose: () => void;
  onTaskStarted: (task: any) => void;
}

export const CreateSiteModal: React.FC<CreateSiteModalProps> = ({
  isOpen,
  servers,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [serverID, setServerID] = useState<string>(servers[0]?.id || '');
  const [domain, setDomain] = useState('');
  const [aliasesInput, setAliasesInput] = useState('');
  const [siteType, setSiteType] = useState<'php' | 'laravel' | 'wordpress' | 'static' | 'reverse_proxy'>('php');
  const [phpVersion, setPHPVersion] = useState('8.3');
  const [fpmPool, setFpmPool] = useState('');
  const [documentRoot, setDocumentRoot] = useState('');
  const [webServer, setWebServer] = useState<'nginx' | 'apache'>('nginx');
  const [proxyPassURL, setProxyPassURL] = useState('http://127.0.0.1:3000');
  const [serverStatusMap, setServerStatusMap] = useState<Record<string, { installed: boolean; status: string }>>({});
  const [serverDiag, setServerDiag] = useState<{ os_name: string; os_version: string } | null>(null);

  // Database & Logs options
  const [createDatabase, setCreateDatabase] = useState(false);
  const [databaseName, setDatabaseName] = useState('');
  const [databaseUser, setDatabaseUser] = useState('');
  const [databasePassword, setDatabasePassword] = useState('');
  const [enableLogs, setEnableLogs] = useState(true);

  // DNS & Plan state
  const [isCheckingDNS, setIsCheckingDNS] = useState(false);
  const [dnsResult, setDnsResult] = useState<site.SiteDNSCheckResult | null>(null);
  const [plan, setPlan] = useState<site.CreateSitePlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    if (servers.length > 0 && !serverID) {
      setServerID(servers[0].id);
    }
  }, [servers]);

  // Inspect web servers on target server to intelligently suggest default
  useEffect(() => {
    if (!serverID || !isOpen) return;
    InspectWebServers(serverID)
      .then((res: any[]) => {
        if (!res) return;
        const map: Record<string, { installed: boolean; status: string }> = {};
        let apacheActive = false;
        let nginxActive = false;

        res.forEach((ws) => {
          map[ws.name] = { installed: ws.is_installed, status: ws.status };
          if (ws.name.includes('apache') && (ws.is_installed || ws.status === 'active')) {
            apacheActive = true;
          }
          if (ws.name.includes('nginx') && (ws.is_installed || ws.status === 'active')) {
            nginxActive = true;
          }
        });

        setServerStatusMap(map);

        // Auto-select Apache if Apache is active and Nginx is not
        if (apacheActive && !nginxActive) {
          setWebServer('apache');
        } else if (nginxActive) {
          setWebServer('nginx');
        }
      })
      .catch((err) => console.log('Could not pre-inspect webservers:', err));
  }, [serverID, isOpen]);

  // Load server diagnostics to restrict PHP versions by OS archive availability
  useEffect(() => {
    if (!serverID || !isOpen) return;
    setServerDiag(null);
    GetDiagnostics(serverID)
      .then((diag: any) => {
        if (diag && diag.os_name) {
          setServerDiag({ os_name: diag.os_name, os_version: diag.os_version || '' });
        }
      })
      .catch(() => {});
  }, [serverID, isOpen]);

  const diagOSName = (serverDiag?.os_name || '').toLowerCase();
  const diagOSVersion = serverDiag?.os_version || '';
  const isUbuntuBionic = diagOSName.includes('ubuntu') && diagOSVersion.startsWith('18.04');
  const isUbuntuFocal = diagOSName.includes('ubuntu') && diagOSVersion.startsWith('20.04');
  const phpOptions = isUbuntuBionic
    ? ['7.2']
    : isUbuntuFocal
      ? ['7.4']
      : ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4', '7.3', '7.2'];
  const phpOptionsKey = phpOptions.join(',');

  // Keep the selected version valid when the target server (and its OS) changes
  useEffect(() => {
    setPHPVersion((prev) => (phpOptions.includes(prev) ? prev : phpOptions[0]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phpOptionsKey]);

  const phpOptionLabel = (v: string) =>
    v === '8.4' ? t('createSite.phpLatest')
      : v === '8.3' ? t('createSite.phpStable')
        : v === '7.4' ? t('createSite.phpLegacy')
          : `PHP ${v}`;

  // Automatically adjust default DocumentRoot and Database name when domain changes
  useEffect(() => {
    if (domain) {
      const cleanDomain = domain.trim().toLowerCase();
      if (siteType === 'laravel') {
        setDocumentRoot(`/var/www/${cleanDomain}/public`);
      } else {
        setDocumentRoot(`/var/www/${cleanDomain}`);
      }
      const rawIdent = cleanDomain.replace(/[^a-z0-9]/g, '_').slice(0, 16);
      if (!databaseName) setDatabaseName(`db_${rawIdent}`);
      if (!databaseUser) setDatabaseUser(`u_${rawIdent}`);
    }
  }, [domain, siteType]);

  if (!isOpen) return null;

  const handleNextToStep2 = () => {
    if (!domain) {
      setError('Domain name is required.');
      return;
    }
    setError(null);
    setStep(2);
  };

  const handleNextToStep3 = async () => {
    setError(null);
    setIsPlanning(true);
    const aliases = aliasesInput
      .split(',')
      .map((a) => a.trim().toLowerCase())
      .filter((a) => a.length > 0);

    const siteInput = new site.SiteInput({
      server_id: serverID,
      domain: domain.trim().toLowerCase(),
      aliases,
      document_root: documentRoot,
      site_type: siteType,
      php_version: phpVersion,
      fpm_pool: fpmPool.trim(),
      web_server: webServer,
      proxy_pass_url: proxyPassURL,
      ssl_enabled: false,
      create_database: createDatabase,
      database_name: createDatabase ? databaseName.trim() : '',
      database_user: createDatabase ? databaseUser.trim() : '',
      database_password: createDatabase ? databasePassword.trim() : '',
      enable_logs: enableLogs,
      disable_logs: !enableLogs,
      redirect_https: false,
    } as any);

    try {
      const p = await PlanSiteCreation(siteInput);
      setPlan(p);
      setStep(3);

      // Check DNS in background
      setIsCheckingDNS(true);
      CheckSiteDNS(domain.trim().toLowerCase(), serverID)
        .then((res) => setDnsResult(res))
        .catch(console.error)
        .finally(() => setIsCheckingDNS(false));
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsPlanning(false);
    }
  };

  const handleCreate = async () => {
    setIsCreating(true);
    setError(null);
    const aliases = aliasesInput
      .split(',')
      .map((a) => a.trim().toLowerCase())
      .filter((a) => a.length > 0);

    const siteInput = new site.SiteInput({
      server_id: serverID,
      domain: domain.trim().toLowerCase(),
      aliases,
      document_root: documentRoot,
      site_type: siteType,
      php_version: phpVersion,
      fpm_pool: fpmPool.trim(),
      web_server: webServer,
      proxy_pass_url: proxyPassURL,
      ssl_enabled: false,
      create_database: createDatabase,
      database_name: createDatabase ? databaseName.trim() : '',
      database_user: createDatabase ? databaseUser.trim() : '',
      database_password: createDatabase ? databasePassword.trim() : '',
      enable_logs: enableLogs,
      disable_logs: !enableLogs,
      redirect_https: false,
    } as any);

    try {
      const task = await CreateSite(siteInput);
      onTaskStarted(task);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[90vh]"
      icon={<Globe className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={t('createSite.modalTitle')}
      subtitle={<>{t('common.step')} {step}/3: {step === 1 ? t('createSite.step1') : step === 2 ? t('createSite.step2') : t('createSite.step3')}</>}
    >
      {/* Form Body */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {error && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* STEP 1: General Info */}
        {step === 1 && (
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('sites.targetServer')}</label>
              <Select
                value={serverID}
                onChange={(e) => setServerID(e.target.value)}
              >
                {servers.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.host})
                  </option>
                ))}
              </Select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.domain')} *</label>
              <Input
                type="text"
                required
                placeholder={t('createSite.domainPlaceholder')}
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                className="font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.aliasesLabel')}</label>
              <Input
                type="text"
                placeholder={t('createSite.aliasesPlaceholder')}
                value={aliasesInput}
                onChange={(e) => setAliasesInput(e.target.value)}
                className="font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-2">{t('createSite.archAndType')}</label>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2.5">
                {[
                  { id: 'php', label: t('createSite.typePhp'), desc: t('createSite.typePhpDesc') },
                  { id: 'laravel', label: t('createSite.typeLaravel'), desc: t('createSite.typeLaravelDesc') },
                  { id: 'wordpress', label: t('createSite.typeWordpress'), desc: t('createSite.typeWordpressDesc') },
                  { id: 'static', label: t('createSite.typeStatic'), desc: t('createSite.typeStaticDesc') },
                  { id: 'reverse_proxy', label: t('createSite.typeReverseProxy'), desc: t('createSite.typeReverseProxyDesc') },
                ].map((tItem) => (
                  <button
                    key={tItem.id}
                    type="button"
                    onClick={() => setSiteType(tItem.id as 'php' | 'laravel' | 'wordpress' | 'static' | 'reverse_proxy')}
                    className={`p-3 rounded-lg border text-left transition-all ${
                      siteType === tItem.id
                        ? 'bg-indigo-600/20 border-indigo-500 text-white shadow-md shadow-indigo-600/20'
                        : 'bg-slate-950 border-slate-800 text-slate-400 hover:text-white'
                    }`}
                  >
                    <div className="font-semibold text-xs text-white">{tItem.label}</div>
                    <div className="text-[10px] text-slate-400 mt-0.5 leading-tight">{tItem.desc}</div>
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* STEP 2: Stack & Routing */}
        {step === 2 && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.webServerLabel')}</label>
                <Select
                  value={webServer}
                  onChange={(e) => setWebServer(e.target.value as 'nginx' | 'apache')}
                >
                  <option value="nginx">
                    Nginx {serverStatusMap['nginx']?.status === 'active' ? t('createSite.activeOnServerBadge') : ''}
                  </option>
                  <option value="apache">
                    Apache HTTP Server {serverStatusMap['apache2']?.status === 'active' || serverStatusMap['apache']?.status === 'active' ? t('createSite.activeOnServerBadge') : ''}
                  </option>
                </Select>
                <p className="text-[11px] text-slate-400 mt-1">
                  {webServer === 'apache'
                    ? t('createSite.apacheDesc') : t('createSite.nginxDesc')}
                </p>
              </div>

              {siteType !== 'static' && siteType !== 'reverse_proxy' && (
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.phpVersionLabel')}</label>
                  <Select
                    value={phpVersion}
                    onChange={(e) => setPHPVersion(e.target.value)}
                  >
                    {phpOptions.map((v) => (
                      <option key={v} value={v}>{phpOptionLabel(v)}</option>
                    ))}
                  </Select>
                  <p className="text-[11px] text-slate-400 mt-1">
                    {t('createSite.fastcgiPoolHint', { version: phpVersion })}
                  </p>
                  {(isUbuntuBionic || isUbuntuFocal) && (
                    <p className="text-[11px] text-amber-400 mt-1">
                      {t('createSite.phpOsRestricted', { os: serverDiag?.os_name || '', version: phpOptions[0] })}
                    </p>
                  )}
                </div>
              )}
            </div>

            {siteType !== 'static' && siteType !== 'reverse_proxy' && (
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  {t('createSite.phpPoolLabel')}
                </label>
                <Input
                  type="text"
                  placeholder={t('createSite.phpPoolPlaceholder')}
                  value={fpmPool}
                  onChange={(e) => setFpmPool(e.target.value)}
                  className="font-mono"
                />
                <p className="text-[11px] text-slate-500 mt-1">
                  {t('createSite.phpPoolEmptyHint')}. {t('createSite.phpPoolCustomSocketHint')} <code className="text-slate-400">{`/run/php/php${phpVersion}-fpm-${fpmPool || 'pool'}.sock`}</code>
                </p>
              </div>
            )}

            {siteType === 'reverse_proxy' ? (
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.proxyUrlLabel')} *</label>
                <Input
                  type="text"
                  required
                  placeholder={t('createSite.proxyUrlPlaceholder')}
                  value={proxyPassURL}
                  onChange={(e) => setProxyPassURL(e.target.value)}
                  className="font-mono"
                />
                <p className="text-[11px] text-slate-500 mt-1">{t('createSite.proxyUrlHint')}</p>
              </div>
            ) : (
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">{t('createSite.docRootLabel')}</label>
                <Input
                  type="text"
                  required
                  icon={<Folder className="w-4 h-4 text-slate-500" />}
                  value={documentRoot}
                  onChange={(e) => setDocumentRoot(e.target.value)}
                  className="font-mono"
                />
                <p className="text-[11px] text-slate-500 mt-1">{t('createSite.docRootAutoHint')}</p>
              </div>
            )}

            {/* Optional Database Provisioning */}
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={createDatabase}
                  onChange={(e) => setCreateDatabase(e.target.checked)}
                  className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                />
                <span className="text-xs font-semibold text-white">
                  {t('createSite.createDbCheckbox')}
                </span>
              </label>

              {createDatabase && (
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1">
                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">{t('createSite.dbNameLabel')}</label>
                    <Input
                      type="text"
                      sizeVariant="sm"
                      value={databaseName}
                      onChange={(e) => setDatabaseName(e.target.value)}
                      placeholder="db_site"
                      className="font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">{t('createSite.dbUserLabel')}</label>
                    <Input
                      type="text"
                      sizeVariant="sm"
                      value={databaseUser}
                      onChange={(e) => setDatabaseUser(e.target.value)}
                      placeholder="u_site"
                      className="font-mono"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">{t('createSite.dbPasswordLabel')}</label>
                    <Input
                      type="password"
                      sizeVariant="sm"
                      value={databasePassword}
                      onChange={(e) => setDatabasePassword(e.target.value)}
                      placeholder={t('createSite.autoGeneratePlaceholder')}
                      className="font-mono"
                    />
                  </div>
                </div>
              )}
            </div>

            {/* Logs Option */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs">
              <div>
                <span className="font-semibold text-white block">{t('createSite.webServerLogsHeading')}</span>
                <span className="text-[11px] text-slate-400">{t('createSite.webServerLogsDesc')}</span>
              </div>
              <input
                type="checkbox"
                checked={enableLogs}
                onChange={(e) => setEnableLogs(e.target.checked)}
                className="rounded border-slate-700 text-indigo-600 focus:ring-0"
              />
            </div>
          </div>
        )}

        {/* STEP 3: DNS Check & Review Plan */}
        {step === 3 && plan && (
          <div className="space-y-4">
            {/* Conflict Alert */}
            {plan.existing_site_conflict && (
              <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2.5">
                <AlertCircle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
                <div>
                  <div className="font-bold text-amber-200">{t('createSite.serverNameConflict')}</div>
                  <p className="mt-0.5 text-amber-300/90 leading-relaxed">{plan.conflict_message}</p>
                </div>
              </div>
            )}

            {/* Preflight Warnings */}
            {plan.preflight_warnings && plan.preflight_warnings.length > 0 && (
              <div className="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs space-y-1.5">
                <div className="font-semibold text-amber-200 flex items-center gap-1.5">
                  <AlertCircle className="w-4 h-4 text-amber-400 shrink-0" />
                  <span>{t('createSite.preflightWarnings')}</span>
                </div>
                <ul className="list-disc list-inside space-y-0.5 text-amber-300/90 pl-1 font-mono text-[11px]">
                  {plan.preflight_warnings.map((warn, i) => (
                    <li key={i}>{warn}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* DNS Verification Box */}
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
              <div className="flex items-center justify-between text-xs">
                <span className="font-bold text-slate-200">{t('createSite.dnsCheckHeading')}</span>
                {isCheckingDNS ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin text-indigo-400" />
                ) : dnsResult?.match ? (
                  <span className="text-emerald-400 font-semibold flex items-center gap-1">
                    <CheckCircle2 className="w-3.5 h-3.5" /> {t('createSite.dnsCorrect')}
                  </span>
                ) : (
                  <span className="text-amber-400 font-semibold flex items-center gap-1">
                    <AlertCircle className="w-3.5 h-3.5" /> {t('createSite.dnsIncorrect')}
                  </span>
                )}
              </div>

              <div className="text-xs text-slate-400">
                {dnsResult?.message || t('createSite.dnsChecking', { domain })}
              </div>
            </div>

            {/* Deployment Blueprint */}
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5 text-xs">
              <span className="font-bold text-slate-200 block">{t('createSite.autoPlanHeading')}</span>

              <div className="space-y-1.5 font-mono text-[11px] text-slate-300">
                <div className="flex justify-between border-b border-slate-800/80 pb-1">
                  <span className="text-slate-500">VirtualHost:</span>
                  <span className="text-indigo-300">{plan.config_file}</span>
                </div>
                <div className="flex justify-between border-b border-slate-800/80 pb-1">
                  <span className="text-slate-500">DocumentRoot:</span>
                  <span className="text-slate-200">{plan.document_root}</span>
                </div>
                <div className="flex justify-between border-b border-slate-800/80 pb-1">
                  <span className="text-slate-500">{t('createSite.planOwner')}</span>
                  <span className="text-emerald-400">{plan.owner_user || 'www-data:www-data'}</span>
                </div>
                <div className="flex justify-between border-b border-slate-800/80 pb-1">
                  <span className="text-slate-500">{t('createSite.planWebServer')}</span>
                  <span className="text-sky-300">{plan.webserver_to_reload}</span>
                </div>
                {siteType !== 'static' && siteType !== 'reverse_proxy' && (
                  <div className="flex justify-between border-b border-slate-800/80 pb-1">
                    <span className="text-slate-500">{t('createSite.fastcgiPoolLabel')}</span>
                    <span className="text-purple-300">{plan.fpm_socket}</span>
                  </div>
                )}
                {plan.database_planned && (
                  <div className="flex justify-between border-b border-slate-800/80 pb-1">
                    <span className="text-slate-500">{t('createSite.planDatabase')}</span>
                    <span className="text-amber-300">{plan.database_name} {t('createSite.planDatabaseUser', { user: plan.database_user })}</span>
                  </div>
                )}
                {plan.packages_to_install && plan.packages_to_install.length > 0 && (
                  <div className="flex justify-between border-b border-slate-800/80 pb-1">
                    <span className="text-slate-500">{t('createSite.planWillInstall')}</span>
                    <span className="text-amber-400">{plan.packages_to_install.join(', ')}</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-slate-500">{t('createSite.planLogging')}</span>
                  <span className="text-slate-400">{enableLogs ? plan.access_log_file : t('createSite.planLoggingDisabled')}</span>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        {step > 1 ? (
          <Button
            variant="secondary"
            size="md"
            onClick={() => setStep((step - 1) as 1 | 2 | 3)}
            icon={<ArrowLeft className="w-3.5 h-3.5" />}
          >
            {t('common.back')}
          </Button>
        ) : (
          <Button
            variant="secondary"
            size="md"
            onClick={onClose}
          >
            {t('common.cancel')}
          </Button>
        )}

        {step === 1 && (
          <Button
            variant="primary"
            size="md"
            onClick={handleNextToStep2}
            iconRight={<ArrowRight className="w-3.5 h-3.5" />}
          >
            {t('common.next')}
          </Button>
        )}

        {step === 2 && (
          <Button
            variant="primary"
            size="md"
            onClick={handleNextToStep3}
            loading={isPlanning}
            iconRight={<ArrowRight className="w-3.5 h-3.5" />}
          >
            {t('common.next')}
          </Button>
        )}

        {step === 3 && (
          <Button
            variant="primary"
            size="md"
            onClick={handleCreate}
            loading={isCreating}
            disabled={Boolean(plan?.existing_site_conflict)}
            icon={<ShieldCheck className="w-4 h-4" />}
          >
            {t('createSite.createSiteBtn')}
          </Button>
        )}
      </div>
    </ModalShell>
  );
};
