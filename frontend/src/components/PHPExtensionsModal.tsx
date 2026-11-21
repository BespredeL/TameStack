import React, { useState } from 'react';
import { Download } from 'lucide-react';
import { TogglePHPExtension } from '../../wailsjs/go/main/App';
import { php } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button, Badge } from './ui';

interface PHPExtensionsModalProps {
  isOpen: boolean;
  serverID: string;
  version: string;
  extensions: php.PHPExtensionInfo[];
  onClose: () => void;
  onRefresh: () => void;
  onTaskStarted: (task: any) => void;
  onInstallExtensions: () => void;
}

export const PHPExtensionsModal: React.FC<PHPExtensionsModalProps> = ({
  isOpen,
  serverID,
  version,
  extensions,
  onClose,
  onRefresh,
  onTaskStarted,
  onInstallExtensions,
}) => {
  const { t } = useTranslation();
  const [loadingExt, setLoadingExt] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleToggle = async (ext: php.PHPExtensionInfo) => {
    setLoadingExt(ext.name);
    try {
      const task = await TogglePHPExtension(serverID, version, ext.name, !ext.is_enabled);
      onTaskStarted(task);
      onClose();
    } catch (err) {
      console.error('Failed to toggle extension:', err);
    } finally {
      setLoadingExt(null);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="xl"
      className="max-h-[85vh]"
      title={t('php.extensionsModal.title', { version })}
      subtitle={t('php.extensionsModal.subtitle')}
    >
      {/* Install missing extensions */}
      <div className="px-6 pt-4 pb-3 border-b border-slate-800 flex items-center justify-between gap-3">
        <span className="text-xs text-slate-400">{t('php.extensionsModal.installHint')}</span>
        <Button
          variant="primary"
          size="sm"
          onClick={onInstallExtensions}
          icon={<Download className="w-3.5 h-3.5" />}
        >
          {t('php.extensionsModal.installBtn')}
        </Button>
      </div>

      {/* Extensions List */}
      <div className="flex-1 overflow-y-auto p-6 divide-y divide-slate-800/80">
        {extensions.map((ext) => {
          const isLoading = loadingExt === ext.name;
          return (
            <div key={ext.name} className="py-3 flex items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-semibold text-white">{ext.name}</span>
                  <Badge variant={ext.is_enabled ? 'success' : 'neutral'} size="sm">
                    {ext.is_enabled
                      ? t('php.extensionsModal.activeBadge')
                      : t('php.extensionsModal.disabledBadge')}
                  </Badge>
                </div>
                <p className="text-xs text-slate-400 mt-0.5">{ext.description}</p>
              </div>

              <Button
                variant={ext.is_enabled ? 'danger' : 'primary'}
                size="sm"
                disabled={isLoading}
                loading={isLoading}
                onClick={() => handleToggle(ext)}
              >
                {ext.is_enabled
                  ? t('php.extensionsModal.disableBtn')
                  : t('php.extensionsModal.enableBtn')}
              </Button>
            </div>
          );
        })}
      </div>

      {/* Footer */}
      <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <span className="text-xs text-slate-400">
          {t('php.extensionsModal.restartNotice', { version })}
        </span>
        <Button
          variant="secondary"
          size="sm"
          onClick={onClose}
        >
          {t('common.close')}
        </Button>
      </div>
    </ModalShell>
  );
};
