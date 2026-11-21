import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  ShieldCheck,
  Globe,
  Mail,
  AlertCircle,
  Lock,
} from 'lucide-react';
import { ServerRecord } from '../types';
import { IssueSSLCertificate, ListSites } from '../../wailsjs/go/main/App';
import { ssl, db } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Select, Button } from './ui';

interface IssueSSLModalProps {
  isOpen: boolean;
  servers: ServerRecord[];
  onClose: () => void;
  onTaskStarted: (task: any) => void;
}

export const IssueSSLModal: React.FC<IssueSSLModalProps> = ({
  isOpen,
  servers,
  onClose,
  onTaskStarted,
}) => {
  const { t } = useTranslation();
  const [serverID, setServerID] = useState<string>(servers[0]?.id || '');
  const [sites, setSites] = useState<db.SiteRecord[]>([]);
  const [selectedSiteID, setSelectedSiteID] = useState<string>('custom');
  const [primaryDomain, setPrimaryDomain] = useState('');
  const [sanDomainsInput, setSanDomainsInput] = useState('');
  const [email, setEmail] = useState('');
  const challengeType = 'http-01' as const;
  const [autoRenew, setAutoRenew] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (servers.length > 0 && !serverID) {
      setServerID(servers[0].id);
    }
  }, [servers]);

  useEffect(() => {
    if (serverID) {
      ListSites(serverID)
        .then((data) => setSites(data || []))
        .catch(console.error);
    }
  }, [serverID]);

  const handleSiteSelect = (siteID: string) => {
    setSelectedSiteID(siteID);
    if (siteID === 'custom') {
      setPrimaryDomain('');
      setSanDomainsInput('');
      setEmail('');
    } else {
      const s = sites.find((x) => x.id === siteID);
      if (s) {
        setPrimaryDomain(s.domain);
        setSanDomainsInput(s.aliases ? s.aliases.join(', ') : '');
        setEmail(`admin@${s.domain}`);
      }
    }
  };

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!primaryDomain) {
      setError(t('ssl.issueModal.domainRequired'));
      return;
    }

    setIsSubmitting(true);
    setError(null);

    const sanDomains = sanDomainsInput
      .split(',')
      .map((d) => d.trim().toLowerCase())
      .filter((d) => d.length > 0 && d !== primaryDomain.toLowerCase());

    const input = new ssl.IssueSSLInput({
      server_id: serverID,
      site_id: selectedSiteID === 'custom' ? '' : selectedSiteID,
      primary_domain: primaryDomain.trim().toLowerCase(),
      san_domains: sanDomains,
      email: email.trim(),
      challenge_type: challengeType,
      auto_renew: autoRenew,
    });

    try {
      const task = await IssueSSLCertificate(input);
      onTaskStarted(task);
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
      size="xl"
      className="max-h-[90vh]"
      icon={<ShieldCheck className="w-5 h-5" />}
      iconClassName="p-2 rounded-lg bg-emerald-600/20 text-emerald-400 border border-emerald-500/30 shrink-0"
      title={t('ssl.issueModal.title')}
      subtitle={t('ssl.issueModal.subtitle')}
    >
      {/* Form */}
      <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-4">
        {error && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('ssl.issueModal.targetServerLabel')}</label>
            <Select
              value={serverID}
              onChange={(e) => setServerID(e.target.value)}
            >
              {servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.host})
                </option>
              ))}
            </Select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('ssl.issueModal.bindSiteLabel')}</label>
            <Select
              value={selectedSiteID}
              onChange={(e) => handleSiteSelect(e.target.value)}
            >
              <option value="custom">{t('ssl.issueModal.customDomainOption')}</option>
              {sites.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.domain}
                </option>
              ))}
            </Select>
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1">{t('ssl.issueModal.domainLabel')}</label>
          <Input
            type="text"
            required
            icon={<Globe className="w-4 h-4" />}
            placeholder={t('ssl.issueModal.domainPlaceholder')}
            value={primaryDomain}
            onChange={(e) => setPrimaryDomain(e.target.value)}
            className="font-mono"
          />
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1">{t('ssl.issueModal.sansLabel')}</label>
          <Input
            type="text"
            placeholder={t('ssl.issueModal.sansPlaceholder')}
            value={sanDomainsInput}
            onChange={(e) => setSanDomainsInput(e.target.value)}
            className="font-mono"
          />
          <p className="text-[11px] text-slate-500 mt-1">{t('ssl.issueModal.sansHint')}</p>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1">{t('ssl.issueModal.contactEmailLabel')}</label>
          <Input
            type="email"
            required
            icon={<Mail className="w-4 h-4" />}
            placeholder="admin@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <p className="text-[11px] text-slate-500 mt-1">{t('ssl.issueModal.emailHint')}</p>
        </div>

        <div className="p-3 bg-slate-950 rounded-xl border border-slate-800 space-y-2">
          <span className="text-xs font-bold text-slate-300 block">{t('ssl.issueModal.checkHeading')}</span>
          <p className="text-[11px] text-slate-400">{t('ssl.issueModal.checkDesc')}</p>
        </div>

        <label className="flex items-center gap-2.5 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer">
          <input
            type="checkbox"
            checked={autoRenew}
            onChange={(e) => setAutoRenew(e.target.checked)}
            className="rounded border-slate-700 text-emerald-600 focus:ring-0"
          />
          <span className="text-xs text-slate-300 font-medium">
            {t('ssl.issueModal.autoRenewCheckbox')}
          </span>
        </label>
      </form>

      {/* Footer */}
      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <Button
          variant="secondary"
          size="md"
          onClick={onClose}
        >
          {t('common.cancel')}
        </Button>

        <Button
          variant="success"
          size="md"
          onClick={handleSubmit}
          loading={isSubmitting}
          icon={<Lock className="w-3.5 h-3.5" />}
        >
          {t('ssl.issueModal.issueBtn')}
        </Button>
      </div>
    </ModalShell>
  );
};
