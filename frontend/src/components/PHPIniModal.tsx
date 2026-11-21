import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Save, AlertCircle, CheckCircle2, ShieldAlert } from 'lucide-react';
import { GetPHPIni, SavePHPIni } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { CodeEditor } from './common/CodeEditor';
import { Button } from './ui';

interface PHPIniModalProps {
  isOpen: boolean;
  serverID: string;
  version: string;
  onClose: () => void;
}

export const PHPIniModal: React.FC<PHPIniModalProps> = ({
  isOpen,
  serverID,
  version,
  onClose,
}) => {
  const { t } = useTranslation();
  const [content, setContent] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setIsLoading(true);
    setStatusMessage(null);

    GetPHPIni(serverID, version)
      .then((data) => setContent(data))
      .catch((err) => {
        setStatusMessage({ type: 'error', text: t('php.iniModal.readError', { error: getErrorMessage(err) }) });
      })
      .finally(() => setIsLoading(false));
  }, [isOpen, serverID, version]);

  if (!isOpen) return null;

  const handleSave = async () => {
    setIsSaving(true);
    setStatusMessage(null);
    try {
      await SavePHPIni(serverID, version, content);
      setStatusMessage({
        type: 'success',
        text: t('php.iniModal.saveSuccess', { version }),
      });
    } catch (err: unknown) {
      setStatusMessage({
        type: 'error',
        text: t('php.iniModal.saveError', { error: getErrorMessage(err) }),
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[90vh]"
      title={t('php.iniModal.title', { version })}
      subtitle={<span className="font-mono">/etc/php/{version}/fpm/php.ini</span>}
    >
      {/* Status Message */}
      {statusMessage && (
        <div
          className={`px-6 py-2.5 text-xs flex items-center gap-2 border-b ${
            statusMessage.type === 'success'
              ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-300'
              : 'bg-rose-500/10 border-rose-500/20 text-rose-300'
          }`}
        >
          {statusMessage.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0" />
          ) : (
            <AlertCircle className="w-4 h-4 shrink-0" />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Code Editor Area */}
      <div className="flex-1 p-3 bg-slate-950 flex flex-col min-h-0">
        {isLoading ? (
          <div className="flex-1 flex items-center justify-center gap-2 text-slate-400 text-xs">
            <span>{t('php.iniModal.loadingConfig')}</span>
          </div>
        ) : (
          <CodeEditor
            value={content}
            onChange={setContent}
            language="ini"
            filePath="php.ini"
            onSave={handleSave}
          />
        )}
      </div>

      {/* Footer */}
      <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-slate-400 text-xs">
          <ShieldAlert className="w-4 h-4 text-amber-400" />
          <span>{t('php.iniModal.safetyNotice')}</span>
        </div>

        <div className="flex items-center gap-3">
          <Button
            variant="secondary"
            size="md"
            onClick={onClose}
          >
            {t('common.close')}
          </Button>
          <Button
            variant="primary"
            size="md"
            onClick={handleSave}
            loading={isSaving}
            disabled={isLoading}
            icon={<Save className="w-3.5 h-3.5" />}
          >
            {t('php.iniModal.verifyAndSave')}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
};
