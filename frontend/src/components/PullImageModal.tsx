import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';

import { Download, Layers } from 'lucide-react';
import { DockerPullImage } from '../../wailsjs/go/main/App';
import { TaskRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Button } from './ui';

interface PullImageModalProps {
  serverID: string;
  onClose: () => void;
  onTaskCreated: (task: TaskRecord) => void;
}

export const PullImageModal: React.FC<PullImageModalProps> = ({
  serverID,
  onClose,
  onTaskCreated,
}) => {
  const { t } = useTranslation();
  const [image, setImage] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!image.trim()) {
      setError(t('docker.pullImageModal.imageRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const task = await DockerPullImage({
        server_id: serverID,
        image: image.trim(),
      });
      onTaskCreated(task as any);
      onClose();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
      setIsSubmitting(false);
    }
  };

  const quickPicks = ['nginx:alpine', 'redis:7-alpine', 'postgres:16-alpine', 'node:20-alpine', 'python:3.11-slim'];

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Download className="w-5 h-5" />}
      title={t('docker.pullImageModal.title')}
      subtitle={t('docker.pullImageModal.subtitle')}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
            <Layers className="w-3.5 h-3.5 text-slate-400" />
            {t('docker.pullImageModal.imageLabel')}
          </label>
          <Input
            type="text"
            required
            placeholder={t('docker.pullImageModal.imagePlaceholder')}
            value={image}
            onChange={(e) => setImage(e.target.value)}
            className="font-mono"
          />
        </div>

        <div>
          <span className="text-[11px] font-medium text-slate-400 block mb-2">{t('docker.pullImageModal.quickSelect')}</span>
          <div className="flex flex-wrap gap-1.5">
            {quickPicks.map((pick) => (
              <button
                type="button"
                key={pick}
                onClick={() => setImage(pick)}
                className="px-2.5 py-1 bg-slate-950 hover:bg-slate-800 border border-slate-800 rounded-lg text-[11px] font-mono text-slate-300 transition-colors cursor-pointer"
              >
                {pick}
              </button>
            ))}
          </div>
        </div>

        <div className="pt-4 flex justify-end gap-3 border-t border-slate-800">
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
            icon={<Download className="w-4 h-4" />}
          >
            {t('docker.pullImageModal.pullBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
