import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Shield, Sparkles } from 'lucide-react';
import { db } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { usePrivacy } from '../context/PrivacyContext';
import { Button, Input } from './ui';
import { IssueSSLCertificate } from '../../wailsjs/go/main/App';

interface SiteSSLTabProps {
  site: db.SiteRecord;
  serverID: string;
  onClose: () => void;
  onOpenTask?: (task: any) => void;
}

export const SiteSSLTab: React.FC<SiteSSLTabProps> = ({ site, serverID, onClose, onOpenTask }) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const { isPrivacyMode, maskDomain } = usePrivacy();
  const [isIssuingSSL, setIsIssuingSSL] = useState(false);
  const [sslEmail, setSslEmail] = useState('admin@' + site.domain);

  const handleIssueSSL = async () => {
    if (!(await confirm(t('manageSite.issueSSLConfirm', { domain: site.domain })))) {
      return;
    }
    setIsIssuingSSL(true);
    try {
      const task = await IssueSSLCertificate({
        server_id: serverID,
        site_id: site.id,
        primary_domain: site.domain,
        san_domains: site.aliases || [],
        email: sslEmail,
        auto_renew: true,
      } as any);
      if (task && onOpenTask) {
        onOpenTask(task);
        onClose();
      }
    } catch (err: unknown) {
      alert(t('manageSite.issueSSLError', { error: getErrorMessage(err) }));
    } finally {
      setIsIssuingSSL(false);
    }
  };

  return (
    <div className="space-y-6 w-full">
      <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className={`w-10 h-10 rounded-xl flex items-center justify-center border ${
                site.ssl_enabled
                  ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                  : 'bg-slate-800 text-slate-400 border-slate-700'
              }`}
            >
              <Shield className="w-5 h-5" />
            </div>
            <div>
              <h4 className="text-xs font-bold text-white uppercase tracking-wider">
                {site.ssl_enabled ? (t('sites.sslSecured')) : (t('sites.sslMissing'))}
              </h4>
              <p className="text-[11px] text-slate-400">
                {site.ssl_enabled
                  ? `Provider: ${site.ssl_provider || "Let's Encrypt"}`
                  : 'HTTP (Port 80)'}
              </p>
            </div>
          </div>
        </div>

        {!site.ssl_enabled && (
          <div className="pt-4 border-t border-slate-800 space-y-3">
            <div>
              <label className="text-xs font-medium text-slate-300 block mb-1">
                Email:
              </label>
              <Input
                type="email"
                value={isPrivacyMode ? `admin@${maskDomain(site.domain)}` : sslEmail}
                onChange={(e) => setSslEmail(e.target.value)}
                readOnly={isPrivacyMode}
                placeholder="admin@domain.com"
                className="font-mono"
              />
            </div>

            <Button
              onClick={handleIssueSSL}
              disabled={isIssuingSSL}
              loading={isIssuingSSL}
              variant="success"
              size="md"
              icon={!isIssuingSSL ? <Sparkles className="w-3.5 h-3.5" /> : undefined}
            >
              {t('manageSite.issueSSLBtn')}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
};
