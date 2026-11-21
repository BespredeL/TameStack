import React from 'react';
import { Shield, Plus, Trash2 } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { security } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { Input, Button } from '../ui';

interface ExclusionsModalProps {
  onClose: () => void;
  exclusions: security.FileSecurityExclusion[];
  newExclusionPattern: string;
  setNewExclusionPattern: React.Dispatch<React.SetStateAction<string>>;
  newExclusionReason: string;
  setNewExclusionReason: React.Dispatch<React.SetStateAction<string>>;
  isAddingExclusion: boolean;
  onAddCustom: (e: React.FormEvent) => void;
  onDeleteExclusion: (id: string) => void;
}

export const ExclusionsModal: React.FC<ExclusionsModalProps> = ({
  onClose,
  exclusions,
  newExclusionPattern,
  setNewExclusionPattern,
  newExclusionReason,
  setNewExclusionReason,
  isAddingExclusion,
  onAddCustom,
  onDeleteExclusion,
}) => {
  const { t } = useTranslation();

  return (
    <ModalShell
      onClose={onClose}
      size="3xl"
      className="max-h-[85vh]"
      icon={<Shield className="w-5 h-5 text-emerald-400" />}
      iconClassName=""
      title={t('fileScanner.exclusionsModalTitle')}
      subtitle={t('fileScanner.exclusionsSubtitle')}
    >
      <div className="p-6 overflow-y-auto space-y-6 flex-1">
        {/* Add form */}
        <form onSubmit={onAddCustom} className="p-4 bg-slate-950/60 rounded-xl border border-slate-800 space-y-3">
          <h4 className="text-xs font-bold text-slate-200">{t('fileScanner.addExclusionHeading')}</h4>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.exclusionPatternLabel')}</label>
              <Input
                type="text"
                sizeVariant="sm"
                placeholder="*.cache, */wp-content/uploads/cache/*"
                value={newExclusionPattern}
                onChange={(e) => setNewExclusionPattern(e.target.value)}
                className="font-mono"
                required
              />
            </div>
            <div>
              <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.exclusionReasonLabel')}</label>
              <Input
                type="text"
                sizeVariant="sm"
                placeholder={t('fileScanner.exclusionReasonPlaceholder')}
                value={newExclusionReason}
                onChange={(e) => setNewExclusionReason(e.target.value)}
              />
            </div>
          </div>
          <Button
            type="submit"
            variant="success"
            size="sm"
            loading={isAddingExclusion}
            disabled={!newExclusionPattern.trim()}
            icon={<Plus className="w-3.5 h-3.5" />}
          >
            {t('fileScanner.addExclusionSubmit')}
          </Button>
        </form>

        {/* Exclusions List */}
        <div className="space-y-2">
          <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">
            {t('fileScanner.activeExclusionsHeader', { count: exclusions.length })}
          </h4>
          {exclusions.length === 0 ? (
            <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">
              {t('fileScanner.emptyExclusionsText')}
            </div>
          ) : (
            <div className="divide-y divide-slate-800 border border-slate-800 rounded-xl bg-slate-950 overflow-hidden">
              {exclusions.map((exc) => (
                <div key={exc.id} className="p-3 flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <span className="font-mono text-xs font-semibold text-white block truncate">
                      {exc.pattern}
                    </span>
                    {exc.reason && (
                      <span className="text-[11px] text-slate-400 block mt-0.5">
                        {exc.reason}
                      </span>
                    )}
                  </div>
                  <button
                    onClick={() => onDeleteExclusion(exc.id)}
                    className="p-1.5 rounded text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition cursor-pointer"
                    title={t('fileScanner.deleteExclusionTooltip')}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="px-6 py-3 border-t border-slate-800 bg-slate-950 flex items-center justify-between text-xs">
        <span className="text-slate-400">
          {t('fileScanner.exclusionsCountFooter', { count: exclusions.length })}
        </span>
        <Button
          variant="secondary"
          size="sm"
          onClick={onClose}
        >
          {t('fileScanner.closeWindowBtn')}
        </Button>
      </div>
    </ModalShell>
  );
};
