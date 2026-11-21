import React, { useState, useEffect } from 'react';
import { useTranslation } from '../../i18n/LanguageContext';
import { useAlert } from '../../context/AlertContext';
import {
  Bot,
  Play,
  Pause,
  RefreshCw,
  AlertTriangle,
  Clock,
  Activity,
  ChevronDown,
  ChevronRight,
  ShieldAlert,
} from 'lucide-react';
import {
  AIListAgents,
  AIToggleAgent,
  AITriggerAgent,
} from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { Button, Badge } from '../ui';

export interface AgentStatusDTO {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  interval_seconds: number;
  last_run_at?: string;
  next_run_at?: string;
  last_status: string;
  last_error?: string;
  findings_count: number;
  recent_findings: Array<{
    id: string;
    agent_id: string;
    server_id: string;
    severity: 'critical' | 'warning' | 'info' | string;
    title: string;
    details: string;
    created_at: string;
  }>;
}

interface AgentsManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const localizeFindingTitle = (title: string, t: any): string => {
  let m = title.match(/^Критическое переполнение диска на (.*?): ([\d\.]+)%/);
  if (m) return t('ai.agents.findingsTemplates.critDisk', { server: m[1], percent: m[2] });

  m = title.match(/^Высокая занятость диска на (.*?): ([\d\.]+)%/);
  if (m) return t('ai.agents.findingsTemplates.highDisk', { server: m[1], percent: m[2] });

  m = title.match(/^Критическая нехватка RAM на (.*?): ([\d\.]+)%/);
  if (m) return t('ai.agents.findingsTemplates.critRam', { server: m[1], percent: m[2] });

  m = title.match(/^Повышенная утилизация RAM на (.*?): ([\d\.]+)%/);
  if (m) return t('ai.agents.findingsTemplates.highRam', { server: m[1], percent: m[2] });

  m = title.match(/^Высокая загрузка процессора на (.*?): ([\d\.]+)%/);
  if (m) return t('ai.agents.findingsTemplates.highCpu', { server: m[1], percent: m[2] });

  m = title.match(/^Брандмауэр отключен на сервере (.*?)$/);
  if (m) return t('ai.agents.findingsTemplates.fwDisabled', { server: m[1] });

  m = title.match(/^Fail2ban не активен на сервере (.*?)$/);
  if (m) return t('ai.agents.findingsTemplates.f2bDisabled', { server: m[1] });

  m = title.match(/^SSL-сертификат для (.*?) истекает через (\d+) дн\.!$/);
  if (m) return t('ai.agents.findingsTemplates.certExpiringUrgent', { domain: m[1], days: m[2] });

  m = title.match(/^SSL-сертификат для (.*?) истекает через (\d+) дн\.$/);
  if (m) return t('ai.agents.findingsTemplates.certExpiring', { domain: m[1], days: m[2] });

  return title;
};

