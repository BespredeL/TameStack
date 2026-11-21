import React from 'react';
import { Lock, Unlock, AlertTriangle } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { Button, Input } from '../ui';
import { useTranslation } from '../../i18n/LanguageContext';

interface UnlockVaultModalProps {
  onClose: () => void;
  unlockPasswordInput: string;
  setUnlockPasswordInput: React.Dispatch<React.SetStateAction<string>>;
  vaultActionLoading: boolean;
  vaultActionError: string | null;
  onSubmit: (e: React.FormEvent) => void;
}

export const UnlockVaultModal: React.FC<UnlockVaultModalProps> = ({
  onClose,
  unlockPasswordInput,
  setUnlockPasswordInput,
  vaultActionLoading,
  vaultActionError,
  onSubmit,
}) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Lock className="w-6 h-6" />}
      iconClassName="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 shrink-0"
      title={t('settings.unlockCryptographicVault')}
      subtitle={t('settings.enterMasterPasswordHint')}
    >
      <div className="p-6 space-y-4">
        <form onSubmit={onSubmit} className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('vault.masterPassword')}
            </label>
            <Input
              type="password"
              value={unlockPasswordInput}
              onChange={(e) => setUnlockPasswordInput(e.target.value)}
              placeholder="••••••••••••"
              required
              autoFocus
              className="font-mono"
            />
          </div>

          {vaultActionError && (
            <div className="p-2.5 bg-rose-950/50 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0" />
              <span>{vaultActionError}</span>
            </div>
          )}

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
              variant="danger"
              size="sm"
              disabled={vaultActionLoading}
              loading={vaultActionLoading}
              icon={<Unlock className="w-3.5 h-3.5" />}
            >
              {vaultActionLoading ? t('settings.verifying') : t('settings.unlock')}
            </Button>
          </div>
        </form>
      </div>
    </ModalShell>
  );
};