import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Sparkles,
  Bot,
  User,
  Send,
  X,
  Plus,
  Maximize2,
  Server,
  Layers,
  Loader2,
  ShieldAlert,
  Settings,
  Wrench,
  ChevronDown,
  ChevronRight,
  Check,
  Brain,
} from 'lucide-react';
import { Badge, Button } from '../ui';
import { MarkdownRenderer } from './MarkdownRenderer';
import { useTranslation } from '../../i18n/LanguageContext';
import { useServer } from '../../context/ServerContext';
import { getErrorMessage } from '../../utils/errors';
import {
  AIApproveTask,
  AICreateConversation,
  AIListMessages,
  AIListProviderConfigs,
  AIRejectTask,
  AIStreamMessageWithContext,
} from '../../../wailsjs/go/main/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import {
  AIMessageRecord,
  AIStreamChunkPayload,
  AIStreamCompletePayload,
  AIStreamErrorPayload,
  NavigationTab,
} from '../../types';

interface GlobalAICopilotProps {
  isOpen: boolean;
  onClose: () => void;
  currentTab: NavigationTab;
  onOpenFullAI: () => void;
}

const getTabSuggestions = (tab: NavigationTab, t: (key: string) => string): string[] => {
  const key = ['sites', 'mail', 'dns', 'users', 'security', 'databases', 'docker'].includes(tab)
    ? tab
    : 'default';
  return [
    t(`ai.copilot.suggestions.${key}.q1`),
    t(`ai.copilot.suggestions.${key}.q2`),
  ];
};

const parseThinkBlocks = (raw: string): { thinking: string; answer: string } => {
  if (!raw) return { thinking: '', answer: '' };
  const thinkingParts: string[] = [];
  let answer = raw.replace(/<think>([\s\S]*?)<\/think>/gi, (_, inner) => {
    const trimmed = String(inner || '').trim();
    if (trimmed) thinkingParts.push(trimmed);
    return '';
  });
  const openIdx = answer.toLowerCase().indexOf('<think>');
  if (openIdx !== -1) {
    const unclosed = answer.slice(openIdx + '<think>'.length).trim();
    if (unclosed) thinkingParts.push(unclosed);
    answer = answer.slice(0, openIdx);
  }
  return {
    thinking: thinkingParts.join('\n\n'),
    answer: answer.trim(),
  };
};

const CopilotAssistantContent: React.FC<{ content: string; reasoningLabel: string }> = ({
  content,
  reasoningLabel,
}) => {
  const [thinkOpen, setThinkOpen] = useState(false);
  const { thinking, answer } = parseThinkBlocks(content);

  return (
    <div className="space-y-2">
      {thinking && (
        <div className="rounded-lg border border-slate-800 bg-slate-950/70 overflow-hidden">
          <button
            type="button"
            onClick={() => setThinkOpen((v) => !v)}
            className="w-full flex items-center justify-between gap-2 px-2.5 py-1.5 text-[11px] text-slate-400 hover:text-slate-200 hover:bg-slate-900/70 transition cursor-pointer text-left"
          >
            <span className="flex items-center gap-1.5 font-medium">
              <Brain className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
              <span>{reasoningLabel}</span>
            </span>
            {thinkOpen ? <ChevronDown className="w-3.5 h-3.5 shrink-0" /> : <ChevronRight className="w-3.5 h-3.5 shrink-0" />}
          </button>
          {thinkOpen && (
            <div className="border-t border-slate-800/80 px-2.5 py-2 text-[11px] text-slate-400 whitespace-pre-wrap leading-relaxed select-text max-h-56 overflow-y-auto">
              {thinking}
            </div>
          )}
        </div>
      )}
      {answer && <MarkdownRenderer content={answer} />}
    </div>
  );
};

