import React, { useState } from 'react';
import { ShieldAlert, Check, X, Terminal, FileCode, AlertTriangle, ArrowRight } from 'lucide-react';
import { AIApproveTask, AIRejectTask } from '../../../wailsjs/go/main/App';
import { useTranslation } from '../../i18n/LanguageContext';
import { useAlert } from '../../context/AlertContext';
import { ModalShell } from '../common/ModalShell';
import { Button, Badge } from '../ui';

interface ExecutionApprovalModalProps {
  isOpen: boolean;
  onClose: () => void;
  taskID: string;
  title: string;
  serverName?: string;
  diff?: string;
  steps?: string[];
  riskLevel?: string;
  onApproved?: () => void;
  onRejected?: () => void;
}

export const ExecutionApprovalModal: React.FC<ExecutionApprovalModalProps> = ({
  isOpen,
  onClose,
  taskID,
  title,
  serverName,
  diff,
  steps,
  riskLevel = 'high',
  onApproved,
  onRejected,
}) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleApprove = async () => {
    setLoading(true);
    try {
      await AIApproveTask(taskID);
      if (onApproved) onApproved();
      onClose();
    } catch (err: unknown) {
      alert(t('ai.approval.approveError', { error: String(err) }));
    } finally {
      setLoading(false);
    }
  };

  const handleReject = async () => {
    setLoading(true);
    try {
      await AIRejectTask(taskID);
      if (onRejected) onRejected();
      onClose();
    } catch (err: unknown) {
      alert(t('ai.approval.rejectError', { error: String(err) }));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="3xl"
      className="max-h-[90vh]"
      icon={<ShieldAlert className="w-5 h-5" />}
      iconClassName="w-9 h-9 rounded-xl bg-amber-500/20 text-amber-400 border border-amber-500/30 flex items-center justify-center shrink-0"
      title={t('ai.approval.title')}
      headerExtra={
        <Badge variant="danger" size="sm" className="uppercase tracking-wider">
          {riskLevel} Risk
        </Badge>
      }
      subtitle={title}
    >
      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {serverName && (
          <div className="text-xs text-slate-300 bg-slate-850 p-2.5 rounded-lg border border-slate-800 flex items-center justify-between">
            <span>{t('ai.approval.targetServer', { server: serverName })}</span>
            <span className="text-[11px] text-slate-400 font-mono">Task ID: {taskID}</span>
          </div>
        )}

        {/* Planned Steps */}
        {steps && steps.length > 0 && (
          <div>
            <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5 text-indigo-400" />
              {t('ai.approval.executionPlan')}
            </h4>
            <div className="bg-slate-950/60 border border-slate-800/80 rounded-lg p-3 space-y-1.5 text-xs text-slate-300 font-mono">
              {steps.map((st, i) => (
                <div key={i} className="flex items-center gap-2">
                  <ArrowRight className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                  <span>{st}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Diff Preview */}
        {diff && (
          <div>
            <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 flex items-center gap-1.5">
              <FileCode className="w-3.5 h-3.5 text-amber-400" />
              {t('ai.approval.unifiedDiff')}
            </h4>
            <div className="bg-slate-950 rounded-lg border border-slate-800 font-mono text-xs overflow-hidden max-h-72 overflow-y-auto">
              {diff.split('\n').map((line, idx) => {
                let lineStyle = 'text-slate-400';
                let bgStyle = '';
                if (line.startsWith('+')) {
                  lineStyle = 'text-emerald-300';
                  bgStyle = 'bg-emerald-950/30';
                } else if (line.startsWith('-')) {
                  lineStyle = 'text-rose-300';
                  bgStyle = 'bg-rose-950/30';
                } else if (line.startsWith('@@') || line.startsWith('---') || line.startsWith('+++')) {
                  lineStyle = 'text-indigo-400 font-bold';
                  bgStyle = 'bg-indigo-950/20';
                }

                return (
                  <div key={idx} className={`px-3 py-0.5 leading-relaxed font-mono ${lineStyle} ${bgStyle}`}>
                    {line || ' '}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 flex items-start gap-2.5 text-xs text-amber-300">
          <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5 text-amber-400" />
          <div>
            {t('ai.approval.rollbackSafetyNotice')}
          </div>
        </div>
      </div>

      {/* Footer Actions */}
      <div className="px-6 py-4 border-t border-slate-800 bg-slate-950/40 flex items-center justify-end gap-3">
        <Button
          variant="secondary"
          size="md"
          onClick={handleReject}
          loading={loading}
          icon={<X className="w-4 h-4" />}
        >
          {t('ai.approval.rejectBtn')}
        </Button>

        <Button
          variant="success"
          size="md"
          onClick={handleApprove}
          loading={loading}
          icon={<Check className="w-4 h-4" />}
        >
          {t('ai.approval.approveBtn')}
        </Button>
      </div>
    </ModalShell>
  );
};
