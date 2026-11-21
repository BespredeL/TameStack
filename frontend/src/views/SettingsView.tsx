import React, { useState, useEffect } from 'react';
import {
  Shield,
  Database,
  HardDrive,
  FolderOpen,
  Sparkles,
  RefreshCw,
  Copy,
  Check,
  AlertTriangle,
  Sliders,
  CheckCircle,
  Lock,
  Unlock,
  KeyRound,
  FileArchive,
  Clock,
} from 'lucide-react';
import { useTranslation } from '../i18n/LanguageContext';
import {
  GetAppSettings,
  OpenDataDirectory,
  VacuumDatabase,
  AIClearFacts,
  GetVaultStatus,
  SetMasterPassword,
  UnlockVault,
  LockVault,
  SetAutoLockMinutes,
  SetAppLogLevel,
  ExportSupportBundle,
  EnableDatabaseEncryption,
  AIGetGlobalSettings,
  AIListProviderConfigs,
} from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { AISettingsModal } from '../components/ai/AISettingsModal';
import { ClearFactsConfirmModal } from '../components/settings/ClearFactsConfirmModal';
import { DbEncryptionModal } from '../components/settings/DbEncryptionModal';
import { MasterPasswordModal } from '../components/settings/MasterPasswordModal';
import { UnlockVaultModal } from '../components/settings/UnlockVaultModal';
import { LanguageSection } from '../components/settings/LanguageSection';
import { UiTerminalSection } from '../components/settings/UiTerminalSection';
import { SystemInfoSection } from '../components/settings/SystemInfoSection';
import { UpdateSection } from '../components/settings/UpdateSection';
import { PageToolbar, Button, Badge } from '../components/ui';
import { getErrorMessage } from '../utils/errors';

const providerPrivacy: Record<string, { label: string; local: boolean }> = {
  ollama: { label: 'Ollama', local: true },
  openai_compatible: { label: 'Unsloth / vLLM', local: true },
  llamacpp: { label: 'llama.cpp', local: true },
  openai: { label: 'OpenAI', local: false },
  gemini: { label: 'Gemini', local: false },
  anthropic: { label: 'Claude', local: false },
};

const describeProvider = (type: string) => providerPrivacy[type] ?? { label: type, local: false };

