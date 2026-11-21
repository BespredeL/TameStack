import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';
import { UserPlus, Key } from 'lucide-react';
import { CreateDatabaseUser } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { TaskRecord } from '../types';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';

interface CreateDatabaseUserModalProps {
  serverID: string;
  engine: string;
  availableDatabases: string[];
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const CreateDatabaseUserModal: React.FC<CreateDatabaseUserModalProps> = ({
  serverID,
  engine,
  availableDatabases,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [host, setHost] = useState(engine === 'postgres' ? 'localhost' : '%');
  const [selectedDatabases, setSelectedDatabases] = useState<string[]>([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generatePassword = () => {
    const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+';
    let pass = '';
    for (let i = 0; i < 20; i++) {
      pass += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    setPassword(pass);
  };

  const toggleDatabase = (dbName: string) => {
    if (selectedDatabases.includes(dbName)) {
      setSelectedDatabases(selectedDatabases.filter((d) => d !== dbName));
    } else {
      setSelectedDatabases([...selectedDatabases, dbName]);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      setError(t('databases.createUserModal.userAndPassRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const task = await CreateDatabaseUser({
        server_id: serverID,
        engine: engine as any,
        username: username.trim(),
        password,
        host,
        databases: selectedDatabases,
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
      icon={<UserPlus className="w-5 h-5" />}
      title={t('databases.createUserModal.title')}
      subtitle={t('databases.createUserModal.dbmsLabel', { engine })}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('databases.createUserModal.userLabel')}
            </label>
            <Input
              type="text"
              required
              placeholder={t('databases.createUserModal.userPlaceholder')}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5">
              {t('databases.createUserModal.allowedHostLabel')}
            </label>
            <Input
              type="text"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              className="font-mono text-xs"
            />
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-1.5">
            <label className="text-xs font-semibold text-slate-300">
              {t('databases.createUserModal.passwordLabel')}
            </label>
            <button
              type="button"
              onClick={generatePassword}
              className="text-[11px] text-indigo-400 hover:text-indigo-300 flex items-center gap-1 font-medium cursor-pointer"
            >
              <Key className="w-3 h-3" />
              {t('databases.createUserModal.generateBtn')}
            </button>
          </div>
          <Input
            type="text"
            required
            placeholder={t('databases.createUserModal.passwordPlaceholder')}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="font-mono text-xs"
          />
        </div>

        {availableDatabases.length > 0 && (
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-2">
              {t('databases.createUserModal.grantAccessLabel')}
            </label>
            <div className="max-h-36 overflow-y-auto bg-slate-950/70 border border-slate-800 rounded-xl p-2.5 space-y-1.5 custom-scrollbar">
              {availableDatabases.map((dbName) => (
                <label
                  key={dbName}
                  className="flex items-center gap-2.5 p-1.5 hover:bg-slate-900 rounded-lg cursor-pointer text-xs text-slate-300"
                >
                  <input
                    type="checkbox"
                    checked={selectedDatabases.includes(dbName)}
                    onChange={() => toggleDatabase(dbName)}
                    className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-950"
                  />
                  <span className="font-mono text-white">{dbName}</span>
                </label>
              ))}
            </div>
          </div>
        )}

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
            {isSubmitting ? t('databases.createUserModal.creating') : t('databases.createUserModal.createBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
