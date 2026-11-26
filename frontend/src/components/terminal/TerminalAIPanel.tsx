import React, { useState, useEffect, useRef } from 'react';
import {
  Sparkles,
  Bot,
  User,
  Send,
  Square,
  Trash2,
  X,
  Copy,
  Check,
  Terminal as TerminalIcon,
  ArrowDownLeft,
  ChevronDown,
  ChevronUp,
  HelpCircle,
  Code2,
  Wrench,
} from 'lucide-react';
import { Badge } from '../ui';
import { useTranslation } from '../../i18n/LanguageContext';
import { useAlert } from '../../context/AlertContext';
import { getErrorMessage } from '../../utils/errors';
import {
  AICreateConversation,
  AIListConversations,
  AIListMessages,
  AIStreamMessage,
} from '../../../wailsjs/go/main/App';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import { AIMessageRecord, AIStreamChunkPayload, AIStreamCompletePayload, AIStreamErrorPayload } from '../../types';

export interface TerminalTabInfo {
  id: string;
  serverId: string;
  serverName: string;
  serverHost: string;
}

interface TerminalAIPanelProps {
  isOpen: boolean;
  onClose: () => void;
  activeTab: TerminalTabInfo | null;
  getTerminalBuffer: (maxLines?: number) => string;
  onInsertCommand: (command: string) => void;
  width?: number;
  onResize?: (width: number) => void;
}

