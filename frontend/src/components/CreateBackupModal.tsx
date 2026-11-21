import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import {
  Archive,
  Globe,
  Database,
  Settings,
  Shield,
  Server,
  Info,
} from 'lucide-react';
import { CreateBackup, ListSites, ListDatabases } from '../../wailsjs/go/main/App';
import { db, database } from '../../wailsjs/go/models';
import { TaskRecord, ServerRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Select, Button, Badge } from './ui';

interface CreateBackupModalProps {
  serverID: string;
  servers: ServerRecord[];
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
  initialType?: 'site' | 'database' | 'config' | 'full';
  initialTargetName?: string;
}

const formatBytes = (bytes: number): string => {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
};

export const CreateBackupModal: React.FC<CreateBackupModalProps> = ({
  serverID,
  servers,
  onClose,
  onTaskCreated,
  initialType = 'site',
  initialTargetName = '',
}) => {
  const { t } = useTranslation();
  const [currentServerID, setCurrentServerID] = useState<string>(serverID);
  const [backupType, setBackupType] = useState<'site' | 'database' | 'config' | 'full'>(initialType);

  // Lists fetched from server
  const [sites, setSites] = useState<db.SiteRecord[]>([]);
  const [databases, setDatabases] = useState<database.DatabaseItem[]>([]);
  const [isLoadingSites, setIsLoadingSites] = useState(false);
  const [isLoadingDatabases, setIsLoadingDatabases] = useState(false);

  // Target selection
  // '__all__' = all, '__custom__' = manual input, or specific domain / db name
  const [targetSelection, setTargetSelection] = useState<string>(initialTargetName || '__all__');
  const [customTargetName, setCustomTargetName] = useState<string>(initialTargetName || '');

  // Destination & Path
  const [destination, setDestination] = useState<string>('local');
  const [targetPath, setTargetPath] = useState<string>('/var/backups/tamestack');
  const [retentionDays, setRetentionDays] = useState<number>(14);

  // Linked DB dump for site backup
  const [includeDatabase, setIncludeDatabase] = useState(false);
  const [selectedDatabaseName, setSelectedDatabaseName] = useState('');

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const currentServer = servers.find((s) => s.id === currentServerID) || servers[0];

  // Load sites for the selected server
  const loadSites = async (sID: string) => {
    setIsLoadingSites(true);
    try {
      const res = await ListSites(sID);
      const list = res || [];
      setSites(list);
      // If we don't have an initialTargetName, keep __all__ or pick first
      if (!initialTargetName && targetSelection !== '__all__' && targetSelection !== '__custom__') {
        if (!list.some((s) => s.domain === targetSelection)) {
          setTargetSelection('__all__');
        }
      }
    } catch (e) {
      console.warn('Failed to load sites for backup modal:', e);
      setSites([]);
    } finally {
      setIsLoadingSites(false);
    }
  };

  // Load databases for the selected server
  const loadDatabases = async (sID: string) => {
    setIsLoadingDatabases(true);
    try {
      const res = await ListDatabases(sID, 'mysql');
      const list = res || [];
      setDatabases(list);
      if (!initialTargetName && targetSelection !== '__all__' && targetSelection !== '__custom__') {
        if (!list.some((d) => d.name === targetSelection)) {
          setTargetSelection('__all__');
        }
      }
    } catch (e) {
      console.warn('Failed to load databases for backup modal:', e);
      setDatabases([]);
    } finally {
      setIsLoadingDatabases(false);
    }
  };

  useEffect(() => {
    if (currentServerID) {
      loadSites(currentServerID);
      loadDatabases(currentServerID);
    }
  }, [currentServerID]);

  // Match database name to site
  useEffect(() => {
    if (backupType === 'site' && targetSelection !== '__all__' && targetSelection !== '__custom__') {
      const clean = targetSelection.replace(/[^a-zA-Z0-9_]/g, '_').toLowerCase();
      const match = databases.find((d) => {
        const name = d.name.toLowerCase();
        return name === clean || clean.startsWith(name) || name.startsWith(clean.slice(0, 5));
      });
      if (match) {
        setSelectedDatabaseName(match.name);
      }
    }
  }, [targetSelection, backupType, databases]);

  // Handle server switch
  const handleServerChange = (newServerID: string) => {
    setCurrentServerID(newServerID);
    setTargetSelection('__all__');
    setCustomTargetName('');
  };

  // Handle type change
  const handleTypeChange = (type: 'site' | 'database' | 'config' | 'full') => {
    setBackupType(type);
    setError(null);
    if (type === 'config' || type === 'full') {
      setTargetSelection('__all__');
    } else {
      setTargetSelection('__all__');
      setCustomTargetName('');
    }
  };

  const selectedSite = sites.find((s) => s.domain === targetSelection);
  const selectedDb = databases.find((d) => d.name === targetSelection);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    let finalTargetName = 'all';

    if (backupType === 'site') {
      if (targetSelection === '__custom__') {
        if (!customTargetName.trim()) {
          setError(t('backups.createModal.targetDomainRequired'));
          return;
        }
        finalTargetName = customTargetName.trim();
      } else if (targetSelection === '__all__') {
        finalTargetName = 'all';
      } else {
        finalTargetName = targetSelection;
      }
    } else if (backupType === 'database') {
      if (targetSelection === '__custom__') {
        if (!customTargetName.trim()) {
          setError(t('backups.createModal.dbNameRequired'));
          return;
        }
        finalTargetName = customTargetName.trim();
      } else if (targetSelection === '__all__') {
        finalTargetName = 'all';
      } else {
        finalTargetName = targetSelection;
      }
    } else {
      finalTargetName = 'all';
    }

    try {
      setIsSubmitting(true);
      setError(null);

      const task = await CreateBackup({
        server_id: currentServerID,
        backup_type: backupType,
        target_name: finalTargetName,
        destination,
        target_path: targetPath.trim() || '/var/backups/tamestack',
        retention_days: Number(retentionDays),
        include_database: backupType === 'site' && includeDatabase && !!selectedDatabaseName,
        database_name: selectedDatabaseName,
      });

      onTaskCreated(task as any);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="xl"
      className="max-h-[92vh]"
      icon={<Archive className="w-5 h-5" />}
      title={t('backups.createModal.title')}
      subtitle={t('backups.createModal.subtitle')}
    >
      {/* Content */}
      <form onSubmit={handleSubmit} className="p-6 space-y-4 overflow-y-auto min-h-0 flex-1">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400 flex items-start gap-2">
            <span className="shrink-0 mt-0.5">⚠️</span>
            <span>{error}</span>
          </div>
        )}

        {/* Server Selector if multiple servers exist */}
        {servers.length > 1 ? (
          <div className="flex items-center justify-between p-3 bg-slate-950/80 border border-slate-800 rounded-xl">
            <div className="flex items-center gap-2 text-xs font-semibold text-slate-300">
              <Server className="w-4 h-4 text-indigo-400" />
              <span>{t('backups.createModal.serverLabel')}</span>
            </div>
            <div className="w-48">
              <Select
                sizeVariant="sm"
                value={currentServerID}
                onChange={(e) => handleServerChange(e.target.value)}
              >
                {servers.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.host})
                  </option>
                ))}
              </Select>
            </div>
          </div>
        ) : currentServer ? (
          <div className="flex items-center justify-between px-3.5 py-2.5 bg-slate-950/50 border border-slate-800/80 rounded-xl text-xs">
            <div className="flex items-center gap-2 text-slate-400">
              <Server className="w-3.5 h-3.5 text-indigo-400" />
              <span>{t('backups.createModal.targetServerLabel')}</span>
            </div>
            <span className="font-semibold text-slate-200">
              {currentServer.name} ({currentServer.host})
            </span>
          </div>
        ) : null}

        {/* Type Selector (4 Cards) */}
        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-2">
            {t('backups.createModal.backupTypeHeading')}
          </label>
          <div className="grid grid-cols-2 gap-2.5">
            <button
              type="button"
              onClick={() => handleTypeChange('site')}
              className={`p-3 rounded-xl border text-left flex items-start gap-3 transition-all cursor-pointer ${
                backupType === 'site'
                  ? 'bg-indigo-600/10 border-indigo-500 text-white shadow-sm shadow-indigo-500/10'
                  : 'bg-slate-950/80 border-slate-800 text-slate-400 hover:border-slate-700'
              }`}
            >
              <div
                className={`p-2 rounded-lg ${
                  backupType === 'site' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-900 text-slate-400'
                }`}
              >
                <Globe className="w-4 h-4 shrink-0" />
              </div>
              <div>
                <div className="text-xs font-bold text-white">{t('backups.createModal.typeSiteFiles')}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">{t('backups.createModal.typeSiteFilesDesc')}</div>
              </div>
            </button>

            <button
              type="button"
              onClick={() => handleTypeChange('database')}
              className={`p-3 rounded-xl border text-left flex items-start gap-3 transition-all cursor-pointer ${
                backupType === 'database'
                  ? 'bg-indigo-600/10 border-indigo-500 text-white shadow-sm shadow-indigo-500/10'
                  : 'bg-slate-950/80 border-slate-800 text-slate-400 hover:border-slate-700'
              }`}
            >
              <div
                className={`p-2 rounded-lg ${
                  backupType === 'database' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-900 text-slate-400'
                }`}
              >
                <Database className="w-4 h-4 shrink-0" />
              </div>
              <div>
                <div className="text-xs font-bold text-white">{t('backups.createModal.typeDatabase')}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">{t('backups.createModal.typeDatabaseDesc')}</div>
              </div>
            </button>

            <button
              type="button"
              onClick={() => handleTypeChange('config')}
              className={`p-3 rounded-xl border text-left flex items-start gap-3 transition-all cursor-pointer ${
                backupType === 'config'
                  ? 'bg-indigo-600/10 border-indigo-500 text-white shadow-sm shadow-indigo-500/10'
                  : 'bg-slate-950/80 border-slate-800 text-slate-400 hover:border-slate-700'
              }`}
            >
              <div
                className={`p-2 rounded-lg ${
                  backupType === 'config' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-900 text-slate-400'
                }`}
              >
                <Settings className="w-4 h-4 shrink-0" />
              </div>
              <div>
                <div className="text-xs font-bold text-white">{t('backups.createModal.typeConfigs')}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">Nginx, Apache, PHP, MySQL</div>
              </div>
            </button>

            <button
              type="button"
              onClick={() => handleTypeChange('full')}
              className={`p-3 rounded-xl border text-left flex items-start gap-3 transition-all cursor-pointer ${
                backupType === 'full'
                  ? 'bg-indigo-600/10 border-indigo-500 text-white shadow-sm shadow-indigo-500/10'
                  : 'bg-slate-950/80 border-slate-800 text-slate-400 hover:border-slate-700'
              }`}
            >
              <div
                className={`p-2 rounded-lg ${
                  backupType === 'full' ? 'bg-indigo-500/20 text-indigo-400' : 'bg-slate-900 text-slate-400'
                }`}
              >
                <Shield className="w-4 h-4 shrink-0" />
              </div>
              <div>
                <div className="text-xs font-bold text-white">{t('backups.createModal.typeFullNode')}</div>
                <div className="text-[10px] text-slate-400 mt-0.5">{t('backups.createModal.typeFullNodeDesc')}</div>
              </div>
            </button>
          </div>
        </div>

        {/* Dynamic Target Selection for Site */}
        {backupType === 'site' && (
          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <label className="block text-xs font-semibold text-slate-300">
                {t('backups.createModal.selectSiteLabel')}
              </label>
              {isLoadingSites && (
                <span className="text-[11px] text-indigo-400">
                  {t('backups.createModal.loadingSites')}
                </span>
              )}
            </div>

            <Select
              value={targetSelection}
              onChange={(e) => setTargetSelection(e.target.value)}
            >
              <option value="__all__">{t('backups.createModal.allSitesOption')}</option>
              {sites.length > 0 && (
                <optgroup label={t('backups.createModal.registeredSitesGroup')}>
                  {sites.map((s) => (
                    <option key={s.id} value={s.domain}>
                      {s.domain}
                    </option>
                  ))}
                </optgroup>
              )}
              <option value="__custom__">{t('backups.createModal.customSiteOption')}</option>
            </Select>

            {/* Informative details badge */}
            {targetSelection === '__all__' ? (
              <div className="p-3 bg-indigo-950/20 border border-indigo-500/20 rounded-xl text-xs text-indigo-200 flex items-start gap-2.5">
                <Info className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
                <div className="space-y-0.5 leading-relaxed">
                  <span className="font-semibold">{t('backups.createModal.allSitesWarning')}</span>
                  <p className="text-[11px] text-slate-400">
                    {t('backups.createModal.allSitesWarningDesc')}
                  </p>
                </div>
              </div>
            ) : selectedSite ? (
              <div className="p-3 bg-slate-950 border border-slate-800/80 rounded-xl text-xs flex items-center justify-between">
                <div className="space-y-0.5">
                  <div className="font-bold text-white flex items-center gap-1.5">
                    <Globe className="w-3.5 h-3.5 text-emerald-400" />
                    <span>{selectedSite.domain}</span>
                  </div>
                  <div className="text-[11px] text-slate-400 font-mono">
                    {selectedSite.document_root || `/var/www/${selectedSite.domain}`}
                  </div>
                </div>
                <div className="flex items-center gap-1.5">
                  {selectedSite.web_server && (
                    <Badge variant="neutral" size="sm">
                      {selectedSite.web_server}
                    </Badge>
                  )}
                  {selectedSite.php_version && (
                    <Badge variant="primary" size="sm">
                      PHP {selectedSite.php_version}
                    </Badge>
                  )}
                </div>
              </div>
            ) : null}

            {/* Single site backup with linked database dump */}
            {targetSelection !== '__all__' && (
              <div className="p-3 bg-slate-950 border border-slate-800/80 rounded-xl space-y-2 text-xs">
                <label className="flex items-center gap-2 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={includeDatabase}
                    onChange={(e) => setIncludeDatabase(e.target.checked)}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-900"
                  />
                  <span className="font-semibold text-slate-200 flex items-center gap-1.5">
                    <Database className="w-3.5 h-3.5 text-indigo-400" />
                    <span>{t('backups.createModal.includeDbDump')}</span>
                  </span>
                </label>
                {includeDatabase && (
                  <div className="pl-5 space-y-1.5 pt-1">
                    <span className="text-[11px] text-slate-400 block">
                      {t('backups.createModal.includeDbDumpDesc')}
                    </span>
                    <Select
                      sizeVariant="sm"
                      value={selectedDatabaseName}
                      onChange={(e) => setSelectedDatabaseName(e.target.value)}
                      className="font-mono"
                    >
                      <option value="">{t('backups.createModal.selectDbPlaceholder')}</option>
                      {databases.map((d) => (
                        <option key={d.name} value={d.name}>
                          {d.name} ({formatBytes(d.size_bytes || 0)})
                        </option>
                      ))}
                    </Select>
                  </div>
                )}
              </div>
            )}

            {/* Manual input if chosen */}
            {targetSelection === '__custom__' && (
              <div className="space-y-1">
                <Input
                  type="text"
                  required
                  autoFocus
                  placeholder={t('backups.createModal.customSitePlaceholder')}
                  value={customTargetName}
                  onChange={(e) => setCustomTargetName(e.target.value)}
                  className="font-mono border-indigo-500/60"
                />
                <p className="text-[10px] text-slate-500">
                  {t('backups.createModal.customSiteHint')}
                </p>
              </div>
            )}
          </div>
        )}

        {/* Dynamic Target Selection for Database */}
        {backupType === 'database' && (
          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <label className="block text-xs font-semibold text-slate-300">
                {t('backups.createModal.selectDbLabel')}
              </label>
              {isLoadingDatabases && (
                <span className="text-[11px] text-indigo-400">
                  {t('backups.createModal.queryingDbs')}
                </span>
              )}
            </div>

            <Select
              value={targetSelection}
              onChange={(e) => setTargetSelection(e.target.value)}
            >
              <option value="__all__">{t('backups.createModal.allDbsOption')}</option>
              {databases.length > 0 && (
                <optgroup label={t('backups.createModal.detectedDbsGroup')}>
                  {databases.map((d) => (
                    <option key={d.name} value={d.name}>
                      {d.name} {d.size_bytes > 0 ? `(${formatBytes(d.size_bytes)})` : ''}
                    </option>
                  ))}
                </optgroup>
              )}
              <option value="__custom__">{t('backups.createModal.customDbOption')}</option>
            </Select>

            {/* Informative details badge */}
            {targetSelection === '__all__' ? (
              <div className="p-3 bg-indigo-950/20 border border-indigo-500/20 rounded-xl text-xs text-indigo-200 flex items-start gap-2.5">
                <Info className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
                <div className="space-y-0.5 leading-relaxed">
                  <span className="font-semibold">{t('backups.createModal.allDbsWarning')}</span>
                  <p className="text-[11px] text-slate-400">
                    {t('backups.createModal.allDbsWarningDesc')}
                  </p>
                </div>
              </div>
            ) : selectedDb ? (
              <div className="p-3 bg-slate-950 border border-slate-800/80 rounded-xl text-xs flex items-center justify-between">
                <div className="space-y-0.5">
                  <div className="font-bold text-white flex items-center gap-1.5">
                    <Database className="w-3.5 h-3.5 text-indigo-400" />
                    <span>{selectedDb.name}</span>
                  </div>
                  <div className="text-[11px] text-slate-400">
                    {selectedDb.tables_count > 0 ? t('backups.createModal.tablesCount', { count: selectedDb.tables_count }) : t('backups.createModal.mysqlDb')}
                    {selectedDb.charset ? ` • ${selectedDb.charset}` : ''}
                  </div>
                </div>
                {selectedDb.size_bytes > 0 && (
                  <Badge variant="primary" size="sm">
                    {formatBytes(selectedDb.size_bytes)}
                  </Badge>
                )}
              </div>
            ) : null}

            {/* Manual input if chosen */}
            {targetSelection === '__custom__' && (
              <div className="space-y-1">
                <Input
                  type="text"
                  required
                  autoFocus
                  placeholder={t('backups.createModal.customDbPlaceholder')}
                  value={customTargetName}
                  onChange={(e) => setCustomTargetName(e.target.value)}
                  className="font-mono border-indigo-500/60"
                />
                <p className="text-[10px] text-slate-500">
                  {t('backups.createModal.customDbHint')}
                </p>
              </div>
            )}
          </div>
        )}

        {/* Config Backup Info Card */}
        {backupType === 'config' && (
          <div className="p-4 bg-slate-950/80 border border-slate-800 rounded-xl text-xs space-y-2">
            <div className="flex items-center gap-2 font-bold text-white">
              <Settings className="w-4 h-4 text-indigo-400" />
              <span>{t('backups.createModal.configBackupHeading')}</span>
            </div>
            <p className="text-slate-400 text-[11px] leading-relaxed">
              {t('backups.createModal.configBackupDesc')}
            </p>
            <div className="flex flex-wrap gap-1.5 pt-1">
              {['/etc/nginx', '/etc/apache2', '/etc/php', '/etc/mysql', '/etc/redis'].map((p) => (
                <span
                  key={p}
                  className="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-900 border border-slate-800 text-indigo-300"
                >
                  {p}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Full Server Backup Info Card */}
        {backupType === 'full' && (
          <div className="p-4 bg-slate-950/80 border border-slate-800 rounded-xl text-xs space-y-2">
            <div className="flex items-center gap-2 font-bold text-white">
              <Shield className="w-4 h-4 text-amber-400" />
              <span>{t('backups.createModal.fullBackupHeading')}</span>
            </div>
            <p className="text-slate-400 text-[11px] leading-relaxed">
              {t('backups.createModal.fullBackupDesc')}
            </p>
            <ul className="text-[11px] text-slate-300 space-y-1 list-disc list-inside">
              <li>{t('backups.createModal.fullBackupItem1')}</li>
              <li>{t('backups.createModal.fullBackupItem2')}</li>
              <li>{t('backups.createModal.fullBackupItem3')}</li>
            </ul>
          </div>
        )}

        {/* Target Directory on Server with Preset Buttons */}
        <div className="space-y-1.5 pt-1">
          <div className="flex items-center justify-between">
            <label className="block text-xs font-semibold text-slate-300">
              {t('backups.createModal.storagePathLabel')}
            </label>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => setTargetPath('/var/backups/tamestack')}
                className={`px-2 py-0.5 rounded text-[10px] transition cursor-pointer border ${
                  targetPath === '/var/backups/tamestack'
                    ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40 font-semibold'
                    : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                }`}
                title={t('backups.createModal.systemDirTooltip')}
              >
                📁 /var/backups/tamestack
              </button>
              <button
                type="button"
                onClick={() => setTargetPath('~/.tamestack/backups')}
                className={`px-2 py-0.5 rounded text-[10px] transition cursor-pointer border ${
                  targetPath === '~/.tamestack/backups'
                    ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40 font-semibold'
                    : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                }`}
                title={t('backups.createModal.userDirTooltip')}
              >
                🏠 ~/.tamestack/backups
              </button>
            </div>
          </div>

          <Input
            type="text"
            value={targetPath}
            onChange={(e) => setTargetPath(e.target.value)}
            placeholder={t('backups.createModal.pathPlaceholder')}
            className="font-mono"
          />
          <p className="text-[10px] text-slate-500">
            {t('backups.createModal.sudoHint')}
          </p>
        </div>

        {/* Retention & Destination */}
        <div className="grid grid-cols-2 gap-3 pt-1">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('backups.createModal.storageLocationLabel')}
            </label>
            <Select
              value={destination}
              onChange={(e) => setDestination(e.target.value)}
            >
              <option value="local">{t('backups.createModal.storageLocal')}</option>
            </Select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('backups.createModal.retentionPolicyLabel')}
            </label>
            <Select
              value={retentionDays}
              onChange={(e) => setRetentionDays(Number(e.target.value))}
            >
              <option value={7}>{t('backups.createModal.retention7')}</option>
              <option value={14}>{t('backups.createModal.retention14')}</option>
              <option value={30}>{t('backups.createModal.retention30')}</option>
              <option value={90}>{t('backups.createModal.retention90')}</option>
              <option value={0}>{t('backups.createModal.retention0')}</option>
            </Select>
          </div>
        </div>

        {/* Footer buttons */}
        <div className="pt-4 flex items-center justify-end gap-3 border-t border-slate-800">
          <Button
            variant="secondary"
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
            loading={isSubmitting}
            icon={<Archive className="w-4 h-4" />}
          >
            {isSubmitting ? t('backups.createModal.creatingTask') : t('backups.createModal.createBackupBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
