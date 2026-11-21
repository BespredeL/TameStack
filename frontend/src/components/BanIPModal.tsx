import React, { useState } from 'react';
import { Ban, AlertCircle } from 'lucide-react';
import { ServerRecord } from '../types';
import { BanIP } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button, Input, Select } from './ui';

interface BanIPModalProps {
  isOpen: boolean;
  onClose: () => void;
  server: ServerRecord;
  jails: string[];
  initialJail?: string;
  onBanned: (task: any) => void;
}

export const BanIPModal: React.FC<BanIPModalProps> = ({
  isOpen,
  onClose,
  server,
  jails,
  initialJail,
  onBanned,
}) => {
  const { t } = useTranslation();
  const [jail, setJail] = useState(initialJail || (jails.length > 0 ? jails[0] : 'sshd'));
  const [ip, setIp] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const cleanIP = ip.trim();
    if (!cleanIP) {
      setError(t('security.banModal.ipRequired'));
      return;
    }

    // Basic IP validation regex (IPv4)
    const ipv4Regex = /^((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}$/;
    if (!ipv4Regex.test(cleanIP) && !cleanIP.includes(':')) {
      setError(t('security.banModal.invalidIp'));
      return;
    }

    // Safety check: Prevent banning localhost
    if (cleanIP === '127.0.0.1' || cleanIP === '::1' || cleanIP === 'localhost') {
      setError(t('security.banModal.localhostBlocked'));
      return;
    }

    try {
      setIsSubmitting(true);
      const task = await BanIP({
        server_id: server.id,
        jail: jail || 'sshd',
        ip: cleanIP,
      } as any);

      onBanned(task);
      onClose();
    } catch (err: unknown) {
      setError(t('security.banModal.banError', { error: err?.toString() || '' }));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Ban className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0"
      title={t('security.banModal.title')}
    >

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {error && (
            <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg text-rose-400 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              {t('security.banModal.jailLabel')}
            </label>
            {jails.length > 0 ? (
              <Select
                value={jail}
                onChange={(e) => setJail(e.target.value)}
              >
                {jails.map((j) => (
                  <option key={j} value={j} className="bg-slate-900 text-white">
                    {j}
                  </option>
                ))}
              </Select>
            ) : (
              <Input
                type="text"
                value={jail}
                onChange={(e) => setJail(e.target.value)}
                placeholder="sshd"
              />
            )}
            <p className="text-[11px] text-slate-500 mt-1">
              {t('security.banModal.jailDesc')}
            </p>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              {t('security.banModal.ipLabel')}
            </label>
            <Input
              type="text"
              value={ip}
              onChange={(e) => setIp(e.target.value)}
              placeholder={t('security.banModal.ipPlaceholder')}
              className="font-mono"
              autoFocus
            />
          </div>

          <div className="pt-2 flex justify-end gap-3 border-t border-slate-800">
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
              variant="danger"
              size="md"
              disabled={isSubmitting}
              loading={isSubmitting}
              icon={<Ban className="w-3.5 h-3.5" />}
            >
              {isSubmitting ? t('security.banModal.banning') : t('security.banModal.banBtn')}
            </Button>
          </div>
        </form>
    </ModalShell>
  );
};