export const TerminalAIPanel: React.FC<TerminalAIPanelProps> = ({
  isOpen,
  onClose,
  activeTab,
  getTerminalBuffer,
  onInsertCommand,
  width = 420,
  onResize,
}) => {
  const { t, language } = useTranslation();
  const alert = useAlert();

  const [convId, setConvId] = useState<string | null>(null);
  const [messages, setMessages] = useState<AIMessageRecord[]>([]);
  const [inputPrompt, setInputPrompt] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [includeContext, setIncludeContext] = useState(true);
  const [showBufferPreview, setShowBufferPreview] = useState(false);
  const [copiedCodeIndex, setCopiedCodeIndex] = useState<number | null>(null);
  const [insertedCodeIndex, setInsertedCodeIndex] = useState<number | null>(null);
  const [isResizing, setIsResizing] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const streamConvRef = useRef<string | null>(null);

  const currentBuffer = isOpen && includeContext ? getTerminalBuffer(60) : '';
  const bufferLinesCount = currentBuffer ? currentBuffer.split('\n').filter(Boolean).length : 0;

  const toolNameMap = React.useMemo(() => {
    const map = new Map<string, string>();
    messages.forEach((msg) => {
      if (msg.tool_calls_json) {
        try {
          const calls = JSON.parse(msg.tool_calls_json);
          if (Array.isArray(calls)) {
            calls.forEach((c: any) => {
              if (c.id && c.function?.name) {
                map.set(c.id, c.function.name);
              }
            });
          }
        } catch { }
      }
    });
    return map;
  }, [messages]);

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing || !onResize) return;
      const windowWidth = window.innerWidth;
      const offsetFromRight = windowWidth - e.clientX;
      const maxW = Math.min(900, Math.floor(windowWidth * 0.75));
      const clamped = Math.max(300, Math.min(maxW, offsetFromRight));
      onResize(clamped);
    };

    const handleMouseUp = () => {
      if (isResizing) {
        setIsResizing(false);
        if (width) {
          localStorage.setItem('tamestack_terminal_ai_width', String(width));
        }
      }
    };

    if (isResizing) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    }

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, onResize, width]);

  // Initialize or fetch conversation for the active server/terminal
  const initConversation = async () => {
    if (!activeTab) return;
    try {
      const list = (await AIListConversations()) as any[];
      const termTitlePrefix = `Terminal: ${activeTab.serverName}`;
      const existing = list?.find(
        (c) => c.server_id === activeTab.serverId && c.title.startsWith(termTitlePrefix)
      );

      if (existing) {
        setConvId(existing.id);
        const msgs = (await AIListMessages(existing.id)) as AIMessageRecord[];
        setMessages(msgs || []);
      } else {
        const sysPrompt = t('ai.terminalPanel.systemPromptInit');

        const created = await AICreateConversation(
          `${termTitlePrefix} (${new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })})`,
          activeTab.serverId,
          sysPrompt,
          '',
          ''
        );
        if (created) {
          setConvId(created.id);
          setMessages([]);
        }
      }
    } catch (e) {
      console.error('Failed to init terminal AI conversation:', e);
    }
  };

  useEffect(() => {
    if (isOpen && activeTab) {
      initConversation();
    }
  }, [isOpen, activeTab?.serverId]);

  useEffect(() => {
    streamConvRef.current = convId;
  }, [convId]);

  // Listen to streaming events
  useEffect(() => {
    const unChunk = EventsOn('ai:stream:chunk', (data: AIStreamChunkPayload) => {
      if (data && data.conversation_id === streamConvRef.current) {
        if (data.content_delta) {
          setStreamingContent((prev) => prev + data.content_delta);
        }
      }
    });

    const unComplete = EventsOn('ai:stream:complete', (data: AIStreamCompletePayload) => {
      if (!data || data.conversation_id === streamConvRef.current) {
        setIsStreaming(false);
        setStreamingContent('');
        if (streamConvRef.current) {
          AIListMessages(streamConvRef.current).then((msgs: any) => {
            setMessages(msgs || []);
          });
        }
      }
    });

    const unError = EventsOn('ai:stream:error', (data: AIStreamErrorPayload) => {
      if (!data || data.conversation_id === streamConvRef.current) {
        setIsStreaming(false);
        alert(data?.error || 'Streaming error');
      }
    });

    return () => {
      if (typeof unChunk === 'function') unChunk();
      if (typeof unComplete === 'function') unComplete();
      if (typeof unError === 'function') unError();
    };
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingContent]);

  const handleSendMessage = async (customPrompt?: string) => {
    const textToSend = (customPrompt || inputPrompt).trim();
    if (!textToSend || isStreaming) return;

    if (!convId) {
      await initConversation();
    }
    const currentId = convId || streamConvRef.current;
    if (!currentId) return;

    let fullPrompt = textToSend;
    if (includeContext && currentBuffer) {
      const header = t('ai.terminalPanel.contextHeader');
      fullPrompt = `${header}\n\`\`\`terminal\n${currentBuffer}\n\`\`\`\n\n${textToSend}`;
    }

    setInputPrompt('');
    setIsStreaming(true);
    setStreamingContent('');

    // Optimistically show user message
    const tempUserMsg: AIMessageRecord = {
      id: 'temp_' + Date.now(),
      conversation_id: currentId,
      role: 'user',
      content: fullPrompt,
      prompt_tokens: 0,
      completion_tokens: 0,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, tempUserMsg]);

    try {
      await AIStreamMessage(currentId, fullPrompt);
    } catch (e) {
      setIsStreaming(false);
      alert(getErrorMessage(e));
    }
  };

  const handleClearChat = async () => {
    if (!activeTab) return;
    try {
      const sysPrompt = t('ai.terminalPanel.systemPromptClear');

      const created = await AICreateConversation(
        `Terminal: ${activeTab.serverName} (${new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })})`,
        activeTab.serverId,
        sysPrompt,
        '',
        ''
      );
      if (created) {
        setConvId(created.id);
        setMessages([]);
        setStreamingContent('');
        setIsStreaming(false);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleCopyCode = (code: string, idx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedCodeIndex(idx);
    setTimeout(() => setCopiedCodeIndex(null), 2000);
  };

  const handleInsertCode = (code: string, idx: number) => {
    onInsertCommand(code.trim());
    setInsertedCodeIndex(idx);
    setTimeout(() => setInsertedCodeIndex(null), 1500);
  };

  if (!isOpen) return null;

  const quickPrompts = [
    { label: t('terminal.aiCopilot.quickChipExplainError'), text: t('terminal.aiCopilot.quickChipExplainError') },
    { label: t('terminal.aiCopilot.quickChipPorts'), text: t('terminal.aiCopilot.quickChipPorts') },
    { label: t('terminal.aiCopilot.quickChipDisk'), text: t('terminal.aiCopilot.quickChipDisk') },
    { label: t('terminal.aiCopilot.quickChipRestart'), text: t('terminal.aiCopilot.quickChipRestart') },
  ];

  interface ParsedUserMessage {
    hasContext: boolean;
    contextText: string;
    linesCount: number;
    userPrompt: string;
  }

  const parseUserMessage = (content: string): ParsedUserMessage => {
    const match = content.match(
      /^(\[(?:Контекст вывода терминала|Terminal output context)[^\]]*\]:?\s*```(?:terminal)?\n([\s\S]*?)\n```\s*)/i
    );
    if (match) {
      const contextText = match[2].trim();
      const userPrompt = content.slice(match[1].length).trim();
      const linesCount = contextText ? contextText.split('\n').filter(Boolean).length : 0;
      return {
        hasContext: true,
        contextText,
        linesCount,
        userPrompt,
      };
    }
    return {
      hasContext: false,
      contextText: '',
      linesCount: 0,
      userPrompt: content,
    };
  };

  const parseInline = (text: string): React.ReactNode => {
    const regex = /(`[^`]+`|\*\*[^*]+\*\*|~~[^~]+~~|\*[^*]+\*|_[^_]+_)/g;
    const tokens = text.split(regex);
    return tokens.map((token, i) => {
      if (token.startsWith('`') && token.endsWith('`') && token.length >= 2) {
        return (
          <code
            key={i}
            className="px-1.5 py-0.5 rounded bg-slate-800 text-emerald-300 font-mono text-[11px] border border-slate-700/50 select-text"
          >
            {token.slice(1, -1)}
          </code>
        );
      }
      if (token.startsWith('**') && token.endsWith('**') && token.length >= 4) {
        return (
          <strong key={i} className="font-semibold text-white select-text">
            {token.slice(2, -2)}
          </strong>
        );
      }
      if (token.startsWith('~~') && token.endsWith('~~') && token.length >= 4) {
        return (
          <del key={i} className="line-through text-slate-500 select-text">
            {token.slice(2, -2)}
          </del>
        );
      }
      if (
        (token.startsWith('*') && token.endsWith('*') && token.length >= 2) ||
        (token.startsWith('_') && token.endsWith('_') && token.length >= 2)
      ) {
        return (
          <em key={i} className="italic text-slate-200 select-text">
            {token.slice(1, -1)}
          </em>
        );
      }
      return token;
    });
  };

  const UserMessageView: React.FC<{ content: string }> = ({ content }) => {
    const { t } = useTranslation();
    const [isExpanded, setIsExpanded] = useState(false);
    const parsed = parseUserMessage(content);

    if (!parsed.hasContext) {
      return <div className="whitespace-pre-wrap break-words leading-relaxed">{parseInline(content)}</div>;
    }

    return (
      <div className="space-y-2">
        <div className="text-left">
          <button
            type="button"
            onClick={() => setIsExpanded(!isExpanded)}
            className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-indigo-950/60 hover:bg-indigo-900/80 text-indigo-200 text-[11px] font-medium border border-indigo-400/30 transition cursor-pointer select-none"
          >
            <TerminalIcon className="w-3 h-3 text-indigo-400" />
            <span>{t('terminal.aiCopilot.contextBadge', { count: parsed.linesCount })}</span>
            {isExpanded ? <ChevronUp className="w-3 h-3 text-indigo-400" /> : <ChevronDown className="w-3 h-3 text-indigo-400" />}
          </button>

          {isExpanded && (
            <pre className="mt-1.5 p-2 rounded-lg bg-slate-950/95 border border-indigo-500/30 text-[10px] font-mono text-slate-300 max-h-48 overflow-y-auto whitespace-pre leading-relaxed select-text">
              {parsed.contextText}
            </pre>
          )}
        </div>

        <div className="whitespace-pre-wrap break-words leading-relaxed font-normal">
          {parseInline(parsed.userPrompt)}
        </div>
      </div>
    );
  };

  const ToolMessageView: React.FC<{ message: AIMessageRecord; toolName?: string }> = ({ message, toolName }) => {
    const { t } = useTranslation();
    const [isOpen, setIsOpen] = useState(false);

    let formattedContent = message.content;
    try {
      const parsed = JSON.parse(message.content);
      formattedContent = JSON.stringify(parsed, null, 2);
    } catch {
      // raw string
    }

    const displayName = toolName
      ? `${t('ai.message.toolOutput')}: ${toolName}`
      : t('ai.message.toolOutput');

    return (
      <div className="my-1.5 text-xs">
        <div className="rounded-xl border border-slate-800 bg-slate-950/70 overflow-hidden shadow-xs">
          <button
            type="button"
            onClick={() => setIsOpen(!isOpen)}
            className="w-full flex items-center justify-between px-3 py-2 text-slate-300 hover:text-white hover:bg-slate-900/60 transition cursor-pointer text-left select-none"
          >
            <div className="flex items-center gap-2 min-w-0">
              <div className="w-5 h-5 rounded-md bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 shrink-0">
                <Wrench className="w-3 h-3" />
              </div>
              <span className="font-mono text-emerald-400 text-[11px] truncate font-medium">
                {displayName}
              </span>
              {message.tool_call_id && (
                <span className="text-[10px] text-slate-500 font-mono truncate hidden sm:inline">
                  ({message.tool_call_id.slice(0, 8)})
                </span>
              )}
            </div>
            <div className="text-slate-400 shrink-0 ml-2">
              {isOpen ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5 text-slate-400" />}
            </div>
          </button>

          {isOpen && (
            <pre className="p-3 bg-slate-950 border-t border-slate-800/80 text-[10px] font-mono text-emerald-300/90 max-h-52 overflow-x-auto overflow-y-auto whitespace-pre leading-relaxed select-text">
              {formattedContent}
            </pre>
          )}
        </div>
      </div>
    );
  };

  const isTableRow = (line: string): boolean => {
    const trimmed = line.trim();
    return trimmed.startsWith('|') && trimmed.endsWith('|') && trimmed.length >= 2;
  };

  const isTableSeparator = (line: string): boolean => {
    const trimmed = line.trim();
    if (!trimmed.startsWith('|') || !trimmed.endsWith('|')) return false;
    const inner = trimmed.slice(1, -1);
    return /^[\s|:\-]+$/.test(inner) && inner.includes('-');
  };

  const renderProse = (text: string) => {
    const lines = text.split('\n');
    const elements: React.ReactNode[] = [];
    let i = 0;

    while (i < lines.length) {
      const line = lines[i];

      // Markdown table
      if (isTableRow(line) && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
        const tableHeaderLine = line;
        const tableSepLine = lines[i + 1];
        const tableBodyLines: string[] = [];
        i += 2;
        while (i < lines.length && isTableRow(lines[i])) {
          tableBodyLines.push(lines[i]);
          i++;
        }

        const headerCells = tableHeaderLine
          .trim()
          .slice(1, -1)
          .split('|')
          .map((c) => c.trim());

        const alignments = tableSepLine
          .trim()
          .slice(1, -1)
          .split('|')
          .map((col) => {
            const c = col.trim();
            if (c.startsWith(':') && c.endsWith(':')) return 'center';
            if (c.endsWith(':')) return 'right';
            return 'left';
          });

        const bodyRows = tableBodyLines.map((rowLine) =>
          rowLine
            .trim()
            .slice(1, -1)
            .split('|')
            .map((c) => c.trim())
        );

        elements.push(
          <div
            key={`table-${i}`}
            className="my-2.5 overflow-x-auto rounded-xl border border-slate-800 bg-slate-950/80 shadow-xs select-text"
          >
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="bg-slate-900/90 border-b border-slate-800 text-slate-200">
                  {headerCells.map((h, colIdx) => (
                    <th
                      key={colIdx}
                      style={{ textAlign: (alignments[colIdx] || 'left') as any }}
                      className="px-3 py-1.5 font-semibold text-[10px] uppercase tracking-wider text-slate-200"
                    >
                      {parseInline(h)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-850/70">
                {bodyRows.map((row, rIdx) => (
                  <tr key={rIdx} className="hover:bg-slate-900/40 transition-colors">
                    {headerCells.map((_, colIdx) => (
                      <td
                        key={colIdx}
                        style={{ textAlign: (alignments[colIdx] || 'left') as any }}
                        className="px-3 py-1.5 text-slate-300 text-[11px] leading-relaxed select-text font-normal"
                      >
                        {parseInline(row[colIdx] || '')}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
        continue;
      }

      if (line.startsWith('### ')) {
        elements.push(
          <h3 key={i} className="text-xs font-bold text-indigo-300 mt-2 mb-0.5 select-text">
            {parseInline(line.slice(4))}
          </h3>
        );
        i++;
        continue;
      }
      if (line.startsWith('## ')) {
        elements.push(
          <h2 key={i} className="text-xs font-bold text-white mt-2.5 mb-1 select-text">
            {parseInline(line.slice(3))}
          </h2>
        );
        i++;
        continue;
      }
      if (line.startsWith('# ')) {
        elements.push(
          <h1 key={i} className="text-sm font-bold text-white mt-3 mb-1 select-text">
            {parseInline(line.slice(2))}
          </h1>
        );
        i++;
        continue;
      }

      const numMatch = line.match(/^(\d+)\.\s+(.*)$/);
      if (numMatch) {
        elements.push(
          <div key={i} className="flex gap-2 text-slate-200 ml-1 items-start select-text">
            <span className="font-semibold text-indigo-400 select-none text-[11px] shrink-0 min-w-[16px]">
              {numMatch[1]}.
            </span>
            <div className="flex-1 leading-relaxed">{parseInline(numMatch[2])}</div>
          </div>
        );
        i++;
        continue;
      }

      const bulletMatch = line.match(/^[-*•]\s+(.*)$/);
      if (bulletMatch) {
        elements.push(
          <div key={i} className="flex gap-2 text-slate-200 ml-1 items-start select-text">
            <span className="text-indigo-400 select-none shrink-0">•</span>
            <div className="flex-1 leading-relaxed">{parseInline(bulletMatch[1])}</div>
          </div>
        );
        i++;
        continue;
      }

      if (line.startsWith('> ')) {
        elements.push(
          <blockquote key={i} className="pl-2.5 py-0.5 my-1 border-l-2 border-indigo-500/60 text-slate-400 italic text-xs select-text">
            {parseInline(line.slice(2))}
          </blockquote>
        );
        i++;
        continue;
      }

      if (line.trim() === '') {
        elements.push(<div key={i} className="h-1.5" />);
        i++;
        continue;
      }

      elements.push(
        <p key={i} className="text-slate-200 leading-relaxed select-text">
          {parseInline(line)}
        </p>
      );
      i++;
    }

    return <div className="space-y-1.5 select-text">{elements}</div>;
  };

  const renderAssistantContent = (content: string) => {
    const parts = content.split(/(```[\s\S]*?```)/g);
    let codeIndex = 0;

    return (
      <div className="space-y-2 text-xs leading-relaxed">
        {parts.map((part, pIdx) => {
          if (part.startsWith('```') && part.endsWith('```')) {
            const lines = part.slice(3, -3).trim().split('\n');
            const lang = lines[0].trim().toLowerCase() || 'bash';
            const code = (lang === 'bash' || lang === 'sh' || lang === 'zsh' || lines.length > 1)
              ? lines.slice(1).join('\n')
              : lines.join('\n');
            const curIdx = codeIndex++;

            return (
              <div
                key={pIdx}
                className="my-2.5 rounded-xl overflow-hidden border border-slate-800 bg-slate-950/90 font-mono shadow-xs"
              >
                <div className="flex items-center justify-between px-3 py-1.5 bg-slate-900/90 border-b border-slate-800/80 text-[11px] text-slate-400">
                  <span className="flex items-center gap-1.5 font-medium text-slate-300">
                    <TerminalIcon className="w-3.5 h-3.5 text-indigo-400" />
                    <span>{lang}</span>
                  </span>
                  <div className="flex items-center gap-1.5">
                    <button
                      type="button"
                      onClick={() => handleCopyCode(code, curIdx)}
                      className="px-2 py-0.5 rounded hover:bg-slate-800 text-slate-400 hover:text-white transition flex items-center gap-1 text-[10px]"
                    >
                      {copiedCodeIndex === curIdx ? (
                        <>
                          <Check className="w-3 h-3 text-emerald-400" />
                          <span className="text-emerald-400">{t('ai.message.copied')}</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3 h-3" />
                          <span>{t('ai.message.copy')}</span>
                        </>
                      )}
                    </button>

                    <button
                      type="button"
                      onClick={() => handleInsertCode(code, curIdx)}
                      className={`px-2.5 py-0.5 rounded-lg border transition flex items-center gap-1.5 text-[10px] font-semibold cursor-pointer shadow-2xs ${insertedCodeIndex === curIdx
                        ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/50'
                        : 'bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 hover:text-emerald-300 border border-emerald-500/30'
                        }`}
                      title={t('terminal.aiCopilot.insertTooltip')}
                    >
                      {insertedCodeIndex === curIdx ? (
                        <>
                          <Check className="w-3 h-3 text-emerald-400" />
                          <span>{t('terminal.aiCopilot.insertedSuccessShort')}</span>
                        </>
                      ) : (
                        <>
                          <ArrowDownLeft className="w-3 h-3" />
                          <span>{t('terminal.aiCopilot.insertBtn')}</span>
                        </>
                      )}
                    </button>
                  </div>
                </div>
                <pre className="p-3 text-[11px] overflow-x-auto text-emerald-300 font-mono leading-relaxed select-text">
                  <code>{code}</code>
                </pre>
              </div>
            );
          }

          return <div key={pIdx}>{renderProse(part)}</div>;
        })}
      </div>
    );
  };

  return (
    <>
      <div
        style={{ width: `${width}px` }}
        className="relative max-w-[85vw] h-full flex flex-col bg-surface-900 border-l border-slate-800 shrink-0 z-20 shadow-xl transition-all duration-75"
      >
        {/* Resize Handle */}
        <div
          onMouseDown={(e) => {
            e.preventDefault();
            setIsResizing(true);
          }}
          className={`absolute top-0 -left-1 bottom-0 w-2 cursor-col-resize select-none z-30 hover:bg-indigo-500/60 active:bg-indigo-600 transition-colors ${isResizing ? 'bg-indigo-600' : 'bg-transparent'
            }`}
          title={t('ai.resizeTooltip')}
        />

        {/* Panel Header */}
        <div className="px-4 py-3 bg-surface-850 border-b border-slate-800 flex items-center justify-between gap-2 shrink-0">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-7 h-7 rounded-lg bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
              <Sparkles className="w-4 h-4" />
            </div>
            <div className="min-w-0">
              <h3 className="text-xs font-bold text-white truncate flex items-center gap-1.5">
                <span>{t('terminal.aiCopilot.title')}</span>
                {activeTab && (
                  <Badge variant="neutral" size="sm" className="font-mono text-[9px] py-0 px-1">
                    {activeTab.serverName}
                  </Badge>
                )}
              </h3>
              <p className="text-[10px] text-slate-400 truncate">
                {t('terminal.aiCopilot.subtitle')}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-1 shrink-0">
            <button
              type="button"
              onClick={handleClearChat}
              className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
              title={t('terminal.aiCopilot.clearChatBtn')}
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={onClose}
              className="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
              title={t('terminal.aiCopilot.closeTooltip')}
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Terminal Context Bar */}
        <div className="px-3 py-2 bg-slate-950/60 border-b border-slate-800/80 text-[11px] space-y-1.5 shrink-0">
          <div className="flex items-center justify-between gap-2">
            <label className="flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={includeContext}
                onChange={(e) => setIncludeContext(e.target.checked)}
                className="w-3.5 h-3.5 rounded bg-slate-800 border-slate-700 text-indigo-600 focus:ring-0 focus:ring-offset-0 cursor-pointer"
              />
              <span className="text-slate-300 font-medium text-[11px]">
                {t('terminal.aiCopilot.includeContext')}
              </span>
            </label>

            {includeContext && bufferLinesCount > 0 ? (
              <button
                type="button"
                onClick={() => setShowBufferPreview(!showBufferPreview)}
                className="text-emerald-400 hover:text-emerald-300 font-mono text-[10px] flex items-center gap-1 cursor-pointer transition"
              >
                <span>{t('terminal.aiCopilot.capturedLines', { count: bufferLinesCount })}</span>
                {showBufferPreview ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
              </button>
            ) : (
              <span className="text-slate-500 text-[10px] italic">
                {t('terminal.aiCopilot.noContext')}
              </span>
            )}
          </div>

          {showBufferPreview && includeContext && currentBuffer && (
            <div className="mt-1 p-2 bg-slate-950 rounded-lg border border-slate-800 text-[10px] font-mono text-slate-400 max-h-28 overflow-y-auto whitespace-pre-wrap leading-tight">
              {currentBuffer}
            </div>
          )}
        </div>

        {/* Messages List Area */}
        <div className="flex-1 p-3 overflow-y-auto space-y-3 min-h-0 bg-surface-900/60 select-text">
          {messages.length === 0 && !streamingContent && (
            <div className="py-6 px-2 text-center space-y-4">
              <div className="w-10 h-10 rounded-2xl bg-indigo-600/10 border border-indigo-500/20 text-indigo-400 flex items-center justify-center mx-auto">
                <Code2 className="w-5 h-5" />
              </div>
              <div className="space-y-1">
                <h4 className="text-xs font-semibold text-slate-200">
                  {t('terminal.aiCopilot.title')}
                </h4>
                <p className="text-[11px] text-slate-400 leading-relaxed max-w-xs mx-auto">
                  {t('terminal.aiCopilot.includeContextDesc')}
                </p>
              </div>

              {/* Quick suggestions chips */}
              <div className="space-y-1.5 pt-2 text-left">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-500 px-1">
                  {t('terminal.aiCopilot.quickQuestions')}
                </span>
                <div className="flex flex-col gap-1.5">
                  {quickPrompts.map((q, idx) => (
                    <button
                      key={idx}
                      type="button"
                      onClick={() => handleSendMessage(q.text)}
                      className="w-full text-left p-2 rounded-lg bg-surface-850 hover:bg-slate-800/80 border border-slate-800 hover:border-slate-700/80 text-[11px] text-slate-300 hover:text-white transition flex items-center justify-between group"
                    >
                      <span className="truncate">{q.label}</span>
                      <ArrowDownLeft className="w-3 h-3 text-slate-500 group-hover:text-emerald-400 shrink-0" />
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {messages
            .filter((m) => {
              // Hide empty assistant messages that only invoked tools without text content
              if (m.role === 'assistant' && (!m.content || !m.content.trim())) {
                return false;
              }
              return true;
            })
            .map((m) => {
              if (m.role === 'tool') {
                const toolName = m.tool_call_id ? toolNameMap.get(m.tool_call_id) : undefined;
                return <ToolMessageView key={m.id} message={m} toolName={toolName} />;
              }

              const isUser = m.role === 'user';
              return (
                <div
                  key={m.id}
                  className={`flex gap-2.5 ${isUser ? 'justify-end' : 'justify-start'}`}
                >
                  {!isUser && (
                    <div className="w-6 h-6 rounded-md bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0 mt-0.5">
                      <Bot className="w-3.5 h-3.5" />
                    </div>
                  )}
                  <div
                    className={`max-w-[88%] rounded-xl p-3 text-xs shadow-xs select-text cursor-text ${
                      isUser
                        ? 'bg-indigo-600 text-white rounded-tr-none'
                        : 'bg-surface-850 border border-slate-800 text-slate-200 rounded-tl-none'
                    }`}
                  >
                    {isUser ? <UserMessageView content={m.content} /> : renderAssistantContent(m.content)}
                  </div>
                  {isUser && (
                    <div className="w-6 h-6 rounded-md bg-slate-800 border border-slate-700 flex items-center justify-center text-slate-400 shrink-0 mt-0.5 select-none">
                      <User className="w-3.5 h-3.5" />
                    </div>
                  )}
                </div>
              );
            })}

          {/* Live Streaming Content */}
          {isStreaming && streamingContent && (
            <div className="flex gap-2.5 justify-start">
              <div className="w-6 h-6 rounded-md bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0 mt-0.5 animate-pulse select-none">
                <Bot className="w-3.5 h-3.5" />
              </div>
              <div className="max-w-[88%] rounded-xl p-3 text-xs bg-surface-850 border border-slate-800 text-slate-200 rounded-tl-none shadow-xs select-text cursor-text">
                {renderAssistantContent(streamingContent)}
                <span className="inline-block w-1.5 h-3 ml-1 bg-indigo-400 animate-pulse align-middle" />
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Input Bar */}
        <div className="p-3 bg-surface-850 border-t border-slate-800 shrink-0 space-y-2">
          <div className="relative rounded-xl bg-slate-950/80 border border-slate-800 focus-within:border-indigo-500/80 focus-within:ring-1 focus-within:ring-indigo-500/30 transition-all p-2 flex flex-col gap-2">
            <textarea
              ref={textareaRef}
              rows={2}
              value={inputPrompt}
              onChange={(e) => setInputPrompt(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  handleSendMessage();
                }
              }}
              placeholder={t('terminal.aiCopilot.promptPlaceholder')}
              className="w-full bg-transparent text-xs text-slate-100 placeholder-slate-500 focus:outline-none resize-none leading-relaxed"
            />

            <div className="flex items-center justify-between pt-1 border-t border-slate-800/60 text-[10px] text-slate-500">
              <span className="flex items-center gap-1">
                <span>Enter</span>
                <span className="text-slate-600">↵</span>
                <span className="ml-1 text-slate-600">({t('terminal.aiCopilot.sendHint')})</span>
              </span>

              {isStreaming ? (
                <button
                  type="button"
                  onClick={() => setIsStreaming(false)}
                  className="flex items-center gap-1 px-2.5 py-1 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-semibold transition"
                >
                  <Square className="w-3 h-3 fill-current" />
                  <span>{t('ai.stop')}</span>
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => handleSendMessage()}
                  disabled={!inputPrompt.trim() || isStreaming}
                  className="flex items-center gap-1.5 px-3 py-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:hover:bg-indigo-600 text-white rounded-lg text-xs font-semibold shadow-xs shadow-indigo-600/20 transition cursor-pointer"
                >
                  <Send className="w-3 h-3" />
                  <span>{t('ai.send')}</span>
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
};
