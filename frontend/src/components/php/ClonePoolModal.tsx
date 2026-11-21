import React, { useState } from 'react';
import { ClonePHPPool } from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { Button, Input } from '../ui';
import { getErrorMessage } from '../../utils/errors';
import { useTranslation } from '../../i18n/LanguageContext';
import { Copy, AlertCircle } from 'lucide-react';

interface ClonePoolModalProps {
  serverId: string;
  version: string;
  sourcePoolName: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const ClonePoolModal: React.FC<ClonePoolModalProps> = ({
  serverId,
  version,
  sourcePoolName,
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [newPoolName, setNewPoolName] = useState(`${sourcePoolName}_copy`);
  const [cloning, setCloning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPoolName.trim()) return;
    setCloning(true);
    setError(null);
    try {
      await ClonePHPPool(serverId, version, sourcePoolName, newPoolName.trim().toLowerCase());
      onSuccess();
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setCloning(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      title={`${t('php.clonePoolTitle')}: ${sourcePoolName}`}
      subtitle={`PHP ${version}-FPM`}
      icon={<Copy className="w-5 h-5 text-indigo-400" />}
      size="md"
    >
      <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
        <div className="p-6 space-y-4 flex-1">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('php.sourcePool')}
            </label>
            <div className="px-4 py-2.5 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-400 font-mono">
              {sourcePoolName} (PHP {version})
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('php.newPoolName')} *
            </label>
            <Input
              type="text"
              required
              value={newPoolName}
              onChange={(e) => setNewPoolName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
              placeholder="e.g.: myapp_staging"
              className="font-mono"
            />
            <p className="text-[11px] text-slate-500 mt-1">
              {t('php.newPoolHelp')}
            </p>
          </div>
        </div>

        {/* Modal Footer */}
        <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-end gap-3 shrink-0">
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={onClose}
            disabled={cloning}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={cloning || !newPoolName.trim()}
            loading={cloning}
          >
            {cloning ? t('php.cloningPool') : t('php.cloneBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
