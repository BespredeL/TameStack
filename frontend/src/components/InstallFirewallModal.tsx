import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Shield, AlertCircle, Loader2, ArrowRight, ShieldCheck, Download } from 'lucide-react';
import { PlanFirewallInstall, InstallFirewall } from '../../wailsjs/go/main/App';
import { security } from '../../wailsjs/go/models';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button } from './ui';

interface InstallFirewallModalProps {
  isOpen: boolean;
  serverID: string;
  serverName: string;
  onClose: () => void;
  onTaskStarted: (task: TaskRecord) => void;
}

export const InstallFirewallModal: React.FC<InstallFirewallModalProps> = ({
  isOpen,
  serverID,
  serverName,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const [variant, setVariant] = useState<string>('ufw');
  const [plan, setPlan] = useState<security.FirewallInstallPlan | null>(null);
  const [isPlanning, setIsPlanning] = useState(false);
  const [planError, setPlanError] = useState<string | null>(null);
  const [isInstalling, setIsInstalling] = useState(false);

  if (!isOpen) return null;

  const handleVariantChange = (v: string) => {
    setVariant(v);
    setPlan(null);
    setPlanError(null);
  };

  const handleCreatePlan = async () => {
    setIsPlanning(true);
    setPlanError(null);
    try {
      const res = await PlanFirewallInstall(serverID, variant);
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
      const task = await InstallFirewall(serverID, variant);
      onTaskStarted(task as unknown as TaskRecord);
      onClose();
    } catch (err: unknown) {
      setPlanError(getErrorMessage(err));
    } finally {
      setIsInstalling(false);
    }
  };

  const installBlocked = plan !== null && (plan.os_compatible === false || plan.already_installed === true);

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[88vh]"
      icon={<Shield className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-emerald-600/20 text-emerald-400 border border-emerald-500/30 shrink-0"
      title={t('security.installFirewallModal.title')}
      subtitle={t('security.installFirewallModal.targetServer', { server: serverName })}
    >
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {planError && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{planError}</span>
          </div>
        )}

        <p className="text-xs text-slate-400 leading-relaxed">
          {t('security.installFirewallModal.hint')}
        </p>

        <div className="space-y-2">
          <span className="text-xs font-bold text-slate-200 block">{t('security.installFirewallModal.variantHeading')}</span>
          <div className="grid grid-cols-2 gap-2">
            {[
              { id: 'ufw', label: 'UFW', desc: t('security.installFirewallModal.variantUfwDesc') },
              { id: 'firewalld', label: 'firewalld', desc: t('security.installFirewallModal.variantFirewalldDesc') },
            ].map((v) => (
              <button
                key={v.id}
                type="button"
                onClick={() => handleVariantChange(v.id)}
                className={`p-3 rounded-lg border text-left transition-all ${
                  variant === v.id
                    ? 'bg-emerald-600/10 border-emerald-500/40 text-white'
                    : 'bg-slate-950 border-slate-800 text-slate-400 hover:text-slate-200'
                }`}
              >
                <div className="font-mono text-xs font-semibold">{v.label}</div>
                <div className="text-[10px] text-slate-400 leading-tight mt-0.5">{v.desc}</div>
              </button>
            ))}
          </div>
        </div>

        <div className="p-4 rounded-xl bg-slate-950/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              <span className="text-xs font-bold text-slate-200">{t('security.installFirewallModal.planHeading')}</span>
            </div>
            {!plan && (
              <button
                type="button"
                onClick={handleCreatePlan}
                disabled={isPlanning}
                className="px-3 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-colors disabled:opacity-50"
              >
                {isPlanning ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                <span>{t('security.installFirewallModal.generatePlan')}</span>
              </button>
            )}
          </div>

          {plan && (
            <div className="space-y-3 text-xs pt-2 border-t border-slate-800">
              {plan.os_compatible === false ? (
                <div className="p-3 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  <div>
                    <div className="font-bold text-rose-200">{t('security.installFirewallModal.compatibilityBlocked')}</div>
                    <div className="mt-0.5">{plan.message}</div>
                  </div>
                </div>
              ) : plan.already_installed === true ? (
                <div className="p-3 rounded-lg bg-amber-500/15 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2">
                  <AlertCircle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                  <div>{plan.message}</div>
                </div>
              ) : (
                <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
                  {plan.message}
                </div>
              )}

              {plan.os_compatible !== false && plan.already_installed !== true && (
                <>
                  <div>
                    <span className="text-[11px] font-semibold text-slate-400 block mb-1">{t('security.installFirewallModal.packagesToInstall')}</span>
                    <div className="flex flex-wrap gap-1.5">
                      {plan.packages_to_install.map((pkg, i) => (
                        <span key={i} className="px-2 py-0.5 rounded bg-slate-900 text-slate-300 font-mono text-[10px] border border-slate-800">
                          {pkg}
                        </span>
                      ))}
                    </div>
                  </div>
                  <div className="text-slate-400 text-[11px] pt-1">
                    {t('security.installFirewallModal.systemdService', { service: plan.service_to_start })}
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={onClose}
        >
          {t('common.cancel')}
        </Button>

        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={plan ? handleStartInstallation : handleCreatePlan}
          disabled={isInstalling || isPlanning || installBlocked}
          loading={isInstalling || isPlanning}
          icon={!isInstalling && !isPlanning ? (plan ? <Download className="w-3.5 h-3.5" /> : <ArrowRight className="w-3.5 h-3.5" />) : undefined}
          className="bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-600/30"
        >
          {plan ? t('security.installFirewallModal.confirmAndInstall') : t('security.installFirewallModal.generatePlan')}
        </Button>
      </div>
    </ModalShell>
  );
};