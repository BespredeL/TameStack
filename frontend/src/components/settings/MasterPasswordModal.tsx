import React from 'react';
import { KeyRound, AlertTriangle } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { Button, Input } from '../ui';
import { useTranslation } from '../../i18n/LanguageContext';

interface MasterPasswordModalProps {
  onClose: () => void;
  hasMasterPassword: boolean;
  masterPasswordInput: string;
  setMasterPasswordInput: React.Dispatch<React.SetStateAction<string>>;
  masterPasswordConfirm: string;
  setMasterPasswordConfirm: React.Dispatch<React.SetStateAction<string>>;
  vaultActionLoading: boolean;
  vaultActionError: string | null;
  onSubmit: (e: React.FormEvent) => void;
}

export const MasterPasswordModal: React.FC<MasterPasswordModalProps> = ({
  onClose,
  hasMasterPassword,
  masterPasswordInput,
  setMasterPasswordInput,
  masterPasswordConfirm,
  setMasterPasswordConfirm,
  vaultActionLoading,
  vaultActionError,
  onSubmit,
}) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<KeyRound className="w-6 h-6" />}
      iconClassName="p-2.5 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 shrink-0"
      title={hasMasterPassword ? t('settings.changeMasterPassword') : t('settings.setupMasterPassword')}
      subtitle={t('settings.zeroTrustDesc')}
    >
      <div className="p-6 space-y-4">
        <p className="text-xs text-slate-300 leading-relaxed">
          {t('settings.masterPasswordInfo')}
        </p>

        <form onSubmit={onSubmit} className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('settings.newPasswordMin8')}
            </label>
            <Input
              type="password"
              value={masterPasswordInput}
              onChange={(e) => setMasterPasswordInput(e.target.value)}
              placeholder="••••••••••••"
              required
              autoFocus
              className="font-mono"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('settings.confirmMasterPassword')}
            </label>
            <Input
              type="password"
              value={masterPasswordConfirm}
              onChange={(e) => setMasterPasswordConfirm(e.target.value)}
              placeholder="••••••••••••"
              required
              className="font-mono"
            />
          </div>

          {vaultActionError && (
            <div className="p-2.5 bg-rose-950/50 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0" />
              <span>{vaultActionError}</span>
            </div>
          )}

          <div className="p-2.5 bg-amber-950/30 border border-amber-500/20 rounded-lg text-[11px] text-amber-300">
            ⚠️ {t('settings.rememberPasswordWarning')}
          </div>

          <div className="flex items-center justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onClose}
              disabled={vaultActionLoading}
            >
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={vaultActionLoading}
              loading={vaultActionLoading}
              icon={<KeyRound className="w-3.5 h-3.5" />}
            >
              {vaultActionLoading ? t('settings.encrypting') : t('settings.savePassword')}
            </Button>
          </div>
        </form>
      </div>
    </ModalShell>
  );
};