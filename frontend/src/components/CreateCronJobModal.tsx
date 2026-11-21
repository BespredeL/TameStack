import React, { useState } from 'react';
import { getErrorMessage } from '../utils/errors';
import { Clock, Code, Terminal, User } from 'lucide-react';
import { SaveCronJob } from '../../wailsjs/go/main/App';
import { cron } from '../../wailsjs/go/models';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button, Input } from './ui';

interface CreateCronJobModalProps {
  serverID: string;
  initialJob?: cron.CronJob | null;
  onClose: () => void;
  onSaved: () => void;
}

export const CreateCronJobModal: React.FC<CreateCronJobModalProps> = ({
  serverID,
  initialJob,
  onClose,
  onSaved,
}) => {
  const { t } = useTranslation();
  const [expression, setExpression] = useState(initialJob?.expression || '* * * * *');
  const [command, setCommand] = useState(initialJob?.command || '');
  const [comment, setComment] = useState(initialJob?.comment || '');
  const [user, setUser] = useState(initialJob?.user || 'root');
  const [enabled, setEnabled] = useState(initialJob ? initialJob.enabled : true);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const presets = [
    { label: t('cron.createModal.presetEveryMinute'), expr: '* * * * *' },
    { label: t('cron.createModal.presetEvery5Min'), expr: '*/5 * * * *' },
    { label: t('cron.createModal.presetHourly'), expr: '0 * * * *' },
    { label: t('cron.createModal.presetDaily'), expr: '0 0 * * *' },
    { label: t('cron.createModal.presetWeekly'), expr: '0 0 * * 0' },
    { label: t('cron.createModal.presetMonthly'), expr: '0 0 1 * *' },
  ];

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!expression.trim() || !command.trim()) {
      setError(t('cron.createModal.scheduleAndCmdRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      await SaveCronJob({
        server_id: serverID,
        job: {
          id: initialJob?.id || '',
          expression: expression.trim(),
          command: command.trim(),
          comment: comment.trim(),
          user: user.trim(),
          enabled,
          source: initialJob?.source || 'crontab',
        },
      } as any);
      onSaved();
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
      icon={<Clock className="w-5 h-5" />}
      title={initialJob ? t('cron.createModal.titleEdit') : t('cron.createModal.titleCreate')}
      subtitle={t('cron.createModal.subtitle')}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
            {error}
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5 text-slate-400" />
            {t('cron.createModal.scheduleExprLabel')}
          </label>
          <Input
            type="text"
            required
            value={expression}
            onChange={(e) => setExpression(e.target.value)}
            className="font-mono text-xs"
          />
          {/* Quick Presets */}
          <div className="flex flex-wrap gap-1.5 mt-2">
            {presets.map((p) => (
              <button
                type="button"
                key={p.expr}
                onClick={() => setExpression(p.expr)}
                className={`px-2 py-0.5 rounded-lg text-[10px] font-mono transition-colors cursor-pointer ${
                  expression === p.expr
                    ? 'bg-indigo-600 text-white'
                    : 'bg-slate-950 text-slate-400 hover:bg-slate-800 border border-slate-800'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5 text-slate-400" />
            {t('cron.createModal.cmdLabel')}
          </label>
          <Input
            type="text"
            required
            placeholder={t('cron.createModal.cmdPlaceholder')}
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            className="font-mono text-xs"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
              <Code className="w-3.5 h-3.5 text-slate-400" />
              {t('cron.createModal.commentLabel')}
            </label>
            <Input
              type="text"
              placeholder={t('cron.createModal.commentPlaceholder')}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              className="text-xs"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1.5 flex items-center gap-1.5">
              <User className="w-3.5 h-3.5 text-slate-400" />
              {t('cron.createModal.userLabel')}
            </label>
            <Input
              type="text"
              required
              value={user}
              onChange={(e) => setUser(e.target.value)}
              className="font-mono text-xs"
            />
          </div>
        </div>

        <label className="flex items-center gap-2.5 p-2 bg-slate-950/60 border border-slate-800 rounded-xl cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="rounded border-slate-700 text-indigo-600 focus:ring-0 bg-slate-950"
          />
          <span className="text-xs text-slate-300">
            {t('cron.createModal.activeCheckbox')}
          </span>
        </label>

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
            {isSubmitting ? t('cron.createModal.saving') : t('cron.createModal.saveBtn')}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
