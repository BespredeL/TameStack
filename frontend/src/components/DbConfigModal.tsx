import React, { useEffect, useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Save, Loader2, FileCode, Check, ShieldCheck } from 'lucide-react';
import { GetDatabaseConfig, SaveDatabaseConfig } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { CodeEditor } from './common/CodeEditor';
import { Button } from './ui';

interface DbConfigModalProps {
  serverID: string;
  engine: string;
  filePath: string;
  onClose: () => void;
  onSaved?: () => void;
}

export const DbConfigModal: React.FC<DbConfigModalProps> = ({
  serverID,
  engine,
  filePath,
  onClose,
  onSaved,
}) => {
  const { t } = useTranslation();
  const [content, setContent] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isSavedSuccess, setIsSavedSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    GetDatabaseConfig(serverID, filePath)
      .then((res) => {
        if (!cancelled) setContent(res);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(getErrorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [serverID, filePath]);

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);
    try {
      await SaveDatabaseConfig(serverID, engine, filePath, content);
      setIsSavedSuccess(true);
      setTimeout(() => setIsSavedSuccess(false), 2500);
      onSaved?.();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="5xl"
      className="h-[85vh]"
      icon={<FileCode className="w-5 h-5" />}
      title={<span className="font-mono">{filePath}</span>}
      subtitle={t('databases.configEditor.subtitle', { engine: engine.toUpperCase() })}
      headerActions={
        <Button
          variant="primary"
          size="sm"
          onClick={handleSave}
          loading={isSaving}
          disabled={isLoading}
          icon={isSavedSuccess ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Save className="w-3.5 h-3.5" />}
        >
          {isSavedSuccess ? t('databases.configEditor.saved') : t('databases.configEditor.save')}
        </Button>
      }
    >
      {/* Top safety notice banner - edge-to-edge, fills void with no border radius */}
      <div className="px-6 py-2.5 bg-amber-500/10 border-b border-amber-500/20 text-xs text-amber-200/90 flex items-center gap-2.5 shrink-0">
        <ShieldCheck className="w-4 h-4 text-amber-400 shrink-0" />
        <span className="leading-relaxed">{t('databases.configEditor.safetyHint')}</span>
      </div>

      {error && (
        <div className="px-6 py-2.5 bg-rose-500/15 border-b border-rose-500/30 text-xs text-rose-300 whitespace-pre-wrap shrink-0">
          {error}
        </div>
      )}

      <div className="flex-1 p-3 bg-slate-950 flex flex-col overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center h-full text-slate-500 gap-2">
            <Loader2 className="w-5 h-5 animate-spin" />
            {t('databases.configEditor.loading')}
          </div>
        ) : (
          <CodeEditor
            value={content}
            onChange={setContent}
            language="ini"
            filePath={filePath}
            onSave={handleSave}
          />
        )}
      </div>
    </ModalShell>
  );
};
