import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Code2, CheckSquare, Square, AlertCircle, ArrowRight, ShieldCheck, Download } from 'lucide-react';
import { PlanPHPInstall, InstallPHPVersion } from '../../wailsjs/go/main/App';
import { php } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button } from './ui';

interface InstallPHPModalProps {
  isOpen: boolean;
  serverID: string;
  serverName: string;
  version: string;
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

export const InstallPHPModal: React.FC<InstallPHPModalProps> = ({
  isOpen,
  serverID,
  serverName,
  version,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const availableExtensions = availableExtensionIds.map(ext => ({
    ...ext,
    desc: t(`php.installModal.${ext.key}`),
  }));
  const [selectedExtensions, setSelectedExtensions] = useState<string[]>([
    'curl',
    'mbstring',
    'mysql',
    'xml',
    'zip',
    'gd',
    'intl',
    'redis',
    'fileinfo',
    'bcmath',
  ]);
  const [plan, setPlan] = useState<php.InstallPHPPlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);

  if (!isOpen) return null;

  const toggleExt = (id: string) => {
    if (selectedExtensions.includes(id)) {
      setSelectedExtensions(selectedExtensions.filter((e) => e !== id));
    } else {
      setSelectedExtensions([...selectedExtensions, id]);
    }
  };

  const handleCreatePlan = async () => {
    setIsPlanning(true);
    setPlanError(null);
    try {
      const res = await PlanPHPInstall(serverID, version, selectedExtensions);
      setPlan(res);
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsPlanning(false);
    }
  };

  const handleStartInstallation = async () => {
    setIsInstalling(true);
    try {
      const task = await InstallPHPVersion(serverID, version, selectedExtensions);
      onTaskStarted(task);
      onClose();
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsInstalling(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[88vh]"
      icon={<Code2 className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={t('php.installModal.title', { version })}
      subtitle={t('php.installModal.targetServer', { server: serverName })}
    >
      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {planError && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{planError}</span>
          </div>
        )}

        {/* Step 1: Select Extensions */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-200">{t('php.installModal.selectExtensions')}</span>
            <span className="text-xs text-indigo-400 font-mono">{t('php.installModal.selectedCount', { count: selectedExtensions.length })}</span>
          </div>

          <div className="border border-slate-800 rounded-xl max-h-60 overflow-y-auto divide-y divide-slate-800/60 bg-slate-950/70">
            {availableExtensions.map((ext) => {
              const isChecked = selectedExtensions.includes(ext.id);
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

        {/* Plan Preview Section */}
        <div className="p-4 rounded-xl bg-slate-950/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-indigo-400" />
              <span className="text-xs font-bold text-slate-200">{t('php.installModal.planHeading')}</span>
            </div>
            {!plan && (
              <Button
                variant="secondary"
                size="sm"
                onClick={handleCreatePlan}
                loading={isPlanning}
              >
                {t('php.installModal.preliminaryPlan')}
              </Button>
            )}
          </div>

          {plan && (
            <div className="space-y-3 text-xs pt-2 border-t border-slate-800">
              {plan.os_compatible === false ? (
                <div className="p-3 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  <div>
                    <div className="font-bold text-rose-200">{t('php.installModal.compatibilityBlocked')}</div>
                    <div className="mt-0.5">{plan.message}</div>
                  </div>
                </div>
              ) : (
                <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
                  {plan.message}
                </div>
              )}

              {plan.repositories_needed && plan.repositories_needed.length > 0 && (
                <div>
                  <span className="text-[11px] font-semibold text-slate-400 block mb-1">{t('php.installModal.reposToConnect')}</span>
                  <div className="space-y-1">
                    {plan.repositories_needed.map((repo, i) => (
                      <div key={i} className="font-mono text-[11px] text-amber-300 bg-slate-900 px-2 py-1 rounded">
                        + {repo}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <span className="text-[11px] font-semibold text-slate-400 block mb-1">{t('php.installModal.packagesToInstall')}</span>
                <div className="flex flex-wrap gap-1.5">
                  {plan.packages_to_install.map((pkg, i) => (
                    <span key={i} className="px-2 py-0.5 rounded bg-slate-900 text-slate-300 font-mono text-[10px] border border-slate-800">
                      {pkg}
                    </span>
                  ))}
                </div>
              </div>

              <div className="flex items-center justify-between text-slate-400 text-[11px] pt-1">
                <span>{t('php.installModal.systemdService', { service: plan.fpm_service_to_start })}</span>
                <span>{t('php.installModal.estimatedDownload', { size: plan.estimated_download_mb })}</span>
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
          disabled={plan !== null && plan.os_compatible === false}
          icon={plan ? <Download className="w-3.5 h-3.5" /> : <ArrowRight className="w-3.5 h-3.5" />}
        >
          {plan ? t('php.installModal.confirmAndInstall') : t('php.installModal.generatePlan')}
        </Button>
      </div>
    </ModalShell>
  );
};
