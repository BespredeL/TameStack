import React from 'react';
import { Database, AlertTriangle, Lock } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { Button, Input } from '../ui';
import { useTranslation } from '../../i18n/LanguageContext';

interface DbEncryptionModalProps {
  onClose: () => void;
  dbEncPassword: string;
  setDbEncPassword: React.Dispatch<React.SetStateAction<string>>;
  dbEncLoading: boolean;
  dbEncError: string | null;
  onSubmit: (e: React.FormEvent) => void;
}

export const DbEncryptionModal: React.FC<DbEncryptionModalProps> = ({
  onClose,
  dbEncPassword,
  setDbEncPassword,
  dbEncLoading,
  dbEncError,
  onSubmit,
}) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Database className="w-6 h-6" />}
      iconClassName="p-2.5 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 shrink-0"
      title={t('settings.dbEncModalTitle')}
      subtitle={t('settings.dbEncModalSubtitle')}
    >
      <div className="p-6 space-y-4">
        <div className="p-3 bg-amber-950/40 border border-amber-500/30 rounded-lg text-xs text-amber-300 leading-relaxed space-y-1.5">
          <p className="flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
            <span>{t('settings.dbEncWarnPassword')}</span>
          </p>
          <p className="pl-6">{t('settings.dbEncWarnRestart')}</p>
        </div>

        <form onSubmit={onSubmit} className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('settings.dbEncCurrentMP')}
            </label>
            <Input
              type="password"
              value={dbEncPassword}
              onChange={(e) => setDbEncPassword(e.target.value)}
              placeholder="••••••••••••"
              required
              autoFocus
              className="font-mono"
            />
          </div>

          {dbEncError && (
            <div className="p-2.5 bg-rose-950/40 border border-rose-500/30 rounded-lg text-xs text-rose-300 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
              <span>{dbEncError}</span>
            </div>
          )}

          <div className="flex items-center justify-end gap-2 pt-1">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onClose}
              disabled={dbEncLoading}
            >
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={dbEncLoading}
              loading={dbEncLoading}
              icon={<Lock className="w-3.5 h-3.5" />}
            >
              {dbEncLoading ? t('settings.encrypting') : t('settings.dbEncCreate')}
            </Button>
          </div>
        </form>
      </div>
    </ModalShell>
  );
};