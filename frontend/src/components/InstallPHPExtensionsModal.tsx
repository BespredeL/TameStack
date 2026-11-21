import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Puzzle, CheckSquare, Square, AlertCircle, ArrowRight, ShieldCheck, Download, CheckCircle2 } from 'lucide-react';
import { PlanPHPExtensionsInstall, InstallPHPExtensions } from '../../wailsjs/go/main/App';
import { php } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button } from './ui';

interface InstallPHPExtensionsModalProps {
  isOpen: boolean;
  serverID: string;
  serverName: string;
  version: string;
  installedExtensions: php.PHPExtensionInfo[];
  onClose: () => void;
  onTaskStarted: (task: any) => void;
}

const availableExtensionIds = [
  { id: 'curl', label: 'curl', key: 'extCurlDesc' },
  { id: 'mbstring', label: 'mbstring', key: 'extMbstringDesc' },
  { id: 'mysql', label: 'mysql', key: 'extMysqlDesc' },
  { id: 'xml', label: 'xml', key: 'extXmlDesc' },
  { id: 'zip', label: 'zip', key: 'extZipDesc' },
  { id: 'gd', label: 'gd', key: 'extGdDesc' },
  { id: 'intl', label: 'intl', key: 'extIntlDesc' },
  { id: 'imagick', label: 'imagick', key: 'extImagickDesc' },
  { id: 'redis', label: 'redis', key: 'extRedisDesc' },
  { id: 'igbinary', label: 'igbinary', key: 'extIgbinaryDesc' },
  { id: 'soap', label: 'soap', key: 'extSoapDesc' },
  { id: 'imap', label: 'imap', key: 'extImapDesc' },
  { id: 'bcmath', label: 'bcmath', key: 'extBcmathDesc' },
  { id: 'sockets', label: 'sockets', key: 'extSocketsDesc' },
  { id: 'fileinfo', label: 'fileinfo', key: 'extFileinfoDesc' },
];

