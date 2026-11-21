import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { HardDrive, AlertCircle, ArrowRight, ShieldCheck, Download, AlertTriangle } from 'lucide-react';
import { PlanWebserverInstall, InstallWebserver } from '../../wailsjs/go/main/App';
import { webserver } from '../../wailsjs/go/models';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button } from './ui';

interface InstallWebServerModalProps {
  isOpen: boolean;
  serverID: string;
  serverName: string;
  wsType: string;
  displayName: string;
  onClose: () => void;
  onTaskStarted: (task: TaskRecord) => void;
}

export const InstallWebServerModal: React.FC<InstallWebServerModalProps> = ({
  isOpen,
  serverID,
  serverName,
  wsType,
  displayName,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const [plan, setPlan] = useState<webserver.InstallPlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);

  if (!isOpen) return null;

  const handleCreatePlan = async () => {
    setIsPlanning(true);
    setPlanError(null);
    try {
      const res = await PlanWebserverInstall(serverID, wsType);
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
      const task = await InstallWebserver(serverID, wsType);
      onTaskStarted(task as unknown as TaskRecord);
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
      icon={<HardDrive className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-indigo-600/20 text-indigo-400 border border-indigo-500/30 shrink-0"
      title={t('webservers.installModal.title', { name: displayName })}
      subtitle={t('webservers.installModal.targetServer', { server: serverName })}
    >
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {planError && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{planError}</span>
          </div>
        )}

        <p className="text-xs text-slate-400 leading-relaxed">
          {t('webservers.installModal.hint')}
        </p>

        <div className="p-4 rounded-xl bg-slate-950/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-indigo-400" />
              <span className="text-xs font-bold text-slate-200">{t('webservers.installModal.planHeading')}</span>
            </div>
            {!plan && (
              <Button
                variant="secondary"
                size="sm"
                onClick={handleCreatePlan}
                loading={isPlanning}
              >
                {t('webservers.installModal.generatePlan')}
              </Button>
            )}
          </div>

          {plan && (
            <div className="space-y-3 text-xs pt-2 border-t border-slate-800">
              {plan.os_compatible === false ? (
                <div className="p-3 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  <div>
                    <div className="font-bold text-rose-200">{t('webservers.installModal.compatibilityBlocked')}</div>
                    <div className="mt-0.5">{plan.message}</div>
                  </div>
                </div>
              ) : (
                <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
                  {plan.message}
                </div>
              )}

              {plan.port_conflict_warning && (
                <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                  <span>{plan.port_conflict_warning}</span>
                </div>
              )}

              {plan.os_compatible !== false && (
                <>
                  <div>
                    <span className="text-[11px] font-semibold text-slate-400 block mb-1">{t('webservers.installModal.packagesToInstall')}</span>
                    <div className="flex flex-wrap gap-1.5">
                      {plan.packages_to_install.map((pkg, i) => (
                        <span key={i} className="px-2 py-0.5 rounded bg-slate-900 text-slate-300 font-mono text-[10px] border border-slate-800">
                          {pkg}
                        </span>
                      ))}
                    </div>
                  </div>
                  <div className="text-slate-400 text-[11px] pt-1">
                    {t('webservers.installModal.systemdService', { service: plan.service_to_start })}
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </div>

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
          {plan ? t('webservers.installModal.confirmAndInstall') : t('webservers.installModal.generatePlan')}
        </Button>
      </div>
    </ModalShell>
  );
};
