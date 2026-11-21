import React, { useState, useEffect } from 'react';
import { Lock, AlertTriangle, Check } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { Button } from '../ui';
import { ChmodRemoteEntry } from '../../../wailsjs/go/main/App';
import { useTranslation } from '../../i18n/LanguageContext';
import { getErrorMessage } from '../../utils/errors';

interface ChmodModalProps {
  isOpen: boolean;
  entryPath: string;
  currentPermissions: string; // e.g. "0755" or "-rwxr-xr-x" or "drwxr-xr-x"
  selectedServerID: string;
  onClose: () => void;
  onChmodSaved: () => void;
}

export const ChmodModal: React.FC<ChmodModalProps> = ({
  isOpen,
  entryPath,
  currentPermissions,
  selectedServerID,
  onClose,
  onChmodSaved,
}) => {
  const { t } = useTranslation();

  // State for 9 bits: Owner r/w/x, Group r/w/x, Others r/w/x
  const [ownerR, setOwnerR] = useState(true);
  const [ownerW, setOwnerW] = useState(true);
  const [ownerX, setOwnerX] = useState(false);

  const [groupR, setGroupR] = useState(true);
  const [groupW, setGroupW] = useState(false);
  const [groupX, setGroupX] = useState(false);

  const [othersR, setOthersR] = useState(true);
  const [othersW, setOthersW] = useState(false);
  const [othersX, setOthersX] = useState(false);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Initialize from currentPermissions
  useEffect(() => {
    if (isOpen) {
      setError(null);
      setIsSubmitting(false);

      const p = currentPermissions.trim();
      // Check if numeric (e.g. "0755" or "755" or "644")
      if (/^0?[0-7]{3}$/.test(p)) {
        const numStr = p.length === 4 ? p.slice(1) : p;
        const o = parseInt(numStr[0], 8);
        const g = parseInt(numStr[1], 8);
        const ot = parseInt(numStr[2], 8);

        setOwnerR((o & 4) !== 0);
        setOwnerW((o & 2) !== 0);
        setOwnerX((o & 1) !== 0);

        setGroupR((g & 4) !== 0);
        setGroupW((g & 2) !== 0);
        setGroupX((g & 1) !== 0);

        setOthersR((ot & 4) !== 0);
        setOthersW((ot & 2) !== 0);
        setOthersX((ot & 1) !== 0);
      } else if (p.length >= 9) {
        // Symbolic like "-rwxr-xr-x" or "rwxr-xr-x"
        const s = p.length === 10 ? p.slice(1) : p;
        setOwnerR(s[0] === 'r');
        setOwnerW(s[1] === 'w');
        setOwnerX(s[2] === 'x');

        setGroupR(s[3] === 'r');
        setGroupW(s[4] === 'w');
        setGroupX(s[5] === 'x');

        setOthersR(s[6] === 'r');
        setOthersW(s[7] === 'w');
        setOthersX(s[8] === 'x');
      }
    }
  }, [isOpen, currentPermissions]);

  if (!isOpen) return null;

  const getOctalDigit = (r: boolean, w: boolean, x: boolean) => {
    let d = 0;
    if (r) d += 4;
    if (w) d += 2;
    if (x) d += 1;
    return d;
  };

  const octalMode = `0${getOctalDigit(ownerR, ownerW, ownerX)}${getOctalDigit(
    groupR,
    groupW,
    groupX
  )}${getOctalDigit(othersR, othersW, othersX)}`;

  const applyPreset = (mode: string) => {
    const o = parseInt(mode[1], 8);
    const g = parseInt(mode[2], 8);
    const ot = parseInt(mode[3], 8);

    setOwnerR((o & 4) !== 0);
    setOwnerW((o & 2) !== 0);
    setOwnerX((o & 1) !== 0);

    setGroupR((g & 4) !== 0);
    setGroupW((g & 2) !== 0);
    setGroupX((g & 1) !== 0);

    setOthersR((ot & 4) !== 0);
    setOthersW((ot & 2) !== 0);
    setOthersX((ot & 1) !== 0);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      await ChmodRemoteEntry({
        server_id: selectedServerID,
        path: entryPath,
        mode: octalMode,
      });
      onChmodSaved();
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Lock className="w-5 h-5 text-amber-400" />}
      iconClassName="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400 shrink-0"
      title={t('filemanager.chmodTitle')}
      subtitle={entryPath}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-5">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg flex items-start gap-2.5 text-xs text-rose-300">
            <AlertTriangle className="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        {/* Numeric and Symbolic Preview */}
        <div className="flex items-center justify-between p-3.5 bg-slate-950 border border-slate-800 rounded-xl">
          <span className="text-xs text-slate-400">{t('filemanager.calculatedMode')}</span>
          <div className="flex items-center gap-3">
            <span className="font-mono text-base font-bold text-amber-400">{octalMode}</span>
            <span className="font-mono text-xs text-slate-400 bg-slate-900 px-2 py-0.5 rounded border border-slate-800">
              {ownerR ? 'r' : '-'}
              {ownerW ? 'w' : '-'}
              {ownerX ? 'x' : '-'}
              {groupR ? 'r' : '-'}
              {groupW ? 'w' : '-'}
              {groupX ? 'x' : '-'}
              {othersR ? 'r' : '-'}
              {othersW ? 'w' : '-'}
              {othersX ? 'x' : '-'}
            </span>
          </div>
        </div>

        {/* Quick Presets */}
        <div>
          <label className="block text-xs font-medium text-slate-400 mb-2">
            {t('filemanager.chmodPresetsLabel')}
          </label>
          <div className="flex flex-wrap gap-1.5">
            {[
              { label: '0644 (File)', mode: '0644' },
              { label: '0755 (Exec/Dir)', mode: '0755' },
              { label: '0600 (Key/Secret)', mode: '0600' },
              { label: '0700 (Private Dir)', mode: '0700' },
              { label: '0777 (Full)', mode: '0777' },
            ].map((preset) => (
              <button
                key={preset.mode}
                type="button"
                onClick={() => applyPreset(preset.mode)}
                className={`px-2.5 py-1 text-xs rounded-lg border transition ${
                  octalMode === preset.mode
                    ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 font-semibold'
                    : 'bg-slate-900 border-slate-800 text-slate-300 hover:bg-slate-800'
                }`}
              >
                {preset.label}
              </button>
            ))}
          </div>
        </div>

        {/* Checkbox Grid */}
        <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-slate-400 border-b border-slate-800/80 pb-2">
                <th className="text-left font-medium pb-2">{t('filemanager.userCategory')}</th>
                <th className="text-center font-medium pb-2">{t('filemanager.readPerm')} (r)</th>
                <th className="text-center font-medium pb-2">{t('filemanager.writePerm')} (w)</th>
                <th className="text-center font-medium pb-2">{t('filemanager.execPerm')} (x)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/40">
              <tr>
                <td className="py-2.5 font-medium text-white">{t('filemanager.ownerUser')}</td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={ownerR}
                    onChange={(e) => setOwnerR(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={ownerW}
                    onChange={(e) => setOwnerW(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={ownerX}
                    onChange={(e) => setOwnerX(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
              </tr>
              <tr>
                <td className="py-2.5 font-medium text-white">{t('filemanager.groupUser')}</td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={groupR}
                    onChange={(e) => setGroupR(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={groupW}
                    onChange={(e) => setGroupW(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={groupX}
                    onChange={(e) => setGroupX(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
              </tr>
              <tr>
                <td className="py-2.5 font-medium text-white">{t('filemanager.othersUser')}</td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={othersR}
                    onChange={(e) => setOthersR(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={othersW}
                    onChange={(e) => setOthersW(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
                <td className="text-center">
                  <input
                    type="checkbox"
                    checked={othersX}
                    onChange={(e) => setOthersX(e.target.checked)}
                    className="rounded bg-slate-900 border-slate-700 text-amber-500 focus:ring-amber-500/20"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={onClose}
            disabled={isSubmitting}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={isSubmitting}
            loading={isSubmitting}
            icon={<Check className="w-4 h-4" />}
            className="bg-amber-600 hover:bg-amber-500 text-white shadow-amber-600/20"
          >
            {isSubmitting ? t('filemanager.saving') : t('filemanager.applyChmodBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