const CopilotToolResult: React.FC<{
  message: AIMessageRecord;
  toolName?: string;
  onApprovalComplete?: () => void;
}> = ({ message, toolName, onApprovalComplete }) => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [approvalStatus, setApprovalStatus] = useState<'pending' | 'approved' | 'rejected'>('pending');

  let parsedData: any = null;
  try {
    parsedData = JSON.parse(message.content);
  } catch {
    parsedData = message.content;
  }

  const isApprovalRequired = parsedData && typeof parsedData === 'object' && parsedData.status === 'awaiting_approval';

  if (isApprovalRequired) {
    return (
      <div className="flex gap-2.5 text-xs justify-start">
        <div className="w-6 h-6 rounded-md bg-amber-500/15 border border-amber-500/30 text-amber-400 flex items-center justify-center shrink-0 mt-0.5">
          <ShieldAlert className="w-3.5 h-3.5" />
        </div>
        <div className="flex-1 min-w-0 rounded-xl border border-amber-500/30 bg-slate-900/80 p-2.5 space-y-2">
          <div className="font-semibold text-amber-300 text-[11px]">
            {t('ai.message.approvalRequired', { title: parsedData.title || parsedData.description || t('ai.message.aiAction') })}
          </div>
          {approvalStatus === 'pending' ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={async () => {
                  if (parsedData.task_id) {
                    await AIApproveTask(parsedData.task_id);
                    setApprovalStatus('approved');
                    onApprovalComplete?.();
                  }
                }}
                className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-[11px] transition cursor-pointer"
              >
                <Check className="w-3 h-3" />
                <span>{t('ai.message.approveAndRun')}</span>
              </button>
              <button
                type="button"
                onClick={async () => {
                  if (parsedData.task_id) {
                    await AIRejectTask(parsedData.task_id);
                    setApprovalStatus('rejected');
                    onApprovalComplete?.();
                  }
                }}
                className="flex items-center gap-1 px-2.5 py-1 rounded-lg border border-slate-700 hover:bg-slate-800 text-slate-300 text-[11px] transition cursor-pointer"
              >
                <X className="w-3 h-3" />
                <span>{t('ai.message.reject')}</span>
              </button>
            </div>
          ) : (
            <div className={`text-[11px] font-semibold ${approvalStatus === 'approved' ? 'text-emerald-400' : 'text-rose-400'}`}>
              {approvalStatus === 'approved' ? t('ai.message.taskApproved') : t('ai.message.taskRejected')}
            </div>
          )}
        </div>
      </div>
    );
  }

  const displayName = toolName ? `${t('ai.message.toolOutput')}: ${toolName}` : t('ai.message.toolOutput');
  const formattedBody =
    typeof parsedData === 'string'
      ? parsedData
      : parsedData && typeof parsedData === 'object' && (parsedData.content || parsedData.raw_output || parsedData.output || parsedData.stdout)
      ? String(parsedData.content || parsedData.raw_output || parsedData.output || parsedData.stdout)
      : JSON.stringify(parsedData, null, 2);

  return (
    <div className="flex gap-2.5 text-xs justify-start">
      <div className="w-6 h-6 rounded-md bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 flex items-center justify-center shrink-0 mt-0.5">
        <Wrench className="w-3.5 h-3.5" />
      </div>
      <div className="flex-1 min-w-0 rounded-xl border border-slate-800 bg-slate-900/60 overflow-hidden">
        <button
          type="button"
          onClick={() => setIsOpen((v) => !v)}
          className="w-full flex items-center justify-between gap-2 px-3 py-1.5 hover:bg-slate-800/60 transition cursor-pointer text-left"
        >
          <span className="font-mono text-[11px] text-emerald-400 font-medium truncate">
            {displayName}
          </span>
          <span className="flex items-center gap-1 text-[10px] text-slate-500 shrink-0">
            <span>{isOpen ? t('common.collapse') : t('common.details')}</span>
            {isOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
          </span>
        </button>
        {isOpen && (
          <pre className="border-t border-slate-800/80 p-2.5 bg-slate-950/90 font-mono text-[10px] text-slate-300 whitespace-pre-wrap overflow-x-auto max-h-56 overflow-y-auto leading-relaxed select-text">
            {formattedBody}
          </pre>
        )}
      </div>
    </div>
  );
};

