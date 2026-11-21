import React, { useState, useEffect } from 'react';
import { php } from '../../../wailsjs/go/models';
import {
  GetPHPSettings,
  SavePHPSettings,
  GetPHPIni,
  SavePHPIni,
  ListPHPIniBackups,
  RestorePHPIniBackup,
} from '../../../wailsjs/go/main/App';
import { getErrorMessage } from '../../utils/errors';
import { useTranslation } from '../../i18n/LanguageContext';
import { useConfirm } from '../../context/ConfirmContext';
import { useAlert } from '../../context/AlertContext';
import {
  Sliders,
  FileCode,
  History,
  Save,
  RotateCcw,
  Loader2,
  AlertCircle,
  CheckCircle2,
  HardDrive,
} from 'lucide-react';
import { CodeEditor } from '../common/CodeEditor';
import { Button, Input, Tabs } from '../ui';

interface PhpSettingsTabProps {
  serverId: string;
  version: string;
}

export const PhpSettingsTab: React.FC<PhpSettingsTabProps> = ({ serverId, version }) => {
  const { t } = useTranslation();
  const confirm = useConfirm();
  const alert = useAlert();

  const [target, setTarget] = useState<'fpm' | 'cli'>('fpm');
  const [activeSubTab, setActiveSubTab] = useState<'visual' | 'raw' | 'backups'>('visual');

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Settings State
  const [settings, setSettings] = useState<php.PHPSettings | null>(null);
  const [rawContent, setRawContent] = useState('');
  const [originalRawContent, setOriginalRawContent] = useState('');

  // Form Fields
  const [memoryLimit, setMemoryLimit] = useState('256M');
  const [uploadMax, setUploadMax] = useState('64M');
  const [postMax, setPostMax] = useState('64M');
  const [maxExecTime, setMaxExecTime] = useState(60);
  const [maxInputTime, setMaxInputTime] = useState(60);
  const [maxInputVars, setMaxInputVars] = useState(1000);
  const [timezone, setTimezone] = useState('UTC');
  const [opcacheEnable, setOpcacheEnable] = useState(true);
  const [opcacheMemory, setOpcacheMemory] = useState('128M');
  const [displayErrors, setDisplayErrors] = useState(false);
  const [errorReporting, setErrorReporting] = useState('E_ALL & ~E_DEPRECATED & ~E_STRICT');

  // Backups
  const [backups, setBackups] = useState<php.PHPIniBackupItem[]>([]);
  const [loadingBackups, setLoadingBackups] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null);

  const loadSettings = async () => {
    if (!serverId || !version) return;
    setLoading(true);
    setError(null);
    setSuccess(null);
    try {
      const data = await GetPHPSettings(serverId, version, target);
      setSettings(data);
      setMemoryLimit(data.memory_limit || '256M');
      setUploadMax(data.upload_max_filesize || '64M');
      setPostMax(data.post_max_size || '64M');
      setMaxExecTime(data.max_execution_time || 60);
      setMaxInputTime(data.max_input_time || 60);
      setMaxInputVars(data.max_input_vars || 1000);
      setTimezone(data.date_timezone || 'UTC');
      setOpcacheEnable(data.opcache_enable);
      setOpcacheMemory(String(data.opcache_memory_consumption || 128));
      setDisplayErrors(data.display_errors);
      setErrorReporting(data.error_reporting || 'E_ALL & ~E_DEPRECATED & ~E_STRICT');
      const raw = await GetPHPIni(serverId, version);
      setRawContent(raw || '');
      setOriginalRawContent(raw || '');
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const loadBackups = async () => {
    if (!serverId || !version) return;
    setLoadingBackups(true);
    try {
      const list = await ListPHPIniBackups(serverId, version, target);
      setBackups(list || []);
    } catch (err: unknown) {
      console.error(err);
    } finally {
      setLoadingBackups(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, [serverId, version, target]);

  useEffect(() => {
    if (activeSubTab === 'backups') {
      loadBackups();
    }
  }, [activeSubTab, serverId, version, target]);

  const handleSaveVisual = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const updated = new php.PHPSettings({
        memory_limit: memoryLimit.trim(),
        upload_max_filesize: uploadMax.trim(),
        post_max_size: postMax.trim(),
        max_execution_time: Number(maxExecTime) || 60,
        max_input_time: Number(maxInputTime) || 60,
        max_input_vars: Number(maxInputVars) || 1000,
        date_timezone: timezone.trim(),
        opcache_enable: opcacheEnable,
        opcache_memory_consumption: Number(opcacheMemory) || 128,
        display_errors: displayErrors,
        error_reporting: errorReporting.trim(),
        log_errors: true,
        opcache_max_accelerated_files: 10000,
        opcache_revalidate_freq: 2,
        short_open_tag: false,
        expose_php: false,
      });
      await SavePHPSettings(serverId, version, target, updated);
      setSuccess(t('php.settingsSavedSuccess') || 'Конфигурация успешно сохранена и проверена');
      await loadSettings();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleSaveRaw = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      await SavePHPIni(serverId, version, rawContent);
      setSuccess(t('php.rawIniSavedSuccess') || 'Файл php.ini успешно сохранен, создан бэкап');
      await loadSettings();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const handleRestoreBackup = async (fileName: string) => {
    if (!(await confirm(t('php.confirmRestoreBackup') || `Восстановить php.ini из резервной копии ${fileName}?`))) {
      return;
    }
    setRestoringBackup(fileName);
    try {
      await RestorePHPIniBackup(serverId, version, target, fileName);
      alert(t('php.backupRestored') || 'Конфигурация успешно восстановлена');
      await loadSettings();
      await loadBackups();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setRestoringBackup(null);
    }
  };

  return (
    <div className="space-y-6">
      {/* Target & SubTabs Header */}
      <div className="flex flex-wrap items-center justify-between gap-4 bg-surface-850 p-4 rounded-xl border border-slate-800">
        <div className="flex items-center gap-3">
          <span className="text-xs text-slate-400 font-medium">{t('php.configTarget') || 'Конфигурация для'}:</span>
          <Tabs
            variant="pill"
            size="sm"
            activeTab={target}
            onChange={(id) => setTarget(id as 'fpm' | 'cli')}
            tabs={[
              { id: 'fpm', label: 'PHP-FPM' },
              { id: 'cli', label: 'CLI' },
            ]}
          />
          <span className="text-[11px] font-mono text-slate-500 hidden sm:inline">
            /etc/php/{version}/{target}/php.ini
          </span>
        </div>

        <Tabs
          variant="pill"
          size="sm"
          activeTab={activeSubTab}
          onChange={(id) => setActiveSubTab(id as any)}
          tabs={[
            {
              id: 'visual',
              label: t('php.visualEditor') || 'Основные параметры',
              icon: <Sliders className="w-3.5 h-3.5" />,
            },
            {
              id: 'raw',
              label: t('php.rawEditor') || 'Редактор php.ini',
              icon: <FileCode className="w-3.5 h-3.5" />,
            },
            {
              id: 'backups',
              label: t('php.backups') || 'Резервные копии',
              icon: <History className="w-3.5 h-3.5" />,
            },
          ]}
        />
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-3">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {success && (
        <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center gap-3">
          <CheckCircle2 className="w-5 h-5 shrink-0" />
          <span>{success}</span>
        </div>
      )}

      {loading ? (
        <div className="flex flex-col items-center justify-center p-12 text-slate-400">
          <Loader2 className="w-8 h-8 animate-spin mb-2 text-indigo-400" />
          <p className="text-xs">{t('common.loadingShort')}</p>
        </div>
      ) : activeSubTab === 'visual' ? (
        <form onSubmit={handleSaveVisual} className="bg-surface-850 p-6 rounded-xl border border-slate-800 space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Memory & Execution */}
            <div className="space-y-4">
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <span>{t('php.resourceLimits') || 'Ресурсы и таймауты'}</span>
              </h4>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">memory_limit</label>
                <Input
                  type="text"
                  value={memoryLimit}
                  onChange={(e) => setMemoryLimit(e.target.value)}
                  placeholder="256M"
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">max_execution_time (sec)</label>
                <Input
                  type="number"
                  value={maxExecTime}
                  onChange={(e) => setMaxExecTime(Number(e.target.value))}
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">max_input_time (sec)</label>
                <Input
                  type="number"
                  value={maxInputTime}
                  onChange={(e) => setMaxInputTime(Number(e.target.value))}
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">max_input_vars</label>
                <Input
                  type="number"
                  value={maxInputVars}
                  onChange={(e) => setMaxInputVars(Number(e.target.value))}
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
            </div>

            {/* Uploads & Localization */}
            <div className="space-y-4">
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <span>{t('php.uploadsAndTimezone') || 'Загрузка файлов и Timezone'}</span>
              </h4>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">upload_max_filesize</label>
                <Input
                  type="text"
                  value={uploadMax}
                  onChange={(e) => setUploadMax(e.target.value)}
                  placeholder="64M"
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">post_max_size</label>
                <Input
                  type="text"
                  value={postMax}
                  onChange={(e) => setPostMax(e.target.value)}
                  placeholder="64M"
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">date.timezone</label>
                <Input
                  type="text"
                  value={timezone}
                  onChange={(e) => setTimezone(e.target.value)}
                  placeholder="UTC or Europe/Moscow"
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
            </div>

            {/* OPcache & Debugging */}
            <div className="space-y-4">
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-2">
                <span>OPcache & Debugging</span>
              </h4>
              <label className="flex items-center gap-3 p-3 rounded-lg bg-slate-900 border border-slate-800 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={opcacheEnable}
                  onChange={(e) => setOpcacheEnable(e.target.checked)}
                  className="w-4 h-4 rounded border-slate-700 bg-slate-950 text-indigo-600 focus:ring-0 cursor-pointer"
                />
                <div>
                  <span className="text-xs font-semibold text-white block">opcache.enable</span>
                  <span className="text-[11px] text-slate-400">{t('php.opcacheDesc') || 'Кэширование байткода'}</span>
                </div>
              </label>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">opcache.memory_consumption</label>
                <Input
                  type="text"
                  value={opcacheMemory}
                  onChange={(e) => setOpcacheMemory(e.target.value)}
                  placeholder="128M"
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>

              <label className="flex items-center gap-3 p-3 rounded-lg bg-slate-900 border border-slate-800 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={displayErrors}
                  onChange={(e) => setDisplayErrors(e.target.checked)}
                  className="w-4 h-4 rounded border-slate-700 bg-slate-950 text-indigo-600 focus:ring-0 cursor-pointer"
                />
                <div>
                  <span className="text-xs font-semibold text-white block">display_errors</span>
                  <span className="text-[11px] text-slate-400">{t('php.displayErrorsDesc') || 'Показывать ошибки в браузере (off в production)'}</span>
                </div>
              </label>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">error_reporting</label>
                <Input
                  type="text"
                  value={errorReporting}
                  onChange={(e) => setErrorReporting(e.target.value)}
                  sizeVariant="sm"
                  className="font-mono"
                />
              </div>
            </div>
          </div>

          <div className="flex items-center justify-between pt-4 border-t border-slate-800">
            <span className="text-xs text-slate-500">
              {t('php.safeConfigHint') || 'Перед применением создается автоматический бэкап и выполняется проверка конфигурации.'}
            </span>
            <Button
              type="submit"
              disabled={saving}
              loading={saving}
              variant="primary"
              size="md"
              icon={!saving ? <Save className="w-4 h-4" /> : undefined}
            >
              {saving ? (t('common.saving') || 'Сохранение...') : (t('common.saveChanges') || 'Применить изменения')}
            </Button>
          </div>
        </form>
      ) : activeSubTab === 'raw' ? (
        <div className="bg-surface-850 p-6 rounded-2xl border border-slate-800 space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-slate-300 font-mono flex items-center gap-2">
              <FileCode className="w-4 h-4 text-indigo-400" />
              /etc/php/{version}/{target}/php.ini
            </span>
            <Button
              onClick={handleSaveRaw}
              disabled={saving || rawContent === originalRawContent}
              loading={saving}
              variant="primary"
              size="md"
              icon={!saving ? <Save className="w-4 h-4" /> : undefined}
            >
              {saving ? t('common.saving') : t('common.save')}
            </Button>
          </div>
          <div className="h-[540px] rounded-xl overflow-hidden border border-slate-800 bg-slate-950 flex flex-col">
            <CodeEditor
              value={rawContent}
              onChange={setRawContent}
              language="ini"
              filePath={`/etc/php/${version}/${target}/php.ini`}
              onSave={handleSaveRaw}
              className="flex-1 min-h-0"
            />
          </div>
        </div>
      ) : (
        <div className="bg-surface-850 p-6 rounded-xl border border-slate-800 space-y-4">
          <div className="flex items-center justify-between mb-2">
            <h4 className="text-xs font-bold text-white uppercase tracking-wider">
              {t('php.availableBackups') || 'Резервные копии php.ini'}
            </h4>
            <Button
              onClick={loadBackups}
              disabled={loadingBackups}
              variant="secondary"
              size="sm"
            >
              {t('common.refresh') || 'Обновить'}
            </Button>
          </div>

          {loadingBackups ? (
            <div className="flex items-center justify-center p-8 text-slate-400">
              <Loader2 className="w-6 h-6 animate-spin mr-2 text-indigo-400" />
              <span className="text-xs">{t('common.loadingShort')}</span>
            </div>
          ) : backups.length === 0 ? (
            <div className="text-center p-8 text-xs text-slate-500">
              {t('php.noBackupsFound') || 'Резервные копии не найдены. Они создаются автоматически при каждом изменении.'}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-slate-800 text-slate-400">
                    <th className="py-2.5 px-3">{t('php.backupFile') || 'Файл'}</th>
                    <th className="py-2.5 px-3">{t('php.backupSize') || 'Размер'}</th>
                    <th className="py-2.5 px-3">{t('php.backupDate') || 'Дата создания'}</th>
                    <th className="py-2.5 px-3 text-right">{t('common.actions') || 'Действия'}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-mono">
                  {backups.map((b) => (
                    <tr key={b.filename} className="hover:bg-slate-900/50">
                      <td className="py-3 px-3 text-white font-semibold flex items-center gap-2">
                        <HardDrive className="w-4 h-4 text-indigo-400 shrink-0" />
                        <span>{b.filename}</span>
                      </td>
                      <td className="py-3 px-3 text-slate-400">{(b.size_bytes / 1024).toFixed(1)} KB</td>
                      <td className="py-3 px-3 text-slate-400">{b.created_at || b.timestamp}</td>
                      <td className="py-3 px-3 text-right">
                        <Button
                          onClick={() => handleRestoreBackup(b.filename)}
                          disabled={restoringBackup === b.filename}
                          loading={restoringBackup === b.filename}
                          variant="secondary"
                          size="sm"
                          icon={restoringBackup !== b.filename ? <RotateCcw className="w-3.5 h-3.5" /> : undefined}
                        >
                          {t('php.restore') || 'Восстановить'}
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
