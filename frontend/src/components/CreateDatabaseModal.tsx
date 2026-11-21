import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';
import { Database } from 'lucide-react';
import { CreateDatabase } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { TaskRecord } from '../types';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';

interface CreateDatabaseModalProps {
  serverID: string;
  engine: string;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const CreateDatabaseModal: React.FC<CreateDatabaseModalProps> = ({
  serverID,
  engine,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [charset, setCharset] = useState(engine === 'postgres' ? 'UTF8' : 'utf8mb4');
  const [collation, setCollation] = useState(engine === 'postgres' ? '' : 'utf8mb4_unicode_ci');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError(t('databases.createModal.nameRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const task = await CreateDatabase({
        server_id: serverID,
        engine: engine as any,
        name: name.trim(),
        charset,
        collation,
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
      size="lg"
      icon={<Database className="w-5 h-5" />}
      title={t('databases.createModal.title')}
      subtitle={t('databases.createModal.dbmsLabel', { engine })}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5">
            {t('databases.createModal.nameLabel')}
          </label>
          <Input
            type="text"
            required
            placeholder={t('databases.createModal.namePlaceholder')}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('databases.createModal.charsetLabel')}
            </label>
            <Input
              type="text"
              value={charset}
              onChange={(e) => setCharset(e.target.value)}
              className="font-mono text-xs"
            />
          </div>

          {engine !== 'postgres' && (
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1.5">
                {t('databases.createModal.collationLabel')}
              </label>
              <Input
                type="text"
                value={collation}
                onChange={(e) => setCollation(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
          )}
        </div>

        <div className="pt-4 flex justify-end gap-3 border-t border-slate-800">
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
          >
            {isSubmitting ? t('databases.createModal.creating') : t('databases.createModal.createBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