export const GlobalAICopilot: React.FC<GlobalAICopilotProps> = ({
  isOpen,
  onClose,
  currentTab,
  onOpenFullAI,
}) => {
  const { t, language } = useTranslation();
  const { selectedServer, selectedServerId } = useServer();

  const [convId, setConvId] = useState<string | null>(null);
  const [messages, setMessages] = useState<AIMessageRecord[]>([]);
  const [input, setInput] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [includeContext, setIncludeContext] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [providersChecked, setProvidersChecked] = useState(false);
  const [isAIConfigured, setIsAIConfigured] = useState(true);
  const [isCloudActive, setIsCloudActive] = useState(false);
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    try {
      const saved = parseInt(localStorage.getItem('tamestack_ai_copilot_width') || '460', 10);
      return Number.isFinite(saved) && saved >= 360 ? saved : 460;
    } catch {
      return 460;
    }
  });
  const [isResizing, setIsResizing] = useState(false);

  const streamConvRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    const isLocalProvider = (p: any): boolean => {
      if (!p) return false;
      if (p.provider_type === 'ollama' || p.provider_type === 'llamacpp') return true;
      const url = String(p.base_url || '').trim().toLowerCase();
      if (!url) return false;
      try {
        const host = new URL(url).hostname;
        if (
          host === 'localhost' ||
          host.endsWith('.localhost') ||
          host.endsWith('.local') ||
          host === '0.0.0.0' ||
          host.startsWith('127.') ||
          host === '::1' ||
          host === '[::1]' ||
          host.startsWith('10.') ||
          host.startsWith('192.168.') ||
          /^172\.(1[6-9]|2\d|3[01])\./.test(host)
        ) {
          return true;
        }
      } catch {
        // ignore invalid URL
      }
      return false;
    };

    AIListProviderConfigs()
      .then((list: any) => {
        if (cancelled) return;
        const providers = Array.isArray(list) ? list : [];
        const active = providers.filter(
          (p: any) =>
            p.is_enabled &&
            (isLocalProvider(p) || p.has_api_key)
        );
        setIsAIConfigured(active.length > 0);
        const def = active.find((p: any) => p.is_default) || active[0];
        setIsCloudActive(!!def && !isLocalProvider(def));
        setProvidersChecked(true);
      })
      .catch(() => {
        if (!cancelled) {
          setProvidersChecked(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isResizing) return;
    const onMouseMove = (e: MouseEvent) => {
      const maxW = Math.min(960, window.innerWidth - 60);
      const next = Math.max(360, Math.min(maxW, window.innerWidth - e.clientX));
      setPanelWidth(next);
    };
    const onMouseUp = () => {
      setIsResizing(false);
      try {
        localStorage.setItem('tamestack_ai_copilot_width', String(panelWidth));
      } catch {
        // ignore
      }
    };
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [isResizing, panelWidth]);

  useEffect(() => {
    streamConvRef.current = convId;
  }, [convId]);

  useEffect(() => {
    const unChunk = EventsOn('ai:stream:chunk', (data: AIStreamChunkPayload) => {
      if (data && data.conversation_id === streamConvRef.current && data.content_delta) {
        setStreamingContent((prev) => prev + data.content_delta);
      }
    });

    const unComplete = EventsOn('ai:stream:complete', (data: AIStreamCompletePayload) => {
      if (data && data.conversation_id === streamConvRef.current) {
        setIsStreaming(false);
        setStreamingContent('');
        AIListMessages(data.conversation_id)
          .then((msgs: any) => setMessages((msgs || []) as AIMessageRecord[]))
          .catch(() => {});
      }
    });

    const unError = EventsOn('ai:stream:error', (data: AIStreamErrorPayload) => {
      if (data && data.conversation_id === streamConvRef.current) {
        setIsStreaming(false);
        setError(data.error || 'AI streaming error');
      }
    });

    return () => {
      if (typeof unChunk === 'function') unChunk();
      if (typeof unComplete === 'function') unComplete();
      if (typeof unError === 'function') unError();
    };
  }, []);

  useEffect(() => {
    if (isOpen) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [messages, streamingContent, isOpen]);

  const toolNameMap = useMemo(() => {
    const map = new Map<string, string>();
    messages.forEach((m) => {
      if (m.role === 'assistant' && m.tool_calls_json) {
        try {
          const tcs = JSON.parse(m.tool_calls_json);
          if (Array.isArray(tcs)) {
            tcs.forEach((tc: any) => {
              if (tc.id && tc.function?.name) {
                map.set(tc.id, tc.function.name);
              }
            });
          }
        } catch {}
      }
    });
    return map;
  }, [messages]);

  const visibleMessages = useMemo(() => {
    return messages.filter((m) => {
      if (m.role === 'assistant' && (!m.content || !m.content.trim())) {
        return false;
      }
      return true;
    });
  }, [messages]);

  if (!isOpen) return null;

  const tabTitle = t(`titles.${currentTab}.title`);
  const tabSubtitle = t(`titles.${currentTab}.subtitle`);
  const suggestions = getTabSuggestions(currentTab, t);

  const startNewChat = async () => {
    setConvId(null);
    setMessages([]);
    setStreamingContent('');
    setError(null);
  };

  const handleSend = async (customPrompt?: string) => {
    const text = (customPrompt ?? input).trim();
    if (!text || isStreaming) return;

    setError(null);
    let targetConvId = convId;
    if (!targetConvId) {
      try {
        const title = `Copilot: ${tabTitle}`.slice(0, 48);
        const created = await AICreateConversation(
          title,
          selectedServerId || null,
          '',
          '',
          ''
        );
        targetConvId = created.id;
        setConvId(created.id);
        streamConvRef.current = created.id;
      } catch (err: unknown) {
        setError(getErrorMessage(err));
        return;
      }
    }

    if (!customPrompt) setInput('');
    setIsStreaming(true);
    setStreamingContent('');

    const optimisticUser: AIMessageRecord = {
      id: 'temp_' + Date.now(),
      conversation_id: targetConvId,
      role: 'user',
      content: text,
      prompt_tokens: 0,
      completion_tokens: 0,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, optimisticUser]);

    const extractPageContext = (): string => {
      try {
        const mainEl = document.querySelector('main');
        if (!mainEl) return tabSubtitle || '';
        const parts: string[] = [];
        if (tabSubtitle) parts.push(`Description: ${tabSubtitle}`);

        const headings = Array.from(mainEl.querySelectorAll('h1, h2, h3, h4'))
          .map((el) => el.textContent?.replace(/\s+/g, ' ').trim())
          .filter((v): v is string => !!v && v.length < 120)
          .slice(0, 8);
        if (headings.length > 0) parts.push(`Visible headings: ${headings.join(' | ')}`);

        const activeTabs = Array.from(mainEl.querySelectorAll('[aria-selected="true"], [data-state="active"]'))
          .map((el) => el.textContent?.replace(/\s+/g, ' ').trim())
          .filter((v): v is string => !!v && v.length < 60)
          .slice(0, 4);
        if (activeTabs.length > 0) parts.push(`Active sub-tabs: ${activeTabs.join(', ')}`);

        const buttons = Array.from(mainEl.querySelectorAll('button'))
          .map((el) => (el.textContent?.replace(/\s+/g, ' ').trim() || el.getAttribute('aria-label') || ''))
          .filter((v) => v.length > 1 && v.length < 50)
          .filter((v, idx, arr) => arr.indexOf(v) === idx)
          .slice(0, 18);
        if (buttons.length > 0) parts.push(`Visible page actions/buttons: ${buttons.join(', ')}`);

        const tableHeaders = Array.from(mainEl.querySelectorAll('thead th'))
          .map((el) => el.textContent?.replace(/\s+/g, ' ').trim())
          .filter((v): v is string => !!v && v.length < 40)
          .slice(0, 12);
        const rowCount = mainEl.querySelectorAll('tbody tr').length;
        if (tableHeaders.length > 0) {
          parts.push(`Table columns (${rowCount} rows): ${tableHeaders.join(', ')}`);
        }

        const alerts = Array.from(mainEl.querySelectorAll('[role="alert"], .text-rose-300, .text-amber-300'))
          .map((el) => el.textContent?.replace(/\s+/g, ' ').trim())
          .filter((v): v is string => !!v && v.length > 3 && v.length < 220)
          .slice(0, 3);
        if (alerts.length > 0) parts.push(`Visible notices/errors: ${alerts.join(' | ')}`);

        return parts.join('\n').slice(0, 1500);
      } catch {
        return tabSubtitle || '';
      }
    };

    const uiCtx = includeContext
      ? {
          active_tab: currentTab,
          active_tab_title: tabTitle,
          selected_server_id: selectedServer?.id || '',
          selected_server_name: selectedServer?.name || '',
          selected_server_host: selectedServer?.host || '',
          screen_summary: extractPageContext(),
        }
      : undefined;

    try {
      await AIStreamMessageWithContext(targetConvId, text, uiCtx as any);
    } catch (err: unknown) {
      setIsStreaming(false);
      setError(getErrorMessage(err));
    }
  };

  return (
    <div
      style={{ width: `min(100vw, ${panelWidth}px)` }}
      className={`fixed inset-y-0 right-0 z-40 bg-surface-950 border-l border-slate-800 shadow-2xl shadow-black/80 flex flex-col ${isResizing ? 'select-none' : ''}`}
    >
      {/* Left Resize Handle */}
      <div
        onMouseDown={(e) => {
          e.preventDefault();
          setIsResizing(true);
        }}
        title={t('ai.copilot.dragToResize')}
        className="hidden sm:block absolute inset-y-0 left-0 w-1.5 cursor-col-resize hover:bg-indigo-500/60 active:bg-indigo-500 transition-colors z-50"
      />
      {/* Header */}
      <div className="h-14 px-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/80 shrink-0">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/15 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
            <Sparkles className="w-4 h-4" />
          </div>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-white truncate">TameStack AI Copilot</div>
            <div className="text-[11px] text-slate-400 truncate">
              {t('ai.copilot.subtitle')}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="sm"
            onClick={startNewChat}
            title={t('ai.copilot.newConversation')}
            aria-label={t('ai.copilot.newConversation')}
          >
            <Plus className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              onClose();
              onOpenFullAI();
            }}
            title={t('ai.copilot.openFullView')}
            aria-label={t('ai.copilot.openFullView')}
          >
            <Maximize2 className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            title={t('ai.copilot.closeTooltip')}
            aria-label={t('ai.copilot.closeAria')}
          >
            <X className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Context Bar */}
      <div className="px-4 py-2.5 border-b border-slate-800/80 bg-slate-900/50 flex flex-wrap items-center justify-between gap-2 text-[11px] shrink-0">
        <div className="flex flex-wrap items-center gap-1.5 min-w-0">
          <Badge variant="indigo" size="sm" className="inline-flex items-center gap-1">
            <Layers className="w-3 h-3" />
            {tabTitle}
          </Badge>
          {selectedServer && (
            <Badge variant="neutral" size="sm" className="inline-flex items-center gap-1 font-mono">
              <Server className="w-3 h-3 text-emerald-400" />
              {selectedServer.name}
            </Badge>
          )}
        </div>
        <label className="flex items-center gap-1.5 text-slate-400 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={includeContext}
            onChange={(e) => setIncludeContext(e.target.checked)}
            className="rounded border-slate-700 bg-slate-950 text-indigo-500"
          />
          <span>{t('ai.copilot.screenContext')}</span>
        </label>
      </div>

      {/* Cloud AI Privacy Notice */}
      {providersChecked && isAIConfigured && isCloudActive && (
        <div className="px-4 py-2 border-b border-amber-500/20 bg-amber-500/10 text-amber-200 text-[11px] flex items-start gap-2 shrink-0">
          <ShieldAlert className="w-3.5 h-3.5 text-amber-400 shrink-0 mt-0.5" />
          <span>
            {t('ai.copilot.cloudWarning')}
          </span>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {providersChecked && !isAIConfigured ? (
          <div className="py-6 space-y-4">
            <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-amber-500/15 border border-amber-500/30 flex items-center justify-center text-amber-400 shrink-0">
                  <Settings className="w-4 h-4" />
                </div>
                <div>
                  <div className="text-xs font-semibold text-white">
                    {t('ai.copilot.notConfiguredTitle')}
                  </div>
                  <div className="text-[11px] text-slate-400">
                    {t('ai.copilot.notConfiguredDesc')}
                  </div>
                </div>
              </div>

              <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/25 text-amber-200 text-[11px] leading-relaxed flex items-start gap-2">
                <ShieldAlert className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                <div>
                  <div className="font-semibold text-amber-300 mb-0.5">
                    {t('ai.copilot.securityRecTitle')}
                  </div>
                  {t('ai.copilot.securityRecDesc')}
                </div>
              </div>

              <Button
                variant="primary"
                size="sm"
                className="w-full"
                onClick={() => {
                  onClose();
                  onOpenFullAI();
                }}
              >
                <Settings className="w-3.5 h-3.5 mr-1.5" />
                {t('ai.copilot.openSettingsBtn')}
              </Button>
            </div>
          </div>
        ) : messages.length === 0 && !streamingContent && (
          <div className="py-6 space-y-4">
            <div className="text-center space-y-1.5">
              <Bot className="w-8 h-8 text-indigo-400 mx-auto" />
              <p className="text-xs font-medium text-slate-200">
                {t('ai.copilot.emptyPrompt')}
              </p>
            </div>
            <div className="space-y-2">
              {suggestions.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => handleSend(s)}
                  className="w-full text-left px-3 py-2 rounded-xl bg-slate-900/80 hover:bg-slate-800 border border-slate-800 text-xs text-slate-300 transition"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {visibleMessages.map((m) =>
          m.role === 'tool' ? (
            <CopilotToolResult
              key={m.id}
              message={m}
              toolName={m.tool_call_id ? toolNameMap.get(m.tool_call_id) : undefined}
              onApprovalComplete={() => {
                if (convId) {
                  AIListMessages(convId).then((msgs: any) => setMessages(msgs || []));
                }
              }}
            />
          ) : (
            <div
              key={m.id}
              className={`flex gap-2.5 text-xs ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              {m.role !== 'user' && (
                <div className="w-6 h-6 rounded-md bg-indigo-500/20 text-indigo-400 flex items-center justify-center shrink-0 mt-0.5">
                  <Bot className="w-3.5 h-3.5" />
                </div>
              )}
              <div
                className={`max-w-[85%] rounded-xl px-3 py-2 leading-relaxed ${
                  m.role === 'user'
                    ? 'bg-indigo-600 text-white'
                    : 'bg-slate-900 border border-slate-800 text-slate-200'
                }`}
              >
                {m.role === 'user' ? (
                  <span className="whitespace-pre-wrap">{m.content}</span>
                ) : (
                  <CopilotAssistantContent content={m.content} reasoningLabel={t('ai.copilot.reasoning')} />
                )}
              </div>
              {m.role === 'user' && (
                <div className="w-6 h-6 rounded-md bg-slate-800 text-slate-300 flex items-center justify-center shrink-0 mt-0.5">
                  <User className="w-3.5 h-3.5" />
                </div>
              )}
            </div>
          )
        )}

        {streamingContent && (
          <div className="flex gap-2.5 text-xs justify-start">
            <div className="w-6 h-6 rounded-md bg-indigo-500/20 text-indigo-400 flex items-center justify-center shrink-0 mt-0.5">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            </div>
            <div className="max-w-[85%] rounded-xl px-3 py-2 bg-slate-900 border border-slate-800 text-slate-200 leading-relaxed">
              <CopilotAssistantContent content={streamingContent} reasoningLabel={t('ai.copilot.reasoning')} />
            </div>
          </div>
        )}

        {error && (
          <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs">
            {error}
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          handleSend();
        }}
        className="p-3 border-t border-slate-800 bg-slate-950/80 flex items-stretch gap-2 shrink-0"
      >
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
          rows={2}
          disabled={!isAIConfigured}
          placeholder={
            !isAIConfigured
              ? t('ai.copilot.placeholderDisabled')
              : t('ai.copilot.placeholderActive')
          }
          className="flex-1 resize-none rounded-xl bg-slate-900 border border-slate-800 px-3 py-2 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-indigo-500 disabled:opacity-50"
        />
        <Button
          type="submit"
          variant="primary"
          size="md"
          disabled={!isAIConfigured || isStreaming || !input.trim()}
          loading={isStreaming}
          className="px-4 self-stretch"
          title={t('ai.copilot.sendMessage')}
          aria-label={t('ai.copilot.sendMessage')}
        >
          <Send className="w-4 h-4" />
        </Button>
      </form>
    </div>
  );
};
