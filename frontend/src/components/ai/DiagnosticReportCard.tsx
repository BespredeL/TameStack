import React from 'react';
import { useTranslation } from '../../i18n/LanguageContext';
import {
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  Info,
  CheckCircle2,
  Activity,
  ArrowRight,
  Wrench,
} from 'lucide-react';

export interface WorkflowIssue {
  id: string;
  severity: 'critical' | 'warning' | 'info';
  component: string;
  title: string;
  description: string;
  recommendation: string;
  proposed_action?: string;
  action_params?: Record<string, any>;
}

export interface DiagnosticReportData {
  workflow_type: string;
  server_id: string;
  server_name?: string;
  target?: string;
  status: 'healthy' | 'warning' | 'critical' | 'error';
  summary: string;
  timestamp: string;
  issues: WorkflowIssue[];
  metrics?: Record<string, any>;
}

interface DiagnosticReportCardProps {
  report: DiagnosticReportData;
  onExecuteAction?: (action: string, params: Record<string, any>) => void;
}

export const DiagnosticReportCard: React.FC<DiagnosticReportCardProps> = ({
  report,
  onExecuteAction,
}) => {
  const { t } = useTranslation();
  const getWorkflowTitle = (type: string) => {
    switch (type) {
      case 'site_outage':
        return t('ai.reports.siteAvailability');
      case 'security_audit':
        return t('ai.reports.securityAudit');
      case 'config_optimizer':
        return t('ai.reports.resourceOptimization');
      default:
        return t('ai.reports.genericReport');
    }
  };

  const getStatusBadge = () => {
    switch (report.status) {
      case 'healthy':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <ShieldCheck className="w-3.5 h-3.5" />
            {t('ai.reports.statusOk')}
          </span>
        );
      case 'warning':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <AlertTriangle className="w-3.5 h-3.5" />
            {t('ai.reports.statusWarning')}
          </span>
        );
      case 'critical':
      case 'error':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20">
            <ShieldAlert className="w-3.5 h-3.5" />
            {t('ai.reports.statusCritical')}
          </span>
        );
    }
  };

  const getSeverityBadge = (severity: 'critical' | 'warning' | 'info') => {
    switch (severity) {
      case 'critical':
        return (
          <span className="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded bg-rose-500/20 text-rose-300 border border-rose-500/30">
            Critical
          </span>
        );
      case 'warning':
        return (
          <span className="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded bg-amber-500/20 text-amber-300 border border-amber-500/30">
            Warning
          </span>
        );
      case 'info':
        return (
          <span className="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded bg-sky-500/20 text-sky-300 border border-sky-500/30">
            Info
          </span>
        );
    }
  };

  return (
    <div className="my-3 rounded-xl border border-slate-700/80 bg-slate-900/90 shadow-xl overflow-hidden backdrop-blur-md">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-slate-800/60 border-b border-slate-700/60">
        <div className="flex items-center gap-2.5">
          <Activity className="w-5 h-5 text-indigo-400" />
          <div>
            <div className="text-sm font-semibold text-white">
              {getWorkflowTitle(report.workflow_type)}
            </div>
            {report.target && (
              <div className="text-xs text-slate-400 font-mono">
                {t('ai.reports.target', { target: `${report.target}${report.server_name ? ` (${report.server_name})` : ''}` })}
              </div>
            )}
          </div>
        </div>
        <div>{getStatusBadge()}</div>
      </div>

      {/* Summary */}
      <div className="p-4 border-b border-slate-800/80">
        <p className="text-sm text-slate-200 leading-relaxed">{report.summary}</p>

        {/* Metrics Grid */}
        {report.metrics && Object.keys(report.metrics).length > 0 && (
          <div className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
            {report.metrics.security_score !== undefined && (
              <div className="p-2 rounded bg-slate-950/60 border border-slate-800/80">
                <span className="text-slate-400 block text-[11px]">{t('ai.securityScore')}</span>
                <span className="font-bold text-white text-base">
                  {report.metrics.security_score}
                  <span className="text-xs text-slate-500 font-normal"> / 100</span>
                </span>
              </div>
            )}
            {report.metrics.http_status !== undefined && (
              <div className="p-2 rounded bg-slate-950/60 border border-slate-800/80">
                <span className="text-slate-400 block text-[11px]">{t('ai.reports.httpStatus')}</span>
                <span className={`font-mono font-bold text-base ${Number(report.metrics.http_status) === 200 ? 'text-emerald-400' : 'text-rose-400'}`}>
                  {report.metrics.http_status || 'ERR'}
                </span>
              </div>
            )}
            {report.metrics.latency_ms !== undefined && (
              <div className="p-2 rounded bg-slate-950/60 border border-slate-800/80">
                <span className="text-slate-400 block text-[11px]">{t('ai.reports.latency')}</span>
                <span className="font-mono font-bold text-slate-200 text-base">
                  {report.metrics.latency_ms} ms
                </span>
              </div>
            )}
            {report.metrics.mem_total_mb !== undefined && (
              <div className="p-2 rounded bg-slate-950/60 border border-slate-800/80">
                <span className="text-slate-400 block text-[11px]">{t('ai.reports.ramUsage')}</span>
                <span className="font-bold text-slate-200 text-base">
                  {report.metrics.mem_total_mb} MB
                </span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Issues list */}
      <div className="p-4 space-y-3">
        <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center justify-between">
          <span>{t('ai.reports.detectedIssues', { count: report.issues?.length || 0 })}</span>
        </div>

        {!report.issues || report.issues.length === 0 ? (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
            <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
            <span>{t('ai.reports.noIssues')}</span>
          </div>
        ) : (
          (report.issues || []).map((issue) => (
            <div
              key={issue.id}
              className="p-3.5 rounded-lg bg-slate-950/70 border border-slate-800/90 hover:border-slate-700/80 transition-colors"
            >
              <div className="flex items-start justify-between gap-2 mb-1.5">
                <div className="flex items-center gap-2">
                  {getSeverityBadge(issue.severity)}
                  <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider font-mono">
                    [{issue.component}]
                  </span>
                </div>
                {issue.proposed_action && onExecuteAction && (
                  <button
                    onClick={() => onExecuteAction(issue.proposed_action!, issue.action_params || {})}
                    className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-indigo-300 bg-indigo-600/20 hover:bg-indigo-600/30 border border-indigo-500/40 rounded transition-colors"
                  >
                    <Wrench className="w-3 h-3" />
                    <span>{t('ai.reports.fixBtn')}</span>
                  </button>
                )}
              </div>

              <div className="text-sm font-semibold text-white mb-1">{issue.title}</div>
              <p className="text-xs text-slate-300 leading-relaxed mb-2">{issue.description}</p>

              {issue.recommendation && (
                <div className="mt-2 pt-2 border-t border-slate-800/60 flex items-start gap-1.5 text-xs text-indigo-300/90 bg-indigo-950/20 p-2 rounded">
                  <ArrowRight className="w-3.5 h-3.5 text-indigo-400 shrink-0 mt-0.5" />
                  <div>
                    <span className="font-semibold text-indigo-200">{t('ai.reports.recommendation')}</span>
                    {issue.recommendation}
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
};
