import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import { CheckCircle2, AlertTriangle, Layers, Save, Folder, Globe, Power } from 'lucide-react';
import { db } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { usePrivacy } from '../context/PrivacyContext';
import { Button, Input, Select, Badge } from './ui';
import {
  SwitchSitePHPVersion,
  UpdateSiteSettings,
  ToggleSiteStatus,
  InspectPHPVersions,
} from '../../wailsjs/go/main/App';

interface SiteOverviewTabProps {
  site: db.SiteRecord;
  serverID: string;
  onSiteChange: React.Dispatch<React.SetStateAction<db.SiteRecord>>;
  onSiteUpdated: () => void;
}

export const SiteOverviewTab: React.FC<SiteOverviewTabProps> = ({ site, serverID, onSiteChange, onSiteUpdated }) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const { isPrivacyMode, maskPath, maskDomain } = usePrivacy();
  const [docRoot, setDocRoot] = useState(site.document_root);
  const [aliases, setAliases] = useState((site.aliases || []).join(', '));
  const [selectedPHP, setSelectedPHP] = useState(site.php_version || '8.2');
  const [installedPHPs, setInstalledPHPs] = useState<string[]>([]);
  const [isSavingSettings, setIsSavingSettings] = useState(false);
  const [isSwitchingPHP, setIsSwitchingPHP] = useState(false);
  const [isTogglingStatus, setIsTogglingStatus] = useState(false);
  const [settingsSuccess, setSettingsSuccess] = useState<string | null>(null);
  const [settingsError, setSettingsError] = useState<string | null>(null);

  useEffect(() => {
    setDocRoot(site.document_root);
    setAliases((site.aliases || []).join(', '));
    setSelectedPHP(site.php_version || '8.2');

    InspectPHPVersions(serverID)
      .then((vers) => {
        if (vers && vers.length > 0) {
          const list = vers.map((v: any) => v.version);
          setInstalledPHPs(list);
          if (!list.includes(site.php_version)) {
            setInstalledPHPs((prev) => [site.php_version, ...prev]);
          }
        } else {
          setInstalledPHPs(['7.2', '7.3', '7.4', '8.0', '8.1', '8.2', '8.3', '8.4']);
        }
      })
      .catch(() => {
        setInstalledPHPs(['7.2', '7.3', '7.4', '8.0', '8.1', '8.2', '8.3', '8.4']);
      });
  }, [site.id, serverID]);

  const handleSwitchPHP = async () => {
    setIsSwitchingPHP(true);
    setSettingsError(null);
    setSettingsSuccess(null);
    try {
      await SwitchSitePHPVersion(site.id, selectedPHP);
      setSettingsSuccess(t('manageSite.phpSwitched', { version: selectedPHP }));
      onSiteChange((prev) => db.SiteRecord.createFrom({ ...prev, php_version: selectedPHP }));
      setTimeout(() => setSettingsSuccess(null), 5000);
      onSiteUpdated();
    } catch (err: unknown) {
      setSettingsError(getErrorMessage(err));
    } finally {
      setIsSwitchingPHP(false);
    }
  };

  const handleSaveGeneralSettings = async () => {
    setIsSavingSettings(true);
    setSettingsError(null);
    setSettingsSuccess(null);
    try {
      const aliasList = aliases
        .split(',')
        .map((a) => a.trim())
        .filter((a) => a.length > 0);
      await UpdateSiteSettings(site.id, docRoot, aliasList);
      setSettingsSuccess(t('manageSite.paramsSaved'));
      onSiteChange((prev) => db.SiteRecord.createFrom({ ...prev, document_root: docRoot, aliases: aliasList }));
      setTimeout(() => setSettingsSuccess(null), 5000);
      onSiteUpdated();
    } catch (err: unknown) {
      setSettingsError(getErrorMessage(err));
    } finally {
      setIsSavingSettings(false);
    }
  };

  const handleToggleStatus = async () => {
    const newStatus = site.status !== 'active';
    if (!newStatus && !(await confirm(t('manageSite.pauseSiteConfirm', { domain: site.domain })))) {
      return;
    }
    setIsTogglingStatus(true);
    setSettingsError(null);
    try {
      await ToggleSiteStatus(site.id, newStatus);
      onSiteChange((prev) => db.SiteRecord.createFrom({ ...prev, status: newStatus ? 'active' : 'disabled' }));
      onSiteUpdated();
    } catch (err: unknown) {
      setSettingsError(getErrorMessage(err));
    } finally {
      setIsTogglingStatus(false);
    }
  };

  return (
    <div className="space-y-6 w-full">
      {settingsSuccess && (
        <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl text-xs text-emerald-300 flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
          <span>{settingsSuccess}</span>
        </div>
      )}
      {settingsError && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/30 rounded-xl text-xs text-rose-300 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 shrink-0 text-rose-400" />
          <span>{settingsError}</span>
        </div>
      )}

      {/* PHP Version Switcher Section */}
      <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-5 space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-purple-300 flex items-center gap-2">
              <Layers className="w-4 h-4" />
              <span>{t('manageSite.phpCardTitle')}</span>
            </h3>
            <p className="text-[11px] text-slate-400 mt-0.5">
              {t('manageSite.phpCardDesc')}
            </p>
          </div>
          <Badge
            variant={
              site.php_version
                ? 'brand'
                : site.site_type === 'static' || site.site_type === 'reverse_proxy'
                ? 'neutral'
                : 'warning'
            }
            className="font-mono"
          >
            {site.php_version
              ? t('manageSite.currentPHP', { version: site.php_version })
              : site.site_type === 'static' || site.site_type === 'reverse_proxy'
              ? t('manageSite.phpNotApplicable')
              : t('manageSite.phpGlobalDefault')}
          </Badge>
        </div>

        <div className="flex items-center gap-3 pt-2">
          <div className="flex-1">
            <Select
              value={selectedPHP}
              onChange={(e) => setSelectedPHP(e.target.value)}
            >
              {installedPHPs.map((v) => (
                <option key={v} value={v}>
                  PHP {v} {v === site.php_version ? `(${t('common.status')})` : ''}
                </option>
              ))}
            </Select>
          </div>

          <Button
            onClick={handleSwitchPHP}
            disabled={isSwitchingPHP || (Boolean(site.php_version) && selectedPHP === site.php_version)}
            loading={isSwitchingPHP}
            icon={!isSwitchingPHP ? <Save className="w-3.5 h-3.5" /> : undefined}
            variant="primary"
            size="md"
            className="whitespace-nowrap"
          >
            {site.php_version ? t('manageSite.switchPHP') : `PHP ${selectedPHP}`}
          </Button>
        </div>
      </div>

      {/* General Settings: DocumentRoot & Aliases */}
      <div className="space-y-4">
        <div>
          <label className="text-xs font-bold text-slate-300 block mb-1.5 flex items-center gap-1.5">
            <Folder className="w-3.5 h-3.5 text-indigo-400" />
            <span>{t('manageSite.docRootTitle')}</span>
          </label>
          <Input
            type="text"
            value={isPrivacyMode ? maskPath(docRoot) : docRoot}
            onChange={(e) => setDocRoot(e.target.value)}
            readOnly={isPrivacyMode}
            placeholder="/var/www/site/public"
            className="font-mono"
          />
          <p className="text-[11px] text-slate-500 mt-1">
            {t('manageSite.docRootHint')}
          </p>
        </div>

        <div>
          <label className="text-xs font-bold text-slate-300 block mb-1.5 flex items-center gap-1.5">
            <Globe className="w-3.5 h-3.5 text-indigo-400" />
            <span>{t('manageSite.aliasesTitle')}</span>
          </label>
          <Input
            type="text"
            value={
              isPrivacyMode
                ? aliases
                    .split(',')
                    .map((a) => a.trim())
                    .filter(Boolean)
                    .map(maskDomain)
                    .join(', ')
                : aliases
            }
            onChange={(e) => setAliases(e.target.value)}
            readOnly={isPrivacyMode}
            placeholder="www.example.com, stage.example.com"
            className="font-mono"
          />
          <p className="text-[11px] text-slate-500 mt-1">
            {t('manageSite.aliasesHint')}
          </p>
        </div>

        <div className="flex items-center justify-between pt-4 border-t border-slate-800">
          <Button
            onClick={handleToggleStatus}
            disabled={isTogglingStatus}
            variant={site.status === 'active' ? 'danger' : 'success'}
            size="md"
            icon={<Power className="w-3.5 h-3.5" />}
          >
            {site.status === 'active' ? t('manageSite.pauseSiteBtn') : t('manageSite.activateSiteBtn')}
          </Button>

          <Button
            onClick={handleSaveGeneralSettings}
            disabled={isSavingSettings || isPrivacyMode}
            loading={isSavingSettings}
            variant="primary"
            size="md"
            icon={!isSavingSettings ? <Save className="w-3.5 h-3.5" /> : undefined}
          >
            {t('manageSite.saveParamsBtn')}
          </Button>
        </div>
      </div>
    </div>
  );
};