export const InstallPHPExtensionsModal: React.FC<InstallPHPExtensionsModalProps> = ({
  isOpen,
  serverID,
  serverName,
  version,
  installedExtensions,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const [selectedExtensions, setSelectedExtensions] = useState<string[]>([]);
  const [plan, setPlan] = useState<php.InstallPHPExtensionsPlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);

  if (!isOpen) return null;

  const activeNames = new Set(
    (installedExtensions || []).filter((e) => e.is_enabled).map((e) => e.name.toLowerCase()),
  );
  const availableExtensions = availableExtensionIds.map((ext) => ({
    ...ext,
    desc: t(`php.installModal.${ext.key}`),
    alreadyActive: activeNames.has(ext.id),
  }));
  const installableCount = availableExtensions.filter((e) => !e.alreadyActive).length;

  const toggleExt = (id: string) => {
    setPlan(null);
    setPlanError(null);
    if (selectedExtensions.includes(id)) {
      setSelectedExtensions(selectedExtensions.filter((e) => e !== id));
    } else {
      setSelectedExtensions([...selectedExtensions, id]);
    }
  };

  const handleCreatePlan = async () => {
    if (selectedExtensions.length === 0) {
      setPlanError(t('php.installExtensionsModal.noSelection'));
      return;
    }
    setIsPlanning(true);
    setPlanError(null);
    try {
      const res = await PlanPHPExtensionsInstall(serverID, version, selectedExtensions);
      setPlan(res);
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsPlanning(false);
    }
  };

  const handleStartInstallation = async () => {
    setIsInstalling(true);
    setPlanError(null);
    try {
      const task = await InstallPHPExtensions(serverID, version, selectedExtensions);
      onTaskStarted(task);
      onClose();
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsInstalling(false);
    }
  };

  const installBlocked =
    selectedExtensions.length === 0 || (plan !== null && plan.os_compatible === false);

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[88vh]"
      icon={<Puzzle className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={t('php.installExtensionsModal.title', { version })}
      subtitle={t('php.installExtensionsModal.targetServer', { server: serverName })}
    >
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {planError && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{planError}</span>
          </div>
        )}

        <p className="text-xs text-slate-400 leading-relaxed">
          {t('php.installExtensionsModal.hint', { version })}
        </p>

        {installableCount === 0 ? (
          <div className="p-4 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{t('php.installExtensionsModal.allInstalled')}</span>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-bold text-slate-200">{t('php.installExtensionsModal.selectExtensions')}</span>
              <span className="text-xs text-indigo-400 font-mono">{t('php.installExtensionsModal.selectedCount', { count: selectedExtensions.length })}</span>
            </div>

            <div className="border border-slate-800 rounded-xl max-h-60 overflow-y-auto divide-y divide-slate-800/60 bg-slate-950/70">
              {availableExtensions.map((ext) => {
                const isChecked = selectedExtensions.includes(ext.id);
                if (ext.alreadyActive) {
                  return (
                    <div
                      key={ext.id}
                      className="p-2.5 flex items-start gap-3 bg-slate-950/40 text-slate-500 opacity-60"
                      title={t('php.installExtensionsModal.alreadyInstalled')}
                    >
                      <CheckCircle2 className="w-4 h-4 text-emerald-500/70 mt-0.5 shrink-0" />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs font-semibold">{ext.label}</span>
                          <span className="text-[10px] px-1.5 py-0.2 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">
                            {t('php.installExtensionsModal.alreadyInstalled')}
                          </span>
                        </div>
                        <div className="text-[11px] text-slate-500 leading-snug mt-0.5">{ext.desc}</div>
                      </div>
                    </div>
                  );
                }
                return (
                  <button
                    key={ext.id}
                    type="button"
                    onClick={() => toggleExt(ext.id)}
                    className={`w-full p-2.5 text-left flex items-start gap-3 transition-colors ${
                      isChecked
                        ? 'bg-indigo-600/10 text-white'
                        : 'hover:bg-slate-900/60 text-slate-300'
                    }`}
                  >
                    {isChecked ? (
                      <CheckSquare className="w-4 h-4 text-indigo-400 mt-0.5 shrink-0" />
                    ) : (
                      <Square className="w-4 h-4 text-slate-600 mt-0.5 shrink-0" />
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="font-mono text-xs font-semibold text-white">{ext.label}</div>
                      <div className="text-[11px] text-slate-400 leading-snug mt-0.5">{ext.desc}</div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {/* Plan Preview */}
        <div className="p-4 rounded-xl bg-slate-950/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-indigo-400" />
              <span className="text-xs font-bold text-slate-200">{t('php.installExtensionsModal.planHeading')}</span>
            </div>
            {!plan && installableCount > 0 && (
              <Button
                variant="secondary"
                size="sm"
                onClick={handleCreatePlan}
                loading={isPlanning}
                disabled={selectedExtensions.length === 0}
              >
                {t('php.installExtensionsModal.preliminaryPlan')}
              </Button>
            )}
          </div>

          {plan && (
            <div className="space-y-3 text-xs pt-2 border-t border-slate-800">
              {plan.os_compatible === false ? (
                <div className="p-3 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  <div>
                    <div className="font-bold text-rose-200">{t('php.installExtensionsModal.compatibilityBlocked')}</div>
                    <div className="mt-0.5">{plan.message}</div>
                  </div>
                </div>
              ) : (
                <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
                  {plan.message}
                </div>
              )}

              {plan.packages_to_install && plan.packages_to_install.length > 0 && (
                <div>
                  <span className="text-[11px] font-semibold text-slate-400 block mb-1">{t('php.installExtensionsModal.packagesToInstall')}</span>
                  <div className="flex flex-wrap gap-1.5">
                    {plan.packages_to_install.map((pkg, i) => (
                      <span key={i} className="px-2 py-0.5 rounded bg-slate-900 text-slate-300 font-mono text-[10px] border border-slate-800">
                        {pkg}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              <div className="text-slate-400 text-[11px] pt-1">
                {t('php.installExtensionsModal.fpmReload', { service: plan.fpm_service_to_reload })}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <Button
          variant="secondary"
          size="md"
          onClick={onClose}
        >
          {t('common.cancel')}
        </Button>

        <Button
          variant="primary"
          size="md"
          onClick={plan ? handleStartInstallation : handleCreatePlan}
          loading={isInstalling || isPlanning}
          disabled={installBlocked}
          icon={plan ? <Download className="w-3.5 h-3.5" /> : <ArrowRight className="w-3.5 h-3.5" />}
        >
          {plan ? t('php.installExtensionsModal.confirmAndInstall') : t('php.installExtensionsModal.generatePlan')}
        </Button>
      </div>
    </ModalShell>
  );
};
