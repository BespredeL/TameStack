import React, { useState, useEffect } from 'react';
import {
  Wrench,
  Server,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  Loader2,
  Shield,
  Database,
  Activity,
  ArrowRight,
  ArrowLeft,
  Check,
  Zap,
  Bookmark,
  Trash2,
  Globe,
  Code2,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import {
  ProvisioningDiscover,
  ProvisioningListProfiles,
  ProvisioningGetProfileBlueprint,
  ProvisioningPlan,
  ProvisioningExecute,
  ProvisioningListBlueprints,
  ProvisioningSaveBlueprint,
  ProvisioningDeleteBlueprint,
} from '../../wailsjs/go/main/App';
import { inventory, profiles, models, db } from '../../wailsjs/go/models';
import { getErrorMessage } from '../utils/errors';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';
import { useConfirm } from '../context/ConfirmContext';
import { useAlert } from '../context/AlertContext';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { Button, Select, PageToolbar } from '../components/ui';
import { ModalShell } from '../components/common/ModalShell';

const localizeWarning = (w: string, t: any): string => {
  const safeAdditiveMatch = w.match(/^Обнаружен уже настроенный сервер \((.*?)\)\. Мастер активировал безопасный режим дополнения/);
  if (safeAdditiveMatch) {
    return t('provisioning.planWarnings.safeAdditive', { details: safeAdditiveMatch[1] });
  }
  const lowRamMatch = w.match(/^Низкий объём оперативной памяти \((\d+) МБ\)/);
  if (lowRamMatch) {
    return t('provisioning.planWarnings.lowRam', { mb: lowRamMatch[1] });
  }
  const ramWarnMatch = w.match(/^Объём оперативной памяти (\d+) МБ может быть недостаточен/);
  if (ramWarnMatch) {
    return t('provisioning.planWarnings.ramWarning', { mb: ramWarnMatch[1] });
  }
  const lowDiskMatch = w.match(/^Мало свободного места на диске \((\d+) ГБ\)/);
  if (lowDiskMatch) {
    return t('provisioning.planWarnings.lowDisk', { gb: lowDiskMatch[1] });
  }
  return w;
};

const localizePlanItemDescription = (desc: string, t: any): string => {
  if (desc === 'Установка Nginx веб-сервера') return t('provisioning.stepsDesc.installNginx');
  if (desc === 'Установка Apache HTTP Server') return t('provisioning.stepsDesc.installApache');
  const phpMatch = desc.match(/^PHP ([\d\.]+) и (\d+) расширений/);
  if (phpMatch) {
    return t('provisioning.stepsDesc.phpWithExtensions', { ver: phpMatch[1], count: phpMatch[2] });
  }
  if (desc === 'Установка СУБД MariaDB Server') return t('provisioning.stepsDesc.installMariaDb');
  if (desc === 'Установка СУБД MySQL Server') return t('provisioning.stepsDesc.installMySql');
  if (desc === 'Установка СУБД PostgreSQL') return t('provisioning.stepsDesc.installPostgres');
  if (desc === 'Установка In-Memory Cache Redis') return t('provisioning.stepsDesc.installRedis');
  if (desc === 'Система контроля версий Git') return t('provisioning.stepsDesc.git');
  if (desc === 'Пакетный менеджер PHP Composer') return t('provisioning.stepsDesc.composer');
  if (desc === 'Node.js & npm runtime') return t('provisioning.stepsDesc.nodejs');
  if (desc === 'Docker Engine и Docker Compose') return t('provisioning.stepsDesc.docker');
  if (desc === 'Обязательное разрешение порта SSH перед включением брандмауэра') return t('provisioning.stepsDesc.sshAllowUfw');
  if (desc === 'UFW брандмауэр') return t('provisioning.stepsDesc.ufw');
  if (desc === 'Обязательное разрешение порта SSH') return t('provisioning.stepsDesc.sshAllowFirewalld');
  if (desc === 'Firewalld брандмауэр') return t('provisioning.stepsDesc.firewalld');
  if (desc === 'Защита от подбора паролей Fail2ban') return t('provisioning.stepsDesc.fail2ban');
  if (desc === 'Системный пользователь для веб-сервера и сайтов') return t('provisioning.stepsDesc.systemUser');
  if (desc === 'Корневой каталог для веб-сайтов с правами 0755') return t('provisioning.stepsDesc.webRoot');
  return desc;
};

interface ProvisioningViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const ProvisioningView: React.FC<ProvisioningViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const { selectedServerId: selectedServerID, setSelectedServerId: setSelectedServerID } = useServer();
  const { t } = useTranslation();
  const confirm = useConfirm();
  const alert = useAlert();

  const [step, setStep] = useState<number>(1);
  const [inventoryData, setInventoryData] = useState<inventory.ServerInventory | null>(null);
  const [isDiscovering, setIsDiscovering] = useState(false);
  const [discoveryError, setDiscoveryError] = useState<string | null>(null);

  const [profilesList, setProfilesList] = useState<profiles.ProfileInfo[]>([]);
  const [blueprintsList, setBlueprintsList] = useState<db.ServerBlueprintRecord[]>([]);
  const [selectedProfileId, setSelectedProfileId] = useState<string>('minimal_php');

  // Blueprint state
  const [blueprint, setBlueprint] = useState<models.Blueprint | null>(null);

  // Plan state
  const [plan, setPlan] = useState<models.ProvisioningPlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);

  // Execution state
  const [isExecuting, setIsExecuting] = useState(false);
  const [activeTask, setActiveTask] = useState<TaskRecord | null>(null);

  // Save blueprint modal
  const [saveName, setSaveName] = useState('');
  const [saveDesc, setSaveDesc] = useState('');
  const [isSavingBp, setIsSavingBp] = useState(false);
  const [showSaveModal, setShowSaveModal] = useState(false);

  const selectedServer = servers.find((s) => s.id === selectedServerID) || servers[0];

  // Discover server
  const runDiscovery = async () => {
    if (!selectedServer) return;
    setIsDiscovering(true);
    setDiscoveryError(null);
    try {
      const inv = await ProvisioningDiscover(selectedServer.id);
      setInventoryData(inv);
    } catch (err: unknown) {
      setDiscoveryError(getErrorMessage(err));
    } finally {
      setIsDiscovering(false);
    }
  };

  useEffect(() => {
    if (selectedServer) {
      runDiscovery();
    }
  }, [selectedServer?.id]);

  // Load profiles & blueprints
  useEffect(() => {
    ProvisioningListProfiles()
      .then((p) => setProfilesList(p || []))
      .catch(console.error);

    loadBlueprints();
  }, []);

  const loadBlueprints = async () => {
    try {
      const bp = await ProvisioningListBlueprints();
      setBlueprintsList(bp || []);
    } catch (err) {
      console.error(err);
    }
  };

  const handleSelectProfile = async (profileId: string) => {
    setSelectedProfileId(profileId);
    if (!selectedServer || !inventoryData) return;
    try {
      const bp = await ProvisioningGetProfileBlueprint(profileId, inventoryData.family);
      setBlueprint(bp);
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    }
  };

  const handleLoadSavedBlueprint = (record: db.ServerBlueprintRecord) => {
    try {
      const parsed = JSON.parse(record.spec_json);
      setBlueprint(models.Blueprint.createFrom(parsed));
      setSelectedProfileId(parsed.profile || 'custom');
      setStep(3);
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    }
  };

  const handleDeleteBlueprint = async (id: string, name: string) => {
    if (!(await confirm(t('provisioning.confirmDeleteBlueprint', { name })))) return;
    try {
      await ProvisioningDeleteBlueprint(id);
      await loadBlueprints();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    }
  };

  const handleGeneratePlan = async () => {
    if (!selectedServer || !blueprint) return;
    setIsPlanning(true);
    setPlanError(null);
    try {
      const p = await ProvisioningPlan(selectedServer.id, blueprint);
      setPlan(p);
      setStep(4);
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsPlanning(false);
    }
  };

  const handleExecute = async () => {
    if (!selectedServer || !blueprint) return;
    if (!(await confirm(t('provisioning.confirmExecute', { name: selectedServer.name })))) {
      return;
    }
    setIsExecuting(true);
    try {
      const task = await ProvisioningExecute(selectedServer.id, blueprint);
      setActiveTask(task as TaskRecord);
      if (onOpenTask) onOpenTask(task as TaskRecord);
      setStep(5);
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setIsExecuting(false);
    }
  };

  const handleSaveBlueprint = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!saveName.trim() || !blueprint) return;
    setIsSavingBp(true);
    try {
      await ProvisioningSaveBlueprint(saveName.trim(), saveDesc.trim(), blueprint);
      setShowSaveModal(false);
      setSaveName('');
      setSaveDesc('');
      await loadBlueprints();
      alert(t('provisioning.blueprintSaved'));
    } catch (err: unknown) {
      alert(getErrorMessage(err));
    } finally {
      setIsSavingBp(false);
    }
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Wrench}
        description={t('empty.provisioningDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="page space-y-6">
      {/* Top Toolbar */}
      <PageToolbar
        icon={<Wrench className="w-5 h-5 text-indigo-400" />}
        servers={servers}
        selectedServerId={selectedServerID}
        onSelectServer={setSelectedServerID}
        serverLabel={t('provisioning.targetServer')}
      />

      {/* Stepper Bar */}
      <div className="bg-surface-850 p-4 rounded-2xl border border-slate-800 shadow-sm">
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-2 text-xs">
          {[
            { num: 1, label: t('provisioning.stepDiscovery') },
            { num: 2, label: t('provisioning.stepProfiles') },
            { num: 3, label: t('provisioning.stepCustomize') },
            { num: 4, label: t('provisioning.stepPlan') },
            { num: 5, label: t('provisioning.stepExecute') },
          ].map((s) => (
            <div
              key={s.num}
              onClick={() => {
                if (s.num <= step || (s.num === 3 && blueprint)) setStep(s.num);
              }}
              className={`p-2.5 rounded-lg border text-center transition cursor-pointer ${
                step === s.num
                  ? 'bg-indigo-600/20 border-indigo-500 text-indigo-300 font-bold'
                  : step > s.num
                  ? 'bg-slate-900/60 border-slate-800 text-emerald-400'
                  : 'bg-slate-900/30 border-slate-800/60 text-slate-500'
              }`}
            >
              <div className="flex items-center justify-center gap-1.5">
                {step > s.num ? <Check className="w-3.5 h-3.5" /> : null}
                <span>{s.label}</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* STEP 1: DISCOVERY */}
      {step === 1 && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-bold text-white uppercase tracking-wider flex items-center gap-2">
              <Activity className="w-4 h-4 text-indigo-400" />
              <span>{t('provisioning.serverFactsTitle')}</span>
            </h4>
            <button
              onClick={runDiscovery}
              disabled={isDiscovering}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin' : ''}`} />
              <span>{t('common.refresh')}</span>
            </button>
          </div>

          {discoveryError && (
            <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-3">
              <AlertTriangle className="w-5 h-5 shrink-0" />
              <span>{discoveryError}</span>
            </div>
          )}

          {isDiscovering ? (
            <div className="flex flex-col items-center justify-center p-16 bg-surface-850 rounded-xl border border-slate-800">
              <Loader2 className="w-8 h-8 animate-spin text-indigo-400 mb-3" />
              <p className="text-xs text-slate-300 font-medium">
                {t('provisioning.discoveringWait')}
              </p>
            </div>
          ) : inventoryData ? (
            <div className="space-y-6">
              {/* Configured Server Detected - Non-destructive Mode Banner */}
              {inventoryData.is_already_configured && (
                <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-200 text-xs flex items-start gap-3">
                  <Shield className="w-5 h-5 shrink-0 text-emerald-400 mt-0.5" />
                  <div>
                    <div className="font-bold text-white flex items-center gap-2">
                      <span>{t('provisioning.alreadyConfiguredBadge')}</span>
                      {(inventoryData.existing_sites_count || 0) > 0 && (
                        <span className="px-2 py-0.5 rounded-full text-[10px] bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                          {inventoryData.existing_sites_count} {t('provisioning.existingSitesCount')}
                        </span>
                      )}
                    </div>
                    <p className="text-slate-300 text-[11px] mt-1 leading-relaxed">
                      {t('provisioning.alreadyConfiguredNotice')}
                    </p>
                  </div>
                </div>
              )}

              {/* Hardware & OS Overview Cards */}
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
                  <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.osAndDistro')}</span>
                  <div className="font-bold text-white text-sm mt-1">{inventoryData.distro} {inventoryData.version}</div>
                  <div className="text-[11px] text-slate-500 font-mono mt-0.5">{inventoryData.kernel} ({inventoryData.arch})</div>
                </div>

                <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
                  <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.cpuCores')}</span>
                  <div className="font-bold text-white text-sm mt-1">{inventoryData.cpu_cores} vCPU</div>
                  <div className="text-[11px] text-slate-500 truncate mt-0.5">{t('provisioning.archLabel', { arch: inventoryData.arch })}</div>
                </div>

                <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
                  <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.ramLabel')}</span>
                  <div className="font-bold text-white text-sm mt-1">{t('provisioning.gbTotal', { size: (inventoryData.ram_total_mb / 1024).toFixed(1) })}</div>
                  <div className="text-[11px] text-slate-500 font-mono mt-0.5">{t('provisioning.gbFree', { size: (inventoryData.ram_free_mb / 1024).toFixed(1) })}</div>
                </div>

                <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
                  <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.primaryStorage')}</span>
                  <div className="font-bold text-white text-sm mt-1">{t('provisioning.gbTotal', { size: inventoryData.disk_total_gb })}</div>
                  <div className="text-[11px] text-slate-500 font-mono mt-0.5">{t('provisioning.gbAvailable', { size: inventoryData.disk_free_gb })}</div>
                </div>
              </div>

              {/* Listening Ports & Existing Software */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-3">
                  <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                    <Shield className="w-4 h-4" />
                    <span>{t('provisioning.detectedSoftware')}</span>
                  </h5>
                  <div className="space-y-2 text-xs">
                    <div className="flex items-center justify-between py-1.5 border-b border-slate-800/60">
                      <span className="text-slate-400">{t('provisioning.webServers')}</span>
                      <span className="text-white font-mono font-semibold">
                        {inventoryData.installed_web && inventoryData.installed_web.length > 0
                          ? inventoryData.installed_web.join(', ')
                          : t('provisioning.noneDetected')}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-1.5 border-b border-slate-800/60">
                      <span className="text-slate-400">{t('provisioning.phpVersions')}</span>
                      <span className="text-white font-mono font-semibold">
                        {inventoryData.installed_php && inventoryData.installed_php.length > 0
                          ? inventoryData.installed_php.join(', ')
                          : t('provisioning.noneDetected')}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-1.5 border-b border-slate-800/60">
                      <span className="text-slate-400">{t('provisioning.databases')}</span>
                      <span className="text-white font-mono font-semibold">
                        {inventoryData.installed_db && inventoryData.installed_db.length > 0
                          ? inventoryData.installed_db.join(', ')
                          : t('provisioning.noneDetected')}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-1.5 border-b border-slate-800/60">
                      <span className="text-slate-400">{t('provisioning.redis')}</span>
                      <span className="text-white font-mono font-semibold">
                        {inventoryData.listening_ports?.includes(6379) ? t('provisioning.activeOnPort', { port: 6379 }) : t('provisioning.notDetected')}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-1.5 border-b border-slate-800/60">
                      <span className="text-slate-400">{t('provisioning.dockerEngine')}</span>
                      <span className="text-white font-mono font-semibold">{inventoryData.has_docker ? t('common.active') : t('common.notInstalled')}</span>
                    </div>
                    <div className="flex items-center justify-between py-1.5">
                      <span className="text-slate-400">{t('provisioning.firewall')}</span>
                      <span className="text-white font-mono font-semibold">{inventoryData.has_ufw || inventoryData.has_firewalld ? t('common.enabled') : t('common.disabled')}</span>
                    </div>
                  </div>
                </div>

                <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-3">
                  <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                    <Activity className="w-4 h-4" />
                    <span>{t('provisioning.listeningPorts')}</span>
                  </h5>
                  <div className="flex flex-wrap gap-2 pt-1">
                    {inventoryData.listening_ports && inventoryData.listening_ports.length > 0 ? (
                      inventoryData.listening_ports.map((p) => (
                        <span
                          key={p}
                          className={`px-2.5 py-1 rounded-lg text-xs font-mono font-bold border ${
                            p === 22 || p === inventoryData.ssh_port
                              ? 'bg-purple-500/20 text-purple-300 border-purple-500/30'
                              : p === 80 || p === 443
                              ? 'bg-amber-500/20 text-amber-300 border-amber-500/30'
                              : 'bg-slate-900 text-slate-300 border-slate-700'
                          }`}
                        >
                          Port {p} {p === inventoryData.ssh_port ? '(SSH)' : p === 80 ? '(HTTP)' : p === 443 ? '(HTTPS)' : ''}
                        </span>
                      ))
                    ) : (
                      <span className="text-xs text-slate-500">{t('provisioning.noListeningPorts')}</span>
                    )}
                  </div>
                  <p className="text-[11px] text-slate-400 pt-2">
                    {t('provisioning.sshPreserveNotice', { port: inventoryData.ssh_port })}
                  </p>
                </div>
              </div>

              {/* Next Button */}
              <div className="flex justify-end pt-4">
                <button
                  onClick={() => {
                    handleSelectProfile(selectedProfileId);
                    setStep(2);
                  }}
                  className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold transition-all shadow-md shadow-indigo-600/30"
                >
                  <span>{t('provisioning.nextSelectProfile')}</span>
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          ) : null}
        </div>
      )}

      {/* STEP 2: PROFILES & BLUEPRINTS */}
      {step === 2 && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h4 className="text-sm font-bold text-white uppercase tracking-wider">
                {t('provisioning.selectProfileTitle')}
              </h4>
              <p className="text-xs text-slate-400 mt-0.5">
                {t('provisioning.selectProfileSubtitle')}
              </p>
            </div>
          </div>

          {/* Profiles Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {profilesList.map((p) => {
              const active = selectedProfileId === p.id;
              const profileName = t(`provisioning.profiles.${p.id}.name`) !== `provisioning.profiles.${p.id}.name`
                ? t(`provisioning.profiles.${p.id}.name`)
                : p.name;
              const profileDesc = t(`provisioning.profiles.${p.id}.desc`) !== `provisioning.profiles.${p.id}.desc`
                ? t(`provisioning.profiles.${p.id}.desc`)
                : p.description;
              return (
                <div
                  key={p.id}
                  onClick={() => handleSelectProfile(p.id)}
                  className={`p-5 rounded-xl border transition-all cursor-pointer flex flex-col justify-between ${
                    active
                      ? 'bg-indigo-600/15 border-indigo-500 shadow-md shadow-indigo-600/20'
                      : 'bg-surface-850 border-slate-800 hover:border-slate-700'
                  }`}
                >
                  <div className="space-y-2">
                    <div className="flex items-start justify-between">
                      <h4 className="font-bold text-white text-sm">{profileName}</h4>
                      {active && <CheckCircle2 className="w-4 h-4 text-indigo-400 shrink-0" />}
                    </div>
                    <p className="text-xs text-slate-400 leading-relaxed">{profileDesc}</p>
                  </div>
                  <div className="mt-4 pt-3 border-t border-slate-800/60 text-[11px] font-mono text-slate-400 flex items-center justify-between">
                    <span>{p.recommended ? t('provisioning.recommendedProfile') : t('provisioning.standardProfile')}</span>
                    <span className="text-indigo-400 font-semibold">{t('common.select')}</span>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Saved Blueprints List */}
          {blueprintsList.length > 0 && (
            <div className="pt-6 border-t border-slate-800 space-y-4">
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <Bookmark className="w-4 h-4" />
                <span>{t('provisioning.savedBlueprints')}</span>
              </h4>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                {blueprintsList.map((b) => (
                  <div
                    key={b.id}
                    className="bg-surface-850 p-4 rounded-xl border border-slate-800 flex items-center justify-between gap-2"
                  >
                    <div
                      className="min-w-0 cursor-pointer flex-1"
                      onClick={() => handleLoadSavedBlueprint(b)}
                    >
                      <h5 className="font-bold text-white text-xs truncate">{b.name}</h5>
                      <p className="text-[11px] text-slate-400 truncate mt-0.5">{b.description || t('provisioning.customBlueprintDefaultDesc')}</p>
                    </div>

                    <button
                      onClick={() => handleDeleteBlueprint(b.id, b.name)}
                      className="p-1 rounded text-slate-500 hover:text-rose-400 transition-colors"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Navigation */}
          <div className="flex items-center justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(1)}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>{t('common.back')}</span>
            </button>

            <button
              onClick={() => setStep(3)}
              disabled={!blueprint}
              className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold transition-all shadow-md shadow-indigo-600/30 disabled:opacity-50"
            >
              <span>{t('provisioning.nextCustomize')}</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: CUSTOMIZE BLUEPRINT */}
      {step === 3 && blueprint && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h4 className="text-sm font-bold text-white uppercase tracking-wider">
                {t('provisioning.customizeTitle')}
              </h4>
              <p className="text-xs text-slate-400 mt-0.5">
                {t('provisioning.customizeSubtitle')}
              </p>
            </div>

            <button
              onClick={() => setShowSaveModal(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600/20 text-indigo-300 border border-indigo-500/30 text-xs font-semibold hover:bg-indigo-600/30 transition-colors"
            >
              <Bookmark className="w-3.5 h-3.5" />
              <span>{t('provisioning.saveAsBlueprintBtn')}</span>
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Web Server */}
            <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-4">
              <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <Globe className="w-4 h-4" />
                <span>{t('provisioning.sectionWebServer')}</span>
              </h5>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">{t('provisioning.fieldWebServer')}</label>
                <Select
                  value={blueprint.webserver.type}
                  onChange={(e) => {
                    const updated = { ...blueprint, webserver: { ...blueprint.webserver, type: e.target.value } };
                    setBlueprint(models.Blueprint.createFrom(updated));
                  }}
                  sizeVariant="md"
                >
                  <option value="nginx">{t('provisioning.optNginx')}</option>
                  <option value="apache">{t('provisioning.optApache')}</option>
                  <option value="both">{t('provisioning.optBoth')}</option>
                  <option value="none">{t('provisioning.optNoneWeb')}</option>
                </Select>
              </div>

              <label className="flex items-center gap-2 cursor-pointer pt-1">
                <input
                  type="checkbox"
                  checked={blueprint.webserver.enable_http2}
                  onChange={(e) => {
                    const updated = { ...blueprint, webserver: { ...blueprint.webserver, enable_http2: e.target.checked } };
                    setBlueprint(models.Blueprint.createFrom(updated));
                  }}
                  className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                />
                <span className="text-xs text-slate-300">{t('provisioning.enableHttp2')}</span>
              </label>
            </div>

            {/* Database & Cache */}
            <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-4">
              <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <Database className="w-4 h-4" />
                <span>{t('provisioning.sectionDatabaseCache')}</span>
              </h5>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">{t('provisioning.fieldDatabase')}</label>
                <Select
                  value={blueprint.database.type}
                  onChange={(e) => {
                    const updated = { ...blueprint, database: { ...blueprint.database, type: e.target.value } };
                    setBlueprint(models.Blueprint.createFrom(updated));
                  }}
                  sizeVariant="md"
                >
                  <option value="mariadb">{t('provisioning.optMariaDb')}</option>
                  <option value="mysql">{t('provisioning.optMySql')}</option>
                  <option value="postgresql">{t('provisioning.optPostgres')}</option>
                  <option value="sqlite">{t('provisioning.optSqlite')}</option>
                  <option value="none">{t('provisioning.optNoneDb')}</option>
                </Select>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">{t('provisioning.fieldCache')}</label>
                <Select
                  value={blueprint.cache.type}
                  onChange={(e) => {
                    const updated = { ...blueprint, cache: { ...blueprint.cache, type: e.target.value } };
                    setBlueprint(models.Blueprint.createFrom(updated));
                  }}
                  sizeVariant="md"
                >
                  <option value="redis">{t('provisioning.optRedis')}</option>
                  <option value="memcached">{t('provisioning.optMemcached')}</option>
                  <option value="none">{t('common.none')}</option>
                </Select>
              </div>
            </div>

            {/* PHP Versions */}
            <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-4">
              <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <Code2 className="w-4 h-4" />
                <span>{t('provisioning.sectionPhp')}</span>
              </h5>
              <div className="space-y-2">
                {blueprint.php.versions.map((ver, idx) => (
                  <div key={ver.version} className="p-3 rounded-lg bg-slate-900/60 border border-slate-800 flex items-center justify-between text-xs">
                    <div>
                      <span className="font-bold text-white font-mono">PHP {ver.version}</span>
                      <span className="text-slate-400 text-[11px] block mt-0.5 font-mono">
                        {t('provisioning.phpHandlerExtensions', { handler: ver.handler, count: ver.extensions.length })}
                      </span>
                    </div>
                    {ver.is_default && (
                      <span className="px-2 py-0.5 rounded bg-indigo-500/20 text-indigo-300 text-[10px] font-mono font-bold">
                        {t('provisioning.defaultCli')}
                      </span>
                    )}
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-4 text-xs text-slate-400">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={blueprint.tools.composer}
                    onChange={(e) => {
                      const updated = { ...blueprint, tools: { ...blueprint.tools, composer: e.target.checked } };
                      setBlueprint(models.Blueprint.createFrom(updated));
                    }}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                  />
                  <span>{t('provisioning.installComposer')}</span>
                </label>
              </div>
            </div>

            {/* Security & Containers */}
            <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-4">
              <h5 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <Shield className="w-4 h-4" />
                <span>{t('provisioning.sectionSecurityContainers')}</span>
              </h5>
              <div className="space-y-3 text-xs">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={blueprint.security.firewall !== 'none'}
                    onChange={(e) => {
                      const updated = { ...blueprint, security: { ...blueprint.security, firewall: e.target.checked ? 'ufw' : 'none' } };
                      setBlueprint(models.Blueprint.createFrom(updated));
                    }}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                  />
                  <span className="text-slate-200">{t('provisioning.enableFirewallWithSsh')}</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={blueprint.security.fail2ban}
                    onChange={(e) => {
                      const updated = { ...blueprint, security: { ...blueprint.security, fail2ban: e.target.checked } };
                      setBlueprint(models.Blueprint.createFrom(updated));
                    }}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                  />
                  <span className="text-slate-200">{t('provisioning.installFail2ban')}</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={blueprint.containers.docker}
                    onChange={(e) => {
                      const updated = { ...blueprint, containers: { ...blueprint.containers, docker: e.target.checked } };
                      setBlueprint(models.Blueprint.createFrom(updated));
                    }}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                  />
                  <span className="text-slate-200">{t('provisioning.installDocker')}</span>
                </label>
              </div>
            </div>
          </div>

          {planError && (
            <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
              {planError}
            </div>
          )}

          {/* Navigation */}
          <div className="flex items-center justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(2)}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>{t('common.back')}</span>
            </button>

            <button
              onClick={handleGeneratePlan}
              disabled={isPlanning}
              className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold transition-all shadow-md shadow-indigo-600/30 disabled:opacity-50"
            >
              {isPlanning && <Loader2 className="w-4 h-4 animate-spin" />}
              <span>{isPlanning ? t('provisioning.calculatingPlan') : t('provisioning.generatePlanBtn')}</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 4: PLAN REVIEW */}
      {step === 4 && plan && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h4 className="text-sm font-bold text-white uppercase tracking-wider flex items-center gap-2">
                <Zap className="w-4 h-4 text-indigo-400" />
                <span>{t('provisioning.planReviewTitle')}</span>
              </h4>
              <p className="text-xs text-slate-400 mt-0.5">
                {t('provisioning.planReviewSubtitle')}
              </p>
            </div>
            <div className="text-xs font-mono text-slate-400">
              {t('provisioning.estimatedTime', { min: plan.estimated_time_min })}
            </div>
          </div>

          {/* Warnings & Risk Alerts */}
          {plan.warnings && plan.warnings.length > 0 && (
            <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs space-y-2">
              <div className="font-bold flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-amber-400" />
                <span>{t('provisioning.warningsTitle')}</span>
              </div>
              <ul className="list-disc pl-5 space-y-1">
                {plan.warnings.map((w, i) => (
                  <li key={i}>{localizeWarning(w, t)}</li>
                ))}
              </ul>
            </div>
          )}

          {/* Plan Summary Blocks */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
              <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.packagesToInstall')}</span>
              <div className="font-bold text-white text-lg mt-1">{plan.packages_to_install.length}</div>
              <div className="text-[11px] text-slate-500 truncate mt-1">{plan.packages_to_install.slice(0, 5).join(', ')}...</div>
            </div>

            <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
              <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.servicesToStart')}</span>
              <div className="font-bold text-white text-lg mt-1">{plan.services_to_start.length}</div>
              <div className="text-[11px] text-slate-500 truncate mt-1">{plan.services_to_start.join(', ')}</div>
            </div>

            <div className="bg-surface-850 p-4 rounded-xl border border-slate-800">
              <span className="text-[11px] text-slate-400 block font-medium">{t('provisioning.portsToOpen')}</span>
              <div className="font-bold text-white text-lg mt-1">{plan.ports_to_open.join(', ')}</div>
              <div className="text-[11px] text-purple-300 font-mono mt-1">{t('provisioning.sshPreserved', { port: plan.ssh_port_preserved })}</div>
            </div>
          </div>

          {/* Categorized Steps List */}
          <div className="bg-surface-850 p-5 rounded-xl border border-slate-800 space-y-3">
            <h5 className="text-xs font-bold text-white uppercase tracking-wider">
              {t('provisioning.planSteps')}
            </h5>
            <div className="divide-y divide-slate-800/60 max-h-96 overflow-y-auto">
              {plan.items.map((item, idx) => (
                <div key={idx} className="py-2.5 flex items-start justify-between gap-3 text-xs">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="px-1.5 py-0.5 rounded bg-slate-800 font-mono text-[10px] text-slate-400 uppercase">
                        {item.category}
                      </span>
                      <span className="font-semibold text-white">{localizePlanItemDescription(item.description, t)}</span>
                    </div>
                    <span className="text-[11px] text-slate-500 font-mono block mt-0.5">{item.action}: {item.target}</span>
                  </div>

                  {item.skip ? (
                    <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-400 text-[10px] font-mono shrink-0">
                      {t('provisioning.stepSkipConfigured')}
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 text-[10px] font-mono shrink-0">
                      {t('provisioning.stepApply')}
                    </span>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Navigation */}
          <div className="flex items-center justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(3)}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>{t('common.back')}</span>
            </button>

            <button
              onClick={handleExecute}
              disabled={isExecuting}
              className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold transition-all shadow-md shadow-emerald-600/30 disabled:opacity-50"
            >
              {isExecuting && <Loader2 className="w-4 h-4 animate-spin" />}
              <span>{isExecuting ? t('provisioning.launching') : t('provisioning.startExecutionBtn')}</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 5: EXECUTION MONITOR */}
      {step === 5 && (
        <div className="bg-surface-850 p-8 rounded-xl border border-slate-800 text-center space-y-6">
          <div className="w-16 h-16 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 flex items-center justify-center mx-auto">
            <Wrench className="w-8 h-8 animate-pulse" />
          </div>

          <div className="space-y-2 max-w-md mx-auto">
            <h4 className="font-bold text-white text-lg">
              {t('provisioning.taskStartedTitle')}
            </h4>
            <p className="text-xs text-slate-400 leading-relaxed">
              {t('provisioning.taskStartedDesc')}
            </p>
          </div>

          {activeTask && (
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 max-w-sm mx-auto text-xs font-mono text-slate-300">
              {t('provisioning.taskIdLabel', { id: activeTask.id })}
            </div>
          )}

          <div className="flex items-center justify-center gap-4 pt-4">
            <button
              onClick={() => {
                if (activeTask && onOpenTask) onOpenTask(activeTask);
              }}
              className="px-6 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-bold transition-all"
            >
              {t('provisioning.openTaskLogBtn')}
            </button>
            <button
              onClick={() => setStep(1)}
              className="px-4 py-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              {t('provisioning.backToWizard')}
            </button>
          </div>
        </div>
      )}

      {/* Modal: Save Blueprint */}
      {showSaveModal && (
        <ModalShell
          onClose={() => setShowSaveModal(false)}
          size="md"
          icon={<Bookmark className="w-5 h-5 text-indigo-400" />}
          title={t('provisioning.saveBlueprintModalTitle')}
          subtitle={t('provisioning.blueprintDesc')}
        >
          <form onSubmit={handleSaveBlueprint} className="flex flex-col flex-1 min-h-0">
            <div className="p-6 space-y-4 flex-1">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  {t('provisioning.blueprintName')} *
                </label>
                <input
                  type="text"
                  required
                  value={saveName}
                  onChange={(e) => setSaveName(e.target.value)}
                  placeholder={t('provisioning.blueprintNamePlaceholder')}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-indigo-500 font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                  {t('provisioning.blueprintDesc')}
                </label>
                <textarea
                  value={saveDesc}
                  onChange={(e) => setSaveDesc(e.target.value)}
                  rows={3}
                  placeholder={t('provisioning.blueprintDescPlaceholder')}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-600 focus:outline-none focus:border-indigo-500 resize-none"
                />
              </div>
            </div>

            {/* Modal Footer */}
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-end gap-3 shrink-0">
              <button
                type="button"
                onClick={() => setShowSaveModal(false)}
                className="px-4 py-2 rounded-xl text-sm font-medium text-slate-300 hover:bg-slate-800 transition-colors"
              >
                {t('common.cancel')}
              </button>
              <button
                type="submit"
                disabled={isSavingBp || !saveName.trim()}
                className="px-5 py-2 rounded-xl text-sm font-medium bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/30 flex items-center gap-2 transition-all disabled:opacity-50"
              >
                {isSavingBp && <Loader2 className="w-4 h-4 animate-spin" />}
                <span>{isSavingBp ? t('common.saving') : t('common.save')}</span>
              </button>
            </div>
          </form>
        </ModalShell>
      )}
    </div>
  );
};