export const AgentsManagerModal: React.FC<AgentsManagerModalProps> = ({
  isOpen,
  onClose,
}) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const [agents, setAgents] = useState<AgentStatusDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [runningAgentId, setRunningAgentId] = useState<string | null>(null);
  const [expandedAgentId, setExpandedAgentId] = useState<string | null>(null);

  const fetchAgents = async () => {
    try {
      setLoading(true);
      const list = await AIListAgents();
      setAgents((list as any) || []);
    } catch (e) {
      console.error('Failed to load agents:', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      fetchAgents();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleToggle = async (agentId: string, currentEnabled: boolean) => {
    try {
      await AIToggleAgent(agentId, !currentEnabled);
      fetchAgents();
    } catch (e) {
      alert(t('ai.agents.toggleError', { error: String(e) }));
    }
  };

  const handleRunNow = async (agentId: string) => {
    try {
      setRunningAgentId(agentId);
      await AITriggerAgent(agentId);
      await fetchAgents();
    } catch (e) {
      alert(t('ai.agents.execError', { error: String(e) }));
    } finally {
      setRunningAgentId(null);
    }
  };

  const formatInterval = (sec: number) => {
    if (sec >= 3600) return t('ai.agents.everyHours', { hours: Math.round(sec / 3600) });
    if (sec >= 60) return t('ai.agents.everyMinutes', { minutes: Math.round(sec / 60) });
    return t('ai.agents.everySeconds', { seconds: sec });
  };

  return (
    <ModalShell
      onClose={onClose}
      size="3xl"
      className="max-h-[85vh]"
      icon={<Bot className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0"
      title={t('ai.agents.title')}
      subtitle={t('ai.agents.subtitle')}
      headerActions={
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={fetchAgents}
          loading={loading}
          icon={<RefreshCw className="w-4 h-4" />}
          title={t('ai.agents.refreshTooltip')}
        />
      }
    >
      {/* Content */}
      <div className="p-6 overflow-y-auto space-y-4 divide-y divide-slate-850/60">
        {agents.length === 0 ? (
          <div className="text-center py-10 text-slate-500 text-sm">
            {t('ai.agents.noAgents')}
          </div>
        ) : (
          agents.map((ag) => {
            const isRunning = runningAgentId === ag.id || ag.last_status === 'running';
            const isExpanded = expandedAgentId === ag.id;

            return (
              <div key={ag.id} className="pt-4 first:pt-0">
                <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800/80 hover:border-slate-700/80 transition-all">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex items-start gap-3">
                      <div
                        className={`w-9 h-9 rounded-lg flex items-center justify-center shrink-0 mt-0.5 border ${
                          ag.last_status === 'healthy'
                            ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                            : ag.last_status === 'warning'
                            ? 'bg-amber-500/10 border-amber-500/30 text-amber-400'
                            : ag.last_status === 'error'
                            ? 'bg-rose-500/10 border-rose-500/30 text-rose-400'
                            : 'bg-slate-800/60 border-slate-700 text-slate-400'
                        }`}
                      >
                        <Activity className="w-5 h-5" />
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-white text-sm">
                            {t(`ai.agents.catalog.${ag.id}.name`) || ag.name}
                          </span>
                          <Badge variant={ag.enabled ? 'success' : 'neutral'} size="sm">
                            {ag.enabled ? t('ai.agents.active') : t('ai.agents.paused')}
                          </Badge>
                        </div>
                        <p className="text-xs text-slate-400 mt-1 leading-relaxed">
                          {t(`ai.agents.catalog.${ag.id}.desc`) || ag.description}
                        </p>

                        <div className="flex items-center gap-4 mt-2 text-[11px] text-slate-500 font-mono">
                          <span className="flex items-center gap-1">
                            <Clock className="w-3.5 h-3.5 text-slate-400" />
                            {formatInterval(ag.interval_seconds)}
                          </span>
                          {ag.last_run_at && (
                            <span>
                              {t('ai.agents.lastRun', { time: new Date(ag.last_run_at).toLocaleTimeString() })}
                            </span>
                          )}
                          {ag.findings_count > 0 && (
                            <span className="text-amber-400 font-semibold">
                              {t('ai.agents.findings', { count: ag.findings_count })}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-2 shrink-0">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => handleRunNow(ag.id)}
                        loading={isRunning}
                        icon={<Play className="w-3.5 h-3.5" />}
                        title={t('ai.agents.runNowTooltip')}
                      >
                        {isRunning ? t('ai.agents.checking') : t('ai.agents.runBtn')}
                      </Button>
                      <button
                        onClick={() => handleToggle(ag.id, ag.enabled)}
                        className={`p-1.5 rounded-lg border text-xs transition-colors cursor-pointer ${
                          ag.enabled
                            ? 'bg-amber-500/10 border-amber-500/30 text-amber-400 hover:bg-amber-500/20'
                            : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400 hover:bg-emerald-500/20'
                        }`}
                        title={ag.enabled ? t('ai.agents.pauseTooltip') : t('ai.agents.activateTooltip')}
                      >
                        {ag.enabled ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>

                  {/* Expand findings toggle */}
                  {ag.recent_findings && ag.recent_findings.length > 0 && (
                    <div className="mt-3 pt-3 border-t border-slate-850">
                      <button
                        onClick={() => setExpandedAgentId(isExpanded ? null : ag.id)}
                        className="flex items-center justify-between w-full text-xs text-slate-400 hover:text-white cursor-pointer"
                      >
                        <span>{t('ai.agents.recentFindingsTitle', { count: ag.recent_findings.length })}</span>
                        {isExpanded ? (
                          <ChevronDown className="w-4 h-4" />
                        ) : (
                          <ChevronRight className="w-4 h-4" />
                        )}
                      </button>

                      {isExpanded && (
                        <div className="mt-2 space-y-2">
                          {ag.recent_findings.map((f) => (
                            <div
                              key={f.id}
                              className="p-2.5 rounded bg-slate-900 border border-slate-800 text-xs"
                            >
                              <div className="flex items-center justify-between gap-2 mb-1">
                                <div className="flex items-center gap-1.5 font-semibold text-slate-200">
                                  {f.severity === 'critical' ? (
                                    <ShieldAlert className="w-3.5 h-3.5 text-rose-400 shrink-0" />
                                  ) : (
                                    <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                                  )}
                                  <span>{localizeFindingTitle(f.title, t)}</span>
                                </div>
                                <span className="text-[10px] text-slate-500 font-mono">
                                  {new Date(f.created_at).toLocaleTimeString()}
                                </span>
                              </div>
                              <p className="text-[11px] text-slate-400 whitespace-pre-wrap leading-relaxed">
                                {f.details}
                              </p>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>
    </ModalShell>
  );
};
