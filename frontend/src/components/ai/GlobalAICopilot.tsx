import React, { useEffect, useRef, useState } from 'react';
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
} from 'lucide-react';
import { Badge, Button } from '../ui';
import { MarkdownRenderer } from './MarkdownRenderer';
import { useTranslation } from '../../i18n/LanguageContext';
import { useServer } from '../../context/ServerContext';
import { getErrorMessage } from '../../utils/errors';
import {
  AICreateConversation,
  AIListMessages,
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

const getTabSuggestions = (tab: NavigationTab, isEN: boolean): string[] => {
  switch (tab) {
    case 'sites':
      return isEN
        ? ['How to set up Node.js/Python with Reverse Proxy?', 'Check why a site returns 502/500']
        : ['Как поднять Node.js/Python за Reverse Proxy?', 'Проверь причины ошибки 502/500'];
    case 'mail':
      return isEN
        ? ['Which DNS records are needed for mail?', 'How to verify DKIM, SPF and DMARC?']
        : ['Какие DNS-записи нужны для почты?', 'Как проверить DKIM, SPF и DMARC?'];
    case 'dns':
      return isEN
        ? ['How to configure A, MX and TXT records?', 'Check domain DNS resolution on the server']
        : ['Как настроить A, MX и TXT записи?', 'Проверь резолвинг домена на сервере'];
    case 'users':
      return isEN
        ? ['How does SFTP chroot isolation work?', 'How to grant a user access to a site folder?']
        : ['Как работает SFTP chroot-изоляция?', 'Как выдать пользователю права на папку сайта?'];
    case 'security':
      return isEN
        ? ['Audit open ports and Fail2ban jails', 'What hardening steps should I enable?']
        : ['Проверь открытые порты и Fail2ban', 'Что стоит включить для усиления защиты?'];
    case 'databases':
      return isEN
        ? ['Check MySQL/PostgreSQL service status', 'How to safely back up and restore a DB?']
        : ['Проверь статус MySQL/PostgreSQL', 'Как безопасно сделать бэкап и восстановление БД?'];
    case 'docker':
      return isEN
        ? ['Inspect running containers and logs', 'How to free disk space from unused images?']
        : ['Проверь запущенные контейнеры и логи', 'Как очистить место от неиспользуемых образов?'];
    default:
      return isEN
        ? ['What can I do in this section?', 'Inspect current server health & logs']
        : ['Что можно сделать в этом разделе?', 'Проверь общее состояние сервера и логи'];
  }
};

export const GlobalAICopilot: React.FC<GlobalAICopilotProps> = ({
  isOpen,
  onClose,
  currentTab,
  onOpenFullAI,
}) => {
  const { t, language } = useTranslation();
  const isEN = language === 'en';
  const { selectedServer, selectedServerId } = useServer();

  const [convId, setConvId] = useState<string | null>(null);
  const [messages, setMessages] = useState<AIMessageRecord[]>([]);
  const [input, setInput] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [includeContext, setIncludeContext] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const streamConvRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

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

  if (!isOpen) return null;

  const tabTitle = t(`titles.${currentTab}.title`);
  const tabSubtitle = t(`titles.${currentTab}.subtitle`);
  const suggestions = getTabSuggestions(currentTab, isEN);

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
    <div className="fixed inset-y-0 right-0 z-40 w-full sm:w-[440px] bg-surface-950 border-l border-slate-800 shadow-2xl shadow-black/80 flex flex-col">
      {/* Header */}
      <div className="h-14 px-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/80 shrink-0">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/15 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
            <Sparkles className="w-4 h-4" />
          </div>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-white truncate">TameStack AI Copilot</div>
            <div className="text-[11px] text-slate-400 truncate">
              {isEN ? 'Context-aware assistant' : 'Контекстный помощник'}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="sm"
            onClick={startNewChat}
            title={isEN ? 'New conversation' : 'Новый диалог'}
            aria-label={isEN ? 'New conversation' : 'Новый диалог'}
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
            title={isEN ? 'Open full AI view' : 'Открыть полный AI-чат'}
            aria-label={isEN ? 'Open full AI view' : 'Открыть полный AI-чат'}
          >
            <Maximize2 className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            title={isEN ? 'Close Copilot (Ctrl+I)' : 'Закрыть (Ctrl+I)'}
            aria-label={isEN ? 'Close Copilot' : 'Закрыть'}
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
          <span>{isEN ? 'Screen context' : 'Контекст экрана'}</span>
        </label>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.length === 0 && !streamingContent && (
          <div className="py-6 space-y-4">
            <div className="text-center space-y-1.5">
              <Bot className="w-8 h-8 text-indigo-400 mx-auto" />
              <p className="text-xs font-medium text-slate-200">
                {isEN
                  ? 'Ask about TameStack features, current screen, or server diagnostics'
                  : 'Спросите о возможностях TameStack, текущем разделе или диагностике сервера'}
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

        {messages.map((m) => (
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
                <MarkdownRenderer content={m.content} />
              )}
            </div>
            {m.role === 'user' && (
              <div className="w-6 h-6 rounded-md bg-slate-800 text-slate-300 flex items-center justify-center shrink-0 mt-0.5">
                <User className="w-3.5 h-3.5" />
              </div>
            )}
          </div>
        ))}

        {streamingContent && (
          <div className="flex gap-2.5 text-xs justify-start">
            <div className="w-6 h-6 rounded-md bg-indigo-500/20 text-indigo-400 flex items-center justify-center shrink-0 mt-0.5">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            </div>
            <div className="max-w-[85%] rounded-xl px-3 py-2 bg-slate-900 border border-slate-800 text-slate-200 leading-relaxed">
              <MarkdownRenderer content={streamingContent} />
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
          placeholder={isEN ? 'Ask AI anything... (Enter to send)' : 'Спросите AI... (Enter — отправить)'}
          className="flex-1 resize-none rounded-xl bg-slate-900 border border-slate-800 px-3 py-2 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-indigo-500"
        />
        <Button
          type="submit"
          variant="primary"
          size="md"
          disabled={isStreaming || !input.trim()}
          loading={isStreaming}
          className="px-4 self-stretch"
          title={isEN ? 'Send message' : 'Отправить сообщение'}
          aria-label={isEN ? 'Send message' : 'Отправить сообщение'}
        >
          <Send className="w-4 h-4" />
        </Button>
      </form>
    </div>
  );
};