export const SettingsView: React.FC = () => {
  const { t } = useTranslation();

  // App settings info from backend
  const [info, setInfo] = useState<main.AppSettingsInfo | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);

  // Operation states
  const [vacuumLoading, setVacuumLoading] = useState<boolean>(false);
  const [vacuumSuccess, setVacuumSuccess] = useState<boolean>(false);
  const [openDirError, setOpenDirError] = useState<string | null>(null);
  const [copiedPath, setCopiedPath] = useState<string | null>(null);

  // AI Modal & Memory
  const [isAIModalOpen, setIsAIModalOpen] = useState<boolean>(false);
  const [showClearConfirm, setShowClearConfirm] = useState<boolean>(false);
  const [clearingFacts, setClearingFacts] = useState<boolean>(false);
  const [clearSuccess, setClearSuccess] = useState<boolean>(false);

  // Vault Master Password & Lock States
  const [vaultStatus, setVaultStatus] = useState<main.VaultStatusInfo | null>(null);
  const [isMasterPassModalOpen, setIsMasterPassModalOpen] = useState(false);
  const [isUnlockModalOpen, setIsUnlockModalOpen] = useState(false);
  const [masterPasswordInput, setMasterPasswordInput] = useState('');
  const [masterPasswordConfirm, setMasterPasswordConfirm] = useState('');
  const [unlockPasswordInput, setUnlockPasswordInput] = useState('');
  const [vaultActionLoading, setVaultActionLoading] = useState(false);
  const [vaultActionError, setVaultActionError] = useState<string | null>(null);
  const [vaultActionSuccess, setVaultActionSuccess] = useState<string | null>(null);

  // Whole-database encryption container
  const [isDbEncModalOpen, setIsDbEncModalOpen] = useState(false);
  const [dbEncPassword, setDbEncPassword] = useState('');
  const [dbEncLoading, setDbEncLoading] = useState(false);
  const [dbEncError, setDbEncError] = useState<string | null>(null);
  const [dbEncDone, setDbEncDone] = useState(false);

  // Logging & Support Bundle
  const [appLogLevel, setAppLogLevel] = useState<string>(() => {
    return localStorage.getItem('tamestack_log_level') || 'INFO';
  });
  const [exportBundleLoading, setExportBundleLoading] = useState(false);
  const [exportBundlePath, setExportBundlePath] = useState<string | null>(null);
  const [exportBundleError, setExportBundleError] = useState<string | null>(null);

  // Vault auto-lock & actual AI provider privacy
  const [autoLockLoading, setAutoLockLoading] = useState(false);
  const [autoLockError, setAutoLockError] = useState<string | null>(null);
  const [aiDefaultProvider, setAiDefaultProvider] = useState<{ label: string; local: boolean } | null>(null);
  const [aiHasConfigs, setAiHasConfigs] = useState(false);

  const loadSettings = async () => {
    try {
      setRefreshing(true);
      const [appInfo, vStatus, aiGlobal, aiConfigs] = await Promise.all([
        GetAppSettings(),
        GetVaultStatus(),
        AIGetGlobalSettings(),
        AIListProviderConfigs(),
      ]);
      setInfo(appInfo);
      setVaultStatus(vStatus);
      setAiHasConfigs((aiConfigs || []).length > 0);
      const defaultType =
        aiGlobal?.['default_provider'] ||
        (aiConfigs || []).find((c) => c.is_default)?.provider_type ||
        '';
      setAiDefaultProvider(defaultType ? describeProvider(defaultType) : null);
    } catch (err) {
      console.error('Failed to load app settings:', err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleAutoLockChange = async (minutes: number) => {
    try {
      setAutoLockLoading(true);
      setAutoLockError(null);
      await SetAutoLockMinutes(minutes);
      await loadSettings();
    } catch (err: unknown) {
      setAutoLockError(getErrorMessage(err, t('settings.autoLockError')));
    } finally {
      setAutoLockLoading(false);
    }
  };

  const handleSetMasterPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (masterPasswordInput.length < 8) {
      setVaultActionError(t('settings.passMinLength'));
      return;
    }
    if (masterPasswordInput !== masterPasswordConfirm) {
      setVaultActionError(t('settings.passMismatch'));
      return;
    }
    try {
      setVaultActionLoading(true);
      setVaultActionError(null);
      await SetMasterPassword(masterPasswordInput);
      setVaultActionSuccess(t('settings.masterPassConfigured'));
      setIsMasterPassModalOpen(false);
      setMasterPasswordInput('');
      setMasterPasswordConfirm('');
      await loadSettings();
      setTimeout(() => setVaultActionSuccess(null), 4000);
    } catch (err: unknown) {
      setVaultActionError(getErrorMessage(err, t('settings.masterPassError')));
    } finally {
      setVaultActionLoading(false);
    }
  };

  const handleEnableDbEncryption = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dbEncPassword) {
      return;
    }
    try {
      setDbEncLoading(true);
      setDbEncError(null);
      await EnableDatabaseEncryption(dbEncPassword);
      setIsDbEncModalOpen(false);
      setDbEncPassword('');
      setDbEncDone(true);
      await loadSettings();
    } catch (err: unknown) {
      setDbEncError(getErrorMessage(err, t('settings.dbEncError')));
    } finally {
      setDbEncLoading(false);
    }
  };

  const handleUnlockVault = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setVaultActionLoading(true);
      setVaultActionError(null);
      await UnlockVault(unlockPasswordInput);
      setIsUnlockModalOpen(false);
      setUnlockPasswordInput('');
      setVaultActionSuccess(t('settings.vaultUnlockedSuccess'));
      await loadSettings();
      setTimeout(() => setVaultActionSuccess(null), 4000);
    } catch (err: unknown) {
      setVaultActionError(getErrorMessage(err, t('settings.invalidMasterPass')));
    } finally {
      setVaultActionLoading(false);
    }
  };

  const handleLockVault = async () => {
    try {
      await LockVault();
      await loadSettings();
    } catch (err) {
      console.error('Failed to lock vault:', err);
    }
  };

  const handleLogLevelChange = async (level: string) => {
    setAppLogLevel(level);
    localStorage.setItem('tamestack_log_level', level);
    try {
      await SetAppLogLevel(level);
    } catch (err) {
      console.error('Failed to set log level:', err);
    }
  };

  const handleExportSupportBundle = async () => {
    try {
      setExportBundleLoading(true);
      setExportBundleError(null);
      setExportBundlePath(null);
      const path = await ExportSupportBundle();
      setExportBundlePath(path);
    } catch (err: unknown) {
      setExportBundleError(getErrorMessage(err, t('settings.bundleError')));
    } finally {
      setExportBundleLoading(false);
    }
  };

  useEffect(() => {
    loadSettings();
    // The backend logger starts at its default level on every launch; re-apply the
    // stored preference so the selector and the actual log output agree.
    SetAppLogLevel(appLogLevel).catch((err) => {
      console.error('Failed to apply stored log level:', err);
    });

    const unVaultUnlocked = EventsOn('vault:unlocked', () => {
      loadSettings();
    });
    const unVaultLocked = EventsOn('vault:locked', () => {
      loadSettings();
    });

    return () => {
      if (typeof unVaultUnlocked === 'function') unVaultUnlocked();
      else EventsOff('vault:unlocked');
      if (typeof unVaultLocked === 'function') unVaultLocked();
      else EventsOff('vault:locked');
    };
  }, []);

  const handleOpenDir = async () => {
    try {
      setOpenDirError(null);
      await OpenDataDirectory();
    } catch (err: unknown) {
      setOpenDirError(getErrorMessage(err, t('settings.openDirError')));
    }
  };

  const handleVacuum = async () => {
    try {
      setVacuumLoading(true);
      setVacuumSuccess(false);
      await VacuumDatabase();
      setVacuumSuccess(true);
      await loadSettings();
      setTimeout(() => setVacuumSuccess(false), 3000);
    } catch (err) {
      console.error('Vacuum failed:', err);
    } finally {
      setVacuumLoading(false);
    }
  };

  const handleClearMemoryFacts = async () => {
    try {
      setClearingFacts(true);
      setClearSuccess(false);
      await AIClearFacts('');
      setShowClearConfirm(false);
      setClearSuccess(true);
      await loadSettings();
      setTimeout(() => setClearSuccess(false), 3000);
    } catch (err) {
      console.error('Failed to clear facts:', err);
    } finally {
      setClearingFacts(false);
    }
  };

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedPath(key);
    setTimeout(() => setCopiedPath(null), 2000);
  };

  return (
    <div className="page space-y-6 pb-16">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Sliders className="w-5 h-5" />}
        title={t('settings.systemConfig')}
        subtitle={t('settings.systemConfigSubtitle')}
        actions={
          <Button
            variant="secondary"
            size="md"
            onClick={loadSettings}
            disabled={refreshing}
            icon={<RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-indigo-400' : ''}`} />}
          >
            {t('common.refresh')}
          </Button>
        }
      />

      {/* Section quick navigation */}
      <div className="sticky top-0 z-20 -mx-1 px-1 py-2 bg-surface-900/95 backdrop-blur border-b border-slate-800/60 flex flex-wrap gap-1.5">
        {[
          { id: 'settings-language', label: t('settings.navLanguage') },
          { id: 'settings-ui', label: t('settings.navUi') },
          { id: 'settings-ai', label: t('settings.navAi') },
          { id: 'settings-storage', label: t('settings.navStorage') },
          { id: 'settings-security', label: t('settings.navSecurity') },
          { id: 'settings-system', label: t('settings.navSystem') },
          { id: 'settings-update', label: t('settings.navUpdate') },
        ].map((item) => (
          <button
            key={item.id}
            onClick={() => document.getElementById(item.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })}
            className="px-2.5 py-1 rounded-lg text-[11px] font-semibold text-slate-400 hover:text-white bg-slate-800/70 hover:bg-slate-700 border border-slate-700/50 transition cursor-pointer"
          >
            {item.label}
          </button>
        ))}
      </div>

      {/* 1. Language & Localization */}
      <LanguageSection />

      {/* 2. UI & Terminal Preferences */}
      <UiTerminalSection />

      {/* 3. AI Subsystem & Knowledge Hub */}
      <div id="settings-ai" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-purple-500/10 text-purple-400 border border-purple-500/20">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-white">
                {t('settings.aiSubsystem')}
              </h3>
              <p className="text-xs text-slate-400">
                {t('settings.aiSubsystemSubtitle')}
              </p>
            </div>
          </div>

          <Button
            variant="primary"
            size="sm"
            onClick={() => setIsAIModalOpen(true)}
            icon={<Sparkles className="w-3.5 h-3.5" />}
          >
            {t('settings.configureAI')}
          </Button>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4 text-xs font-mono">
          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.supportedEngines')}
            </span>
            <span className="text-purple-300 font-semibold">
              Ollama, Unsloth, llama.cpp, OpenAI, Gemini, Claude
            </span>
          </div>

          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.privacyMode')}
            </span>
            {aiDefaultProvider ? (
              <span className={`font-semibold ${aiDefaultProvider.local ? 'text-emerald-400' : 'text-amber-400'}`}>
                {aiDefaultProvider.local
                  ? t('settings.privacyModeLocal')
                  : t('settings.privacyCloudValue', { provider: aiDefaultProvider.label })}
              </span>
            ) : (
              <span className="text-slate-400 font-semibold">
                {aiHasConfigs ? t('settings.privacyNoDefault') : t('settings.privacyNotConfigured')}
              </span>
            )}
          </div>

          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 flex items-center justify-between">
            <div>
              <span className="text-slate-500 block text-[11px]">
                {t('settings.memoryFacts')}
              </span>
              <span className="text-white font-semibold">
                {t('settings.factsCount', { count: info?.ai_memory_fact_count ?? 0 })}
              </span>
            </div>
            <button
              onClick={() => setShowClearConfirm(true)}
              disabled={clearingFacts || (info?.ai_memory_fact_count ?? 0) === 0}
              className="px-2.5 py-1 bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 rounded text-[11px] font-semibold transition disabled:opacity-40"
              title={t('settings.clearFactsTooltip')}
            >
              {t('common.delete')}
            </button>
          </div>
        </div>

        {clearSuccess && (
          <div className="p-2.5 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 rounded-lg text-xs flex items-center gap-2">
            <CheckCircle className="w-4 h-4" />
            {t('settings.clearFactsSuccess')}
          </div>
        )}
      </div>

      {/* 4. Local Storage & Database Maintenance */}
      <div id="settings-storage" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
              <Database className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-white">
                {t('settings.localStorage')}
              </h3>
              <p className="text-xs text-slate-400">
                {t('settings.localStorageSubtitle')}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleVacuum}
              disabled={vacuumLoading}
              icon={<HardDrive className={`w-3.5 h-3.5 ${vacuumLoading ? 'animate-spin' : ''}`} />}
              title={t('settings.vacuumTooltip')}
            >
              {vacuumLoading ? t('settings.vacuuming') : t('settings.vacuumBtn')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleOpenDir}
              icon={<FolderOpen className="w-3.5 h-3.5" />}
            >
              {t('settings.openDataDir')}
            </Button>
          </div>
        </div>

        {vacuumSuccess && (
          <div className="p-2.5 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 rounded-lg text-xs flex items-center gap-2">
            <CheckCircle className="w-4 h-4" />
            {t('settings.vacuumSuccess')}
          </div>
        )}

        {openDirError && (
          <div className="p-2.5 bg-red-500/10 border border-red-500/20 text-red-400 rounded-lg text-xs flex items-center gap-2">
            <AlertTriangle className="w-4 h-4" />
            {openDirError}
          </div>
        )}

        <div className="space-y-2.5 text-xs font-mono">
          {/* Data Dir */}
          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <span className="text-slate-500 block text-[11px]">
                {t('settings.appDataDir')}
              </span>
              <span className="text-slate-200 text-xs break-all select-all font-mono">
                {info?.data_dir || t('common.loadingShort')}
              </span>
            </div>
            {info?.data_dir && (
              <button
                onClick={() => handleCopy(info.data_dir, 'datadir')}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white rounded transition flex-shrink-0"
                title={t('settings.copyPath')}
              >
                {copiedPath === 'datadir' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            )}
          </div>

          {/* Database Path & Size */}
          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <span className="text-slate-500 block text-[11px]">
                {t('settings.dbFile')}
              </span>
              <span className="text-slate-200 text-xs break-all select-all font-mono">
                {info?.database_path || t('common.loadingShort')}
              </span>
            </div>
            <div className="flex items-center gap-2 flex-shrink-0">
              <span className="px-2 py-0.5 rounded bg-indigo-500/20 text-indigo-300 text-[10px] font-bold">
                {t('settings.walModeBadge')}
              </span>
              <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 text-[11px] font-semibold">
                {info?.database_size_human || '0 B'}
              </span>
            </div>
          </div>

          {/* Log File Path & Size */}
          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <span className="text-slate-500 block text-[11px]">
                {t('settings.logFile')}
              </span>
              <span className="text-slate-200 text-xs break-all select-all font-mono">
                {info?.log_path || t('common.loadingShort')}
              </span>
            </div>
            <div className="flex items-center gap-2 flex-shrink-0">
              <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 text-[11px] font-semibold">
                {info?.log_size_human || '0 B'}
              </span>
            </div>
          </div>
        </div>

        {/* Structured Logging Level & Support Bundle */}
        <div className="pt-2 border-t border-slate-800/80 flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-2">
            <span className="text-slate-400 font-medium">
              {t('settings.logLevel')}
            </span>
            <div className="flex bg-slate-900 p-0.5 rounded-lg border border-slate-800">
              {['DEBUG', 'INFO', 'WARN', 'ERROR'].map((lvl) => (
                <button
                  key={lvl}
                  onClick={() => handleLogLevelChange(lvl)}
                  className={`px-2.5 py-1 rounded-lg text-[11px] font-semibold transition ${
                    appLogLevel === lvl
                      ? 'bg-indigo-600 text-white shadow'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  {lvl}
                </button>
              ))}
            </div>
          </div>

          <Button
            variant="secondary"
            size="sm"
            onClick={handleExportSupportBundle}
            disabled={exportBundleLoading}
            icon={<FileArchive className="w-3.5 h-3.5 text-indigo-400" />}
          >
            {exportBundleLoading ? t('settings.exportingBundle') : t('settings.exportBundle')}
          </Button>
        </div>

        {exportBundlePath && (
          <div className="p-3 bg-emerald-950/40 border border-emerald-500/30 rounded-lg text-xs text-emerald-300 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <CheckCircle className="w-4 h-4 text-emerald-400 flex-shrink-0" />
              <span>
                {t('settings.bundleSaved')} 
                <strong className="font-mono text-emerald-200">{exportBundlePath}</strong>
              </span>
            </div>
          </div>
        )}

        {exportBundleError && (
          <div className="p-3 bg-rose-950/40 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0" />
            <span>{exportBundleError}</span>
          </div>
        )}
      </div>

      {/* 5. Security & Vault Status */}
      <div id="settings-security" className="scroll-mt-14 bg-surface-850 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className={`p-2.5 rounded-xl border ${
              vaultStatus?.is_locked
                ? 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
            }`}>
              {vaultStatus?.is_locked ? <Lock className="w-5 h-5" /> : <Shield className="w-5 h-5" />}
            </div>
            <div>
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                {t('settings.vaultAndZeroTrust')}
                {vaultStatus?.has_master_password ? (
                  vaultStatus.is_locked ? (
                    <Badge variant="danger" size="xs">
                      {t('settings.lockedBadge')}
                    </Badge>
                  ) : (
                    <Badge variant="success" size="xs">
                      {t('settings.masterPassActiveBadge')}
                    </Badge>
                  )
                ) : (
                  <Badge variant="indigo" size="xs">
                    {t('settings.machineKeyBadge')}
                  </Badge>
                )}
              </h3>
              <p className="text-xs text-slate-400">
                {t('settings.vaultSecuritySubtitle')}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {vaultStatus?.has_master_password ? (
              vaultStatus.is_locked ? (
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => {
                    setVaultActionError(null);
                    setUnlockPasswordInput('');
                    setIsUnlockModalOpen(true);
                  }}
                  icon={<Unlock className="w-3.5 h-3.5" />}
                >
                  {t('settings.unlock')}
                </Button>
              ) : (
                <>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleLockVault}
                    icon={<Lock className="w-3.5 h-3.5 text-amber-400" />}
                  >
                    {t('settings.lockNow')}
                  </Button>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => {
                      setVaultActionError(null);
                      setMasterPasswordInput('');
                      setMasterPasswordConfirm('');
                      setIsMasterPassModalOpen(true);
                    }}
                    icon={<KeyRound className="w-3.5 h-3.5" />}
                  >
                    {t('settings.changeMasterPassword')}
                  </Button>
                </>
              )
            ) : (
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  setVaultActionError(null);
                  setMasterPasswordInput('');
                  setMasterPasswordConfirm('');
                  setIsMasterPassModalOpen(true);
                }}
                icon={<KeyRound className="w-3.5 h-3.5" />}
              >
                {t('settings.setupMasterPassword')}
              </Button>
            )}
          </div>
        </div>

        {vaultActionSuccess && (
          <div className="p-3 bg-emerald-950/40 border border-emerald-500/30 rounded-lg text-xs text-emerald-300 flex items-center gap-2">
            <CheckCircle className="w-4 h-4 text-emerald-400 flex-shrink-0" />
            <span>{vaultActionSuccess}</span>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.cipher')}
            </span>
            <span className="text-emerald-400 font-semibold flex items-center gap-1.5">
              <Lock className="w-3.5 h-3.5 text-emerald-400" />
              {t('settings.cipherValue')}
            </span>
          </div>

          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.kdf')}
            </span>
            <span className="text-indigo-400 font-semibold">
              {vaultStatus?.has_master_password
                ? t('settings.kdfMasterValue')
                : t('settings.kdfMachineValue')}
            </span>
          </div>

          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.sshHostKeyChecking')}
            </span>
            <div className="flex items-center justify-between">
              <span className="text-emerald-400 font-semibold">
                {t('settings.strictKnownHostsValue')}
              </span>
              <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 text-[10px] font-bold border border-emerald-500/20">
                {t('settings.entriesCount', { count: info?.known_hosts_count ?? 0 })}
              </span>
            </div>
          </div>

          <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-1">
            <span className="text-slate-500 block text-[11px]">
              {t('settings.logScrubber')}
            </span>
            <span className="text-emerald-400 font-semibold">
              {t('settings.logScrubberValue')}
            </span>
          </div>
        </div>

        {/* Vault auto-lock timeout */}
        <div className="pt-2 border-t border-slate-800/80 flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-2.5">
            <div className="p-2 rounded-lg bg-slate-800/80 border border-slate-700 text-slate-300">
              <Clock className="w-4 h-4" />
            </div>
            <div>
              <span className="block text-xs font-semibold text-slate-200">
                {t('settings.autoLock')}
              </span>
              <span className="block text-[11px] text-slate-400">
                {t('settings.autoLockDesc')}
              </span>
            </div>
          </div>
          {vaultStatus?.has_master_password ? (
            <div className="flex bg-slate-900 p-0.5 rounded-lg border border-slate-800">
              {[0, 5, 15, 30, 60].map((mins) => (
                <button
                  key={mins}
                  onClick={() => handleAutoLockChange(mins)}
                  disabled={autoLockLoading}
                  className={`px-2.5 py-1 rounded-lg text-[11px] font-semibold transition disabled:opacity-50 ${
                    (vaultStatus?.auto_lock_minutes ?? 0) === mins
                      ? 'bg-indigo-600 text-white shadow'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  {mins === 0 ? t('settings.autoLockOff') : t('settings.autoLockMinutes', { count: mins })}
                </button>
              ))}
            </div>
          ) : (
            <span className="text-[11px] text-slate-500 max-w-[16rem] text-right leading-snug">
              {t('settings.autoLockNeedsMP')}
            </span>
          )}
        </div>

        {autoLockError && (
          <div className="p-3 bg-rose-950/40 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
            <span>{autoLockError}</span>
          </div>
        )}

        {/* Whole-database encryption container */}
        <div className="pt-2 border-t border-slate-800/80">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-2.5">
              <div className={`p-2 rounded-lg border ${
                info?.db_encrypted
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                  : 'bg-amber-500/10 text-amber-400 border-amber-500/20'
              }`}>
                <Database className="w-4 h-4" />
              </div>
              <div>
                <span className="block text-xs font-semibold text-slate-200">
                  {t('settings.dbEncTitle')}
                </span>
                <span className="block text-[11px] text-slate-400">
                  {info?.db_encrypted ? t('settings.dbEncActiveDesc') : t('settings.dbEncInactiveDesc')}
                </span>
              </div>
            </div>
            {!info?.db_encrypted && !dbEncDone && (
              vaultStatus?.has_master_password ? (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    setDbEncError(null);
                    setDbEncPassword('');
                    setIsDbEncModalOpen(true);
                  }}
                  icon={<Lock className="w-3.5 h-3.5" />}
                >
                  {t('settings.dbEncEnable')}
                </Button>
              ) : (
                <span className="text-[11px] text-slate-400 max-w-[16rem] text-right leading-snug">
                  {t('settings.dbEncNeedsMP')}
                </span>
              )
            )}
            {info?.db_encrypted && (
              <Badge variant="success" size="xs">
                {t('settings.dbEncActiveBadge')}
              </Badge>
            )}
          </div>

          {dbEncDone && !info?.db_encrypted && (
            <div className="mt-3 p-3 bg-amber-950/40 border border-amber-500/30 rounded-lg text-xs text-amber-300 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
              <span>{t('settings.dbEncRestartNotice')}</span>
            </div>
          )}
          {dbEncError && (
            <div className="mt-3 p-3 bg-rose-950/40 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
              <span>{dbEncError}</span>
            </div>
          )}
        </div>
      </div>

      {/* 6. System & Architecture Information */}
      <SystemInfoSection info={info} />

      {/* 7. Application Update */}
      <UpdateSection />

      {/* Confirmation Modal for Clearing AI Memory */}
      {showClearConfirm && (
        <ClearFactsConfirmModal
          onClose={() => setShowClearConfirm(false)}
          clearingFacts={clearingFacts}
          onConfirm={handleClearMemoryFacts}
        />
      )}

      {/* AI Settings Modal */}
      <AISettingsModal
        isOpen={isAIModalOpen}
        onClose={() => setIsAIModalOpen(false)}
        onSaved={loadSettings}
      />

      {/* Enable Whole-Database Encryption Modal */}
      {isDbEncModalOpen && (
        <DbEncryptionModal
          onClose={() => setIsDbEncModalOpen(false)}
          dbEncPassword={dbEncPassword}
          setDbEncPassword={setDbEncPassword}
          dbEncLoading={dbEncLoading}
          dbEncError={dbEncError}
          onSubmit={handleEnableDbEncryption}
        />
      )}

      {/* Set / Change Master Password Modal */}
      {isMasterPassModalOpen && (
        <MasterPasswordModal
          onClose={() => setIsMasterPassModalOpen(false)}
          hasMasterPassword={!!vaultStatus?.has_master_password}
          masterPasswordInput={masterPasswordInput}
          setMasterPasswordInput={setMasterPasswordInput}
          masterPasswordConfirm={masterPasswordConfirm}
          setMasterPasswordConfirm={setMasterPasswordConfirm}
          vaultActionLoading={vaultActionLoading}
          vaultActionError={vaultActionError}
          onSubmit={handleSetMasterPassword}
        />
      )}

      {/* Unlock Vault Modal */}
      {isUnlockModalOpen && (
        <UnlockVaultModal
          onClose={() => setIsUnlockModalOpen(false)}
          unlockPasswordInput={unlockPasswordInput}
          setUnlockPasswordInput={setUnlockPasswordInput}
          vaultActionLoading={vaultActionLoading}
          vaultActionError={vaultActionError}
          onSubmit={handleUnlockVault}
        />
      )}
    </div>
  );
};
