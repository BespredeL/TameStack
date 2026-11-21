import React, { useState, useEffect, useRef } from 'react';
import { getErrorMessage } from '../utils/errors';
import { Lock, Unlock, Eye, EyeOff, AlertTriangle, KeyRound } from 'lucide-react';
import { UnlockVault } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { Button, Input } from './ui';
import { ModalShell } from './common/ModalShell';

interface UnlockVaultModalProps {
  isOpen: boolean;
  onClose: () => void;
  onUnlocked?: () => void;
  reason?: string | null;
  isClosable?: boolean;
}

export const UnlockVaultModal: React.FC<UnlockVaultModalProps> = ({
  isOpen,
  onClose,
  onUnlocked,
  reason,
  isClosable = true,
}) => {
  const { t } = useTranslation();

  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setPassword('');
      setError(null);
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password.trim() || isLoading) return;

    setIsLoading(true);
    setError(null);

    try {
      await UnlockVault(password);
      setPassword('');
      if (onUnlocked) {
        onUnlocked();
      }
      onClose();
    } catch (err: unknown) {
      const msg = getErrorMessage(err);
      if (msg.includes('invalid master password')) {
        setError(t('vault.invalidPassword'));
      } else {
        setError(msg);
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <ModalShell
      onClose={isClosable ? onClose : () => {}}
      size="md"
      icon={<Lock className="w-5 h-5 text-amber-400" />}
      iconClassName="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400 shrink-0"
      title={t('vault.unlockTitle')}
      subtitle={t('vault.unlockSubtitle')}
      hideCloseButton={!isClosable}
      closeOnEsc={isClosable}
      closeOnBackdrop={false}
    >
      <div className="p-6 space-y-4">
        {/* Reason banner if triggered by a specific action */}
        {reason && (
          <div className="p-3 bg-amber-950/40 border border-amber-500/30 rounded-xl text-xs text-amber-200 flex items-start gap-2.5">
            <KeyRound className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <span className="leading-relaxed">{reason}</span>
          </div>
        )}

        {/* Password Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <label className="block text-xs font-semibold text-slate-300">
              {t('vault.masterPassword')}
            </label>
            <Input
              ref={inputRef}
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••••••"
              required
              disabled={isLoading}
              className="font-mono"
              iconRight={
                <button
                  type="button"
                  onClick={() => setShowPassword((prev) => !prev)}
                  className="text-slate-400 hover:text-slate-200 transition cursor-pointer"
                  tabIndex={-1}
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              }
            />
          </div>

          {error && (
            <div className="p-3 bg-rose-950/50 border border-rose-500/40 rounded-xl text-xs text-rose-300 flex items-start gap-2.5 animate-in fade-in">
              <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <span className="leading-relaxed">{error}</span>
            </div>
          )}

          <div className="flex items-center justify-end gap-2.5 pt-2">
            {isClosable && (
              <Button
                type="button"
                variant="ghost"
                size="md"
                onClick={onClose}
                disabled={isLoading}
              >
                {t('common.cancel')}
              </Button>
            )}

            <Button
              type="submit"
              variant="primary"
              size="md"
              disabled={isLoading || !password.trim()}
              loading={isLoading}
              icon={<Unlock className="w-4 h-4" />}
              className="bg-amber-600 hover:bg-amber-500 text-white shadow-amber-600/25"
            >
              {isLoading ? t('vault.verifying') : t('vault.unlockBtn')}
            </Button>
          </div>
        </form>
      </div>
    </ModalShell>
  );
};
