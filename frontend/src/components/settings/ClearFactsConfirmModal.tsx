import React from 'react';
import { AlertTriangle, Trash2 } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { useTranslation } from '../../i18n/LanguageContext';
import { Button } from '../ui';

interface ClearFactsConfirmModalProps {
  onClose: () => void;
  clearingFacts: boolean;
  onConfirm: () => void;
}

export const ClearFactsConfirmModal: React.FC<ClearFactsConfirmModalProps> = ({ onClose, clearingFacts, onConfirm }) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<AlertTriangle className="w-6 h-6" />}
      iconClassName="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400 shrink-0"
      title={t('settings.clearFactsConfirmTitle')}
      subtitle={t('settings.clearFactsConfirmSubtitle')}
    >
      <div className="p-6 space-y-4">
        <p className="text-xs text-slate-300 leading-relaxed">
          {t('settings.clearFactsConfirmDesc')}
        </p>

        <div className="flex items-center justify-end gap-2 pt-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={onClose}
            disabled={clearingFacts}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={onConfirm}
            loading={clearingFacts}
            icon={<Trash2 className="w-3.5 h-3.5" />}
          >
            {clearingFacts ? t('settings.clearing') : t('settings.confirmClear')}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
};
