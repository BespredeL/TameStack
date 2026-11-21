import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';
import {
  ShieldCheck,
  RotateCw,
  Play,
  Trash2,
  Copy,
  Check,
  AlertCircle,
  FileText,
  Sliders,
  KeyRound,
  Terminal,
  Save,
  CheckCircle2,
  Clock,
} from 'lucide-react';
import {
  GetCertbotConfig,
  SaveCertbotConfig,
  RunCertbotRenew,
  ListCertbotCertificates,
  DeleteCertbotCertificate,
} from '../../wailsjs/go/main/App';
import { ssl } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { ModalShell } from './common/ModalShell';
import { Input, Button, Badge, Tabs } from './ui';

interface CertbotSettingsModalProps {
  isOpen: boolean;
  serverID: string;
  serverName: string;
  version?: string;
  onClose: () => void;
  onConfigSaved?: () => void;
}

export const CertbotSettingsModal: React.FC<CertbotSettingsModalProps> = ({
  isOpen,
  serverID,
  serverName,
  version,
  onClose,
  onConfigSaved,
}) => {
  const { t } = useTranslation();
  const confirm = useConfirm();

  const [activeTab, setActiveTab] = useState<'config' | 'renew' | 'certs'>('config');

  // Config tab state
  const [, setConfig] = useState<ssl.CertbotConfig | null>(null);
  const [isLoadingConfig, setIsLoadingConfig] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);

  // Form fields
  const [email, setEmail] = useState('');
  const [keyType, setKeyType] = useState('ecdsa');
  const [rsaKeySize, setRsaKeySize] = useState(4096);
  const [ellipticCurve, setEllipticCurve] = useState('secp256r1');
  const [deployHook, setDeployHook] = useState('');
  const [autoRenew, setAutoRenew] = useState(true);
  const [agreeTos, setAgreeTos] = useState(true);

  // Renew tab state
  const [renewResult, setRenewResult] = useState<ssl.CertbotRenewResult | null>(null);
  const [isRunningRenew, setIsRunningRenew] = useState(false);
  const [renewType, setRenewType] = useState<'dry-run' | 'force' | null>(null);
  const [copied, setCopied] = useState(false);

  // Certs tab state
  const [certsList, setCertsList] = useState<ssl.CertbotCertificateItem[]>([]);
  const [isLoadingCerts, setIsLoadingCerts] = useState(false);
  const [isDeletingCert, setIsDeletingCert] = useState<string | null>(null);

  // General feedback
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const loadConfig = async () => {
    setIsLoadingConfig(true);
    setError(null);
    try {
      const res = await GetCertbotConfig(serverID);
      setConfig(res);
      setEmail(res.email || '');
      setKeyType(res.key_type || 'ecdsa');
      setRsaKeySize(res.rsa_key_size || 4096);
      setEllipticCurve(res.elliptic_curve || 'secp256r1');
      setDeployHook(res.deploy_hook || '');
      setAutoRenew(res.auto_renew);
      setAgreeTos(res.agree_tos);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsLoadingConfig(false);
    }
  };

  const loadCerts = async () => {
    setIsLoadingCerts(true);
    setError(null);
    try {
      const res = await ListCertbotCertificates(serverID);
      setCertsList(res || []);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsLoadingCerts(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      loadConfig();
      if (activeTab === 'certs') {
        loadCerts();
      }
    }
  }, [isOpen, serverID]);

  useEffect(() => {
    if (isOpen && activeTab === 'certs' && certsList.length === 0 && !isLoadingCerts) {
      loadCerts();
    }
  }, [activeTab]);

  if (!isOpen) return null;

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSavingConfig(true);
    setError(null);
    setSuccessMsg(null);
    try {
      const updated = new ssl.CertbotConfig({
        email: email.trim(),
        key_type: keyType,
        rsa_key_size: Number(rsaKeySize) || 4096,
        elliptic_curve: ellipticCurve,
        deploy_hook: deployHook.trim(),
        auto_renew: autoRenew,
        agree_tos: agreeTos,
        raw_cli_ini: '',
      });
      await SaveCertbotConfig(serverID, updated);
      setSuccessMsg(t('ssl.certbot.configSaved'));
      if (onConfigSaved) onConfigSaved();
      setTimeout(() => setSuccessMsg(null), 3500);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsSavingConfig(false);
    }
  };

  const handleRunRenew = async (dryRun: boolean) => {
    if (!dryRun) {
      if (!(await confirm(t('ssl.certbot.forceRenewConfirm')))) {
        return;
      }
    }
    setIsRunningRenew(true);
    setRenewType(dryRun ? 'dry-run' : 'force');
    setError(null);
    try {
      const res = await RunCertbotRenew(serverID, dryRun);
      setRenewResult(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsRunningRenew(false);
    }
  };

  const handleDeleteCert = async (certName: string) => {
    if (!(await confirm(t('ssl.certbot.deleteCertConfirm', { name: certName })))) {
      return;
    }
    setIsDeletingCert(certName);
    setError(null);
    try {
      await DeleteCertbotCertificate(serverID, certName);
      setSuccessMsg(t('ssl.certbot.certDeleted', { name: certName }));
      await loadCerts();
      if (onConfigSaved) onConfigSaved();
      setTimeout(() => setSuccessMsg(null), 3500);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsDeletingCert(null);
    }
  };

  const handleCopyLog = () => {
    if (!renewResult?.output) return;
    navigator.clipboard.writeText(renewResult.output);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[90vh]"
      icon={<ShieldCheck className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0"
      title={
        <div className="flex items-center gap-2">
          <span>{t('ssl.certbot.settingsModalTitle')}</span>
          {version && (
            <Badge variant="neutral" size="sm" className="font-mono">
              {version}
            </Badge>
          )}
        </div>
      }
      subtitle={t('ssl.certbot.settingsModalSubtitle', { server: serverName })}
    >
      <div className="flex flex-col flex-1 min-h-0">
        {/* Navigation Tabs with proper horizontal padding */}
        <Tabs
          variant="line"
          activeTab={activeTab}
          onChange={(id) => setActiveTab(id as 'config' | 'renew' | 'certs')}
          className="px-6 bg-surface-900 shrink-0"
          tabs={[
            {
              id: 'config',
              label: t('ssl.certbot.tabConfig'),
              icon: <Sliders className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'renew',
              label: t('ssl.certbot.tabRenew'),
              icon: <Terminal className="w-4 h-4 shrink-0" />,
            },
            {
              id: 'certs',
              label: t('ssl.certbot.tabCertificates'),
              icon: <FileText className="w-4 h-4 shrink-0" />,
            },
          ]}
        />

        {/* TAB 1: CONFIGURATION */}
        {activeTab === 'config' && (
          isLoadingConfig ? (
            <div className="py-16 flex flex-col items-center justify-center gap-3 text-slate-400 text-xs">
              <RotateCw className="w-6 h-6 animate-spin text-indigo-400" />
              <span>{t('common.loadingShort')}</span>
            </div>
          ) : (
            <form onSubmit={handleSaveConfig} className="flex flex-col flex-1 min-h-0">
              <div className="p-6 space-y-5 overflow-y-auto max-h-[calc(85vh-170px)] custom-scrollbar flex-1">
                {/* Global Feedback Notifications */}
                {error && (
                  <div className="p-3.5 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-2 text-xs text-rose-300">
                    <div className="flex items-center gap-2 min-w-0">
                      <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
                      <span className="break-all">{error}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => setError(null)}
                      className="text-xs hover:underline text-rose-400 shrink-0 cursor-pointer"
                    >
                      ×
                    </button>
                  </div>
                )}

                {successMsg && (
                  <div className="p-3.5 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-center gap-2 text-xs text-emerald-300">
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                    <span>{successMsg}</span>
                  </div>
                )}

                {/* Auto-renew Switch Card */}
                <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-semibold text-slate-200">
                        {t('ssl.certbot.autoRenewLabel')}
                      </span>
                      <Badge variant={autoRenew ? 'success' : 'neutral'} size="sm">
                        <span
                          className={`w-1.5 h-1.5 rounded-full mr-1.5 ${
                            autoRenew ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                          }`}
                        />
                        {autoRenew ? t('ssl.certbot.renewActive') : t('ssl.certbot.renewInactive')}
                      </Badge>
                    </div>
                    <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                      {autoRenew
                        ? t('ssl.certbot.autoRenewActiveDesc')
                        : t('ssl.certbot.autoRenewInactiveDesc')}
                    </p>
                  </div>

                  <button
                    type="button"
                    role="switch"
                    aria-checked={autoRenew}
                    onClick={() => setAutoRenew(!autoRenew)}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none cursor-pointer shrink-0 ${
                      autoRenew ? 'bg-emerald-500' : 'bg-slate-700'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform shadow ${
                        autoRenew ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                {/* Admin Email */}
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-300 block">
                    {t('ssl.certbot.emailLabel')}
                  </label>
                  <Input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t('ssl.certbot.emailPlaceholder')}
                    className="font-mono"
                  />
                  <p className="text-[11px] text-slate-400 leading-relaxed">{t('ssl.certbot.emailHint')}</p>
                </div>

                {/* Key Type Selector */}
                <div className="space-y-2.5">
                  <label className="text-xs font-semibold text-slate-300 block">
                    {t('ssl.certbot.keyTypeLabel')}
                  </label>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div
                      onClick={() => setKeyType('ecdsa')}
                      className={`p-3.5 rounded-xl border cursor-pointer transition flex items-start gap-3 ${
                        keyType === 'ecdsa'
                          ? 'bg-indigo-600/10 border-indigo-500/50 text-indigo-300 shadow-sm'
                          : 'bg-slate-900/60 border-slate-800 hover:border-slate-700 text-slate-400'
                      }`}
                    >
                      <div className={`p-2 rounded-lg shrink-0 ${keyType === 'ecdsa' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-800 text-slate-400'}`}>
                        <KeyRound className="w-4 h-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="font-semibold text-xs text-white">
                          {t('ssl.certbot.keyTypeEcdsa')}
                        </div>
                        <div className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                          {t('ssl.certbot.keyTypeEcdsaDesc')}
                        </div>
                      </div>
                    </div>

                    <div
                      onClick={() => setKeyType('rsa')}
                      className={`p-3.5 rounded-xl border cursor-pointer transition flex items-start gap-3 ${
                        keyType === 'rsa'
                          ? 'bg-indigo-600/10 border-indigo-500/50 text-indigo-300 shadow-sm'
                          : 'bg-slate-900/60 border-slate-800 hover:border-slate-700 text-slate-400'
                      }`}
                    >
                      <div className={`p-2 rounded-lg shrink-0 ${keyType === 'rsa' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-800 text-slate-400'}`}>
                        <KeyRound className="w-4 h-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="font-semibold text-xs text-white">
                          {t('ssl.certbot.keyTypeRsa')}
                        </div>
                        <div className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                          2048 / 4096 бит
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Sub-options for active key type */}
                  <div className="p-3 bg-slate-950/60 rounded-xl border border-slate-800/80 flex flex-wrap items-center justify-between gap-3">
                    <span className="text-xs text-slate-300 font-medium">
                      {keyType === 'rsa' ? t('ssl.certbot.rsaSizeLabel') : t('ssl.certbot.ellipticCurveLabel')}:
                    </span>
                    {keyType === 'rsa' ? (
                      <div className="flex gap-2">
                        {[2048, 4096].map((sz) => (
                          <Button
                            key={sz}
                            variant={rsaKeySize === sz ? 'primary' : 'secondary'}
                            size="xs"
                            onClick={() => setRsaKeySize(sz)}
                            className="font-mono"
                          >
                            {sz} бит
                          </Button>
                        ))}
                      </div>
                    ) : (
                      <div className="flex gap-2">
                        {['secp256r1', 'secp384r1'].map((c) => (
                          <Button
                            key={c}
                            variant={ellipticCurve === c ? 'primary' : 'secondary'}
                            size="xs"
                            onClick={() => setEllipticCurve(c)}
                            className="font-mono"
                          >
                            {c}
                          </Button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* Deploy Hook */}
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-300 block">
                    {t('ssl.certbot.deployHookLabel')}
                  </label>
                  <Input
                    type="text"
                    value={deployHook}
                    onChange={(e) => setDeployHook(e.target.value)}
                    placeholder="systemctl reload nginx 2>/dev/null || systemctl reload apache2 2>/dev/null || true"
                    className="font-mono"
                  />
                  <div className="flex flex-wrap items-center gap-1.5 pt-1 text-xs">
                    <span className="text-slate-400 text-[11px] font-medium mr-1">
                      {t('ssl.certbot.presetsLabel')}
                    </span>
                    <button
                      type="button"
                      onClick={() => setDeployHook('systemctl reload nginx 2>/dev/null || systemctl reload apache2 2>/dev/null || true')}
                      className="px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800 text-[11px] transition cursor-pointer"
                    >
                      {t('ssl.certbot.presetAutoReload')}
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeployHook('systemctl reload nginx')}
                      className="px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800 text-[11px] transition cursor-pointer"
                    >
                      {t('ssl.certbot.presetNginxOnly')}
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeployHook('systemctl reload apache2')}
                      className="px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-800 text-[11px] transition cursor-pointer"
                    >
                      {t('ssl.certbot.presetApache2Only')}
                    </button>
                  </div>
                  <p className="text-[11px] text-slate-400 leading-relaxed pt-0.5">{t('ssl.certbot.deployHookHint')}</p>
                </div>

                {/* Agree TOS Card */}
                <label className="flex items-start gap-3 p-3.5 rounded-xl bg-slate-900/60 border border-slate-800 cursor-pointer hover:bg-slate-900 transition">
                  <input
                    type="checkbox"
                    checked={agreeTos}
                    onChange={(e) => setAgreeTos(e.target.checked)}
                    className="mt-0.5 rounded border-slate-700 bg-slate-950 text-indigo-600 focus:ring-0 cursor-pointer"
                  />
                  <div className="text-xs">
                    <span className="font-semibold text-slate-200">{t('ssl.certbot.agreeTosLabel')}</span>
                    <p className="text-[11px] text-slate-400 mt-0.5 leading-relaxed">
                      {t('ssl.certbot.agreeTosDesc')}
                    </p>
                  </div>
                </label>
              </div>

              {/* Modal Sticky Footer */}
              <div className="px-6 py-4 bg-slate-950/60 border-t border-slate-800 flex items-center justify-between shrink-0">
                <span className="text-[11px] text-slate-500 font-mono hidden sm:inline">
                  /etc/letsencrypt/cli.ini
                </span>

                <div className="flex items-center gap-2.5 ml-auto">
                  <Button
                    variant="secondary"
                    size="md"
                    onClick={onClose}
                    disabled={isSavingConfig}
                  >
                    {t('common.cancel')}
                  </Button>
                  <Button
                    type="submit"
                    variant="primary"
                    size="md"
                    loading={isSavingConfig}
                    icon={<Save className="w-3.5 h-3.5" />}
                  >
                    {t('ssl.certbot.saveConfigBtn')}
                  </Button>
                </div>
              </div>
            </form>
          )
        )}

        {/* TAB 2: TESTING & RENEWAL */}
        {activeTab === 'renew' && (
          <div className="flex flex-col flex-1 min-h-0">
            <div className="p-6 space-y-5 overflow-y-auto max-h-[calc(85vh-170px)] custom-scrollbar flex-1">
              {error && (
                <div className="p-3.5 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-2 text-xs text-rose-300">
                  <div className="flex items-center gap-2 min-w-0">
                    <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
                    <span className="break-all">{error}</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setError(null)}
                    className="text-xs hover:underline text-rose-400 shrink-0 cursor-pointer"
                  >
                    ×
                  </button>
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
                {/* Dry Run Box */}
                <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3.5 flex flex-col justify-between">
                  <div>
                    <h4 className="text-xs font-bold text-white flex items-center gap-2">
                      <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                      <span>{t('ssl.certbot.dryRunBtn')}</span>
                    </h4>
                    <p className="text-[11px] text-slate-400 mt-1.5 leading-relaxed">
                      {t('ssl.certbot.dryRunDesc')}
                    </p>
                  </div>
                  <Button
                    variant="secondary"
                    size="md"
                    onClick={() => handleRunRenew(true)}
                    loading={isRunningRenew && renewType === 'dry-run'}
                    disabled={isRunningRenew}
                    icon={<Play className="w-3.5 h-3.5 text-emerald-400" />}
                    className="w-full"
                  >
                    {t('ssl.certbot.dryRunBtn')}
                  </Button>
                </div>

                {/* Force Renew Box */}
                <div className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3.5 flex flex-col justify-between">
                  <div>
                    <h4 className="text-xs font-bold text-white flex items-center gap-2">
                      <RotateCw className="w-4 h-4 text-amber-400" />
                      <span>{t('ssl.certbot.forceRenewBtn')}</span>
                    </h4>
                    <p className="text-[11px] text-slate-400 mt-1.5 leading-relaxed">
                      {t('ssl.certbot.forceRenewDesc')}
                    </p>
                  </div>
                  <Button
                    variant="secondary"
                    size="md"
                    onClick={() => handleRunRenew(false)}
                    loading={isRunningRenew && renewType === 'force'}
                    disabled={isRunningRenew}
                    icon={<RotateCw className="w-3.5 h-3.5 text-amber-400" />}
                    className="w-full bg-amber-600/15 hover:bg-amber-600/25 text-amber-300 border-amber-500/40"
                  >
                    {t('ssl.certbot.forceRenewBtn')}
                  </Button>
                </div>
              </div>

              {/* Terminal Console Output */}
              {renewResult && (
                <div className="space-y-2 pt-2">
                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-2">
                      <Terminal className="w-4 h-4 text-slate-400" />
                      <span className="font-semibold text-slate-200">
                        {t('ssl.certbot.outputLogTitle')}
                      </span>
                      <Badge variant={renewResult.success ? 'success' : 'danger'} size="sm">
                        {renewResult.success ? t('ssl.certbot.renewSuccess') : t('ssl.certbot.renewFailed')}
                      </Badge>
                    </div>

                    <Button
                      variant="secondary"
                      size="xs"
                      onClick={handleCopyLog}
                      icon={copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                    >
                      {copied ? t('ssl.certbot.logCopied') : t('ssl.certbot.copyLogBtn')}
                    </Button>
                  </div>

                  <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-[11px] text-slate-300 max-h-72 overflow-y-auto whitespace-pre-wrap select-text leading-relaxed custom-scrollbar">
                    {renewResult.output || t('ssl.certbot.noOutput')}
                  </div>
                </div>
              )}
            </div>

            {/* Modal Sticky Footer */}
            <div className="px-6 py-4 bg-slate-950/60 border-t border-slate-800 flex items-center justify-end shrink-0">
              <Button
                variant="secondary"
                size="md"
                onClick={onClose}
              >
                {t('common.close')}
              </Button>
            </div>
          </div>
        )}

        {/* TAB 3: CERTIFICATES ON DISK */}
        {activeTab === 'certs' && (
          <div className="flex flex-col flex-1 min-h-0">
            <div className="p-6 space-y-4 overflow-y-auto max-h-[calc(85vh-170px)] custom-scrollbar flex-1">
              <div className="flex items-center justify-between">
                <span className="text-xs text-slate-400">
                  {t('ssl.certbot.registeredCerts', { count: certsList.length })}
                </span>

                <Button
                  variant="secondary"
                  size="sm"
                  onClick={loadCerts}
                  loading={isLoadingCerts}
                  icon={<RotateCw className="w-3.5 h-3.5" />}
                >
                  {t('common.refresh')}
                </Button>
              </div>

              {error && (
                <div className="p-3.5 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-2 text-xs text-rose-300">
                  <div className="flex items-center gap-2 min-w-0">
                    <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
                    <span className="break-all">{error}</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setError(null)}
                    className="text-xs hover:underline text-rose-400 shrink-0 cursor-pointer"
                  >
                    ×
                  </button>
                </div>
              )}

              {successMsg && (
                <div className="p-3.5 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-center gap-2 text-xs text-emerald-300">
                  <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                  <span>{successMsg}</span>
                </div>
              )}

              {isLoadingCerts ? (
                <div className="py-16 flex flex-col items-center justify-center gap-3 text-slate-400 text-xs">
                  <RotateCw className="w-6 h-6 animate-spin text-indigo-400" />
                  <span>{t('common.loadingShort')}</span>
                </div>
              ) : certsList.length === 0 ? (
                <div className="py-16 text-center text-slate-500 text-xs bg-slate-900/30 border border-slate-800/60 rounded-xl">
                  {t('ssl.certbot.noCertificatesFound')}
                </div>
              ) : (
                <div className="space-y-3">
                  {certsList.map((cert) => {
                    const isExp = cert.is_expired || cert.days_left <= 0;
                    const isWarn = !isExp && cert.days_left <= 30;

                    return (
                      <div
                        key={cert.name}
                        className="p-4 bg-slate-900/60 border border-slate-800 rounded-xl space-y-3 shadow-sm hover:border-slate-700/80 transition"
                      >
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800/80 pb-2.5">
                          <div className="flex items-center gap-2.5 flex-wrap min-w-0">
                            <h4 className="font-mono text-sm font-bold text-white truncate">
                              {cert.name}
                            </h4>
                            {cert.key_type && (
                              <Badge variant="neutral" size="sm" className="font-mono">
                                {cert.key_type}
                              </Badge>
                            )}
                          </div>

                          <div className="flex items-center gap-2 self-end sm:self-auto shrink-0">
                            <Badge
                              variant={isExp ? 'danger' : isWarn ? 'warning' : 'success'}
                              size="sm"
                              className="inline-flex items-center gap-1.5"
                            >
                              <Clock className="w-3 h-3" />
                              <span>
                                {isExp
                                  ? t('ssl.certbot.expired')
                                  : t('ssl.certbot.daysLeft', { count: cert.days_left })}
                              </span>
                            </Badge>

                            <button
                              type="button"
                              onClick={() => handleDeleteCert(cert.name)}
                              disabled={isDeletingCert === cert.name}
                              className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition cursor-pointer disabled:opacity-50"
                              title={t('ssl.certbot.deleteCertBtn')}
                            >
                              <Trash2 className={`w-3.5 h-3.5 ${isDeletingCert === cert.name ? 'animate-spin' : ''}`} />
                            </button>
                          </div>
                        </div>

                        {/* Domains */}
                        <div className="flex flex-wrap items-center gap-1.5 text-xs">
                          <span className="text-slate-400 font-medium text-[11px] mr-1">
                            {t('ssl.certbot.domainsLabel')}
                          </span>
                          {cert.domains?.map((d) => (
                            <span
                              key={d}
                              className="px-2 py-0.5 rounded-lg bg-slate-950 border border-slate-800 text-slate-300 font-mono text-[11px]"
                            >
                              {d}
                            </span>
                          ))}
                        </div>

                        {/* Paths */}
                        <div className="space-y-1 text-[11px] font-mono text-slate-400 pt-1 border-t border-slate-800/40">
                          {cert.cert_path && (
                            <div className="truncate flex items-center gap-1.5" title={cert.cert_path}>
                              <span className="text-slate-500 font-medium">{t('ssl.certbot.certPathLabel')}</span>
                              <span className="text-slate-300 select-all">{cert.cert_path}</span>
                            </div>
                          )}
                          {cert.key_path && (
                            <div className="truncate flex items-center gap-1.5" title={cert.key_path}>
                              <span className="text-slate-500 font-medium">{t('ssl.certbot.keyPathLabel')}</span>
                              <span className="text-slate-300 select-all">{cert.key_path}</span>
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Modal Sticky Footer */}
            <div className="px-6 py-4 bg-slate-950/60 border-t border-slate-800 flex items-center justify-end shrink-0">
              <Button
                variant="secondary"
                size="md"
                onClick={onClose}
              >
                {t('common.close')}
              </Button>
            </div>
          </div>
        )}
      </div>
    </ModalShell>
  );
};
