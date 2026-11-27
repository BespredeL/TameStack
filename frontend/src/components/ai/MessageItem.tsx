import React, { useState } from 'react';
import { AIMessageRecord } from '../../types';
import { Bot, User, Wrench, Check, Copy, ChevronDown, ChevronRight, Terminal, ShieldAlert, X } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';
import { usePrivacy } from '../../context/PrivacyContext';
import { AIApproveTask, AIRejectTask } from '../../../wailsjs/go/main/App';
import { DiagnosticReportCard } from './DiagnosticReportCard';

interface MessageItemProps {
  message: AIMessageRecord;
  isStreaming?: boolean;
  onApprovalComplete?: () => void;
  toolName?: string;
}

export const MessageItem: React.FC<MessageItemProps> = ({
  message,
  isStreaming = false,
  onApprovalComplete,
  toolName,
}) => {
  const { t } = useTranslation();
  const { maskText } = usePrivacy();
  const [copiedCodeIndex, setCopiedCodeIndex] = useState<number | null>(null);
  const [isCopiedMessage, setIsCopiedMessage] = useState(false);
  const [isToolOpen, setIsToolOpen] = useState(false);
  const [approvalStatus, setApprovalStatus] = useState<'pending' | 'approved' | 'rejected'>('pending');

  const handleCopyMessage = () => {
    navigator.clipboard.writeText(message.content);
    setIsCopiedMessage(true);
    setTimeout(() => setIsCopiedMessage(false), 2000);
  };

  const isUser = message.role === 'user';
  const isTool = message.role === 'tool';
  const isAssistant = message.role === 'assistant';

  // Do not render empty assistant messages (e.g. intermediate tool-calling steps)
  if (isAssistant && (!message.content || !message.content.trim())) {
    return null;
  }

  const handleCopy = (text: string, idx: number) => {
    navigator.clipboard.writeText(text);
    setCopiedCodeIndex(idx);
    setTimeout(() => setCopiedCodeIndex(null), 2000);
  };

  // Render tool result messages
  if (isTool) {
    let parsedData: any = null;
    try {
      parsedData = JSON.parse(message.content);
    } catch (e) {
      parsedData = message.content;
    }

    const isApprovalRequired = parsedData && parsedData.status === 'awaiting_approval';
    const isDiagnosticReport = parsedData && typeof parsedData === 'object' && parsedData.workflow_type;

    if (isDiagnosticReport) {
      return (
        <div className="py-3 px-4 bg-slate-950/30">
          <div className="max-w-4xl xl:max-w-5xl 2xl:max-w-6xl mx-auto">
            <DiagnosticReportCard report={parsedData} />
          </div>
        </div>
      );
    }

    const displayName = toolName
      ? `${t('ai.message.toolOutput')}: ${toolName}`
      : t('ai.message.toolOutput');

    const renderToolOutput = (data: any) => {
      if (!data) return null;

      if (typeof data === 'string') {
        return (
          <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-[11px] text-slate-300 whitespace-pre-wrap overflow-x-auto leading-relaxed select-text">
            {maskText(data)}
          </pre>
        );
      }

      if (typeof data === 'object') {
        // Ports inspection tool output
        if (data.ports_raw) {
          return (
            <div className="space-y-2">
              {data.server_id && (
                <div className="text-[10px] text-slate-400 font-mono">
                  Server ID: <span className="text-slate-300 font-medium">{maskText(String(data.server_id))}</span>
                </div>
              )}
              <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-[11px] text-emerald-300 whitespace-pre overflow-x-auto leading-relaxed select-text">
                {maskText(String(data.ports_raw))}
              </pre>
            </div>
          );
        }

        // Generic stdout/output text
        if (data.output || data.stdout) {
          const text = data.output || data.stdout;
          return (
            <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-[11px] text-emerald-300 whitespace-pre-wrap overflow-x-auto leading-relaxed select-text">
              {maskText(typeof text === 'string' ? text : JSON.stringify(text, null, 2))}
            </pre>
          );
        }

        // Generic structured JSON
        return (
          <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-[11px] text-slate-300 whitespace-pre overflow-x-auto leading-relaxed select-text">
            {maskText(JSON.stringify(data, null, 2))}
          </pre>
        );
      }

      return (
        <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 font-mono text-[11px] text-slate-300 whitespace-pre-wrap overflow-x-auto leading-relaxed select-text">
          {maskText(String(data))}
        </pre>
      );
    };

    return (
      <div className="py-2.5 px-4 bg-slate-950/25">
        <div className="max-w-4xl xl:max-w-5xl 2xl:max-w-6xl mx-auto flex gap-4">
          <div
            className={`w-8 h-8 rounded-lg border flex items-center justify-center shrink-0 shadow-xs ${
              isApprovalRequired
                ? 'bg-amber-500/10 border-amber-500/30 text-amber-400'
                : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
            }`}
          >
            {isApprovalRequired ? <ShieldAlert className="w-4 h-4" /> : <Wrench className="w-4 h-4" />}
          </div>

          <div className="flex-1 min-w-0">
            {isApprovalRequired ? (
              <div className="space-y-3 bg-slate-900/60 border border-slate-800 rounded-xl p-3.5">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-amber-300 flex items-center gap-1.5 text-xs">
                    <ShieldAlert className="w-4 h-4 text-amber-400" />
                    <span>{t('ai.message.approvalRequired', { title: parsedData.title || parsedData.description || t('ai.message.aiAction') })}</span>
                  </span>
                  <span className="text-[10px] text-slate-500 font-mono">Task ID: {parsedData.task_id}</span>
                </div>

                {parsedData.diff && (
                  <div className="bg-slate-950 p-2.5 rounded-lg border border-slate-800 font-mono text-[11px] max-h-48 overflow-y-auto">
                    {parsedData.diff.split('\n').map((l: string, idx: number) => {
                      let col = 'text-slate-400';
                      if (l.startsWith('+')) col = 'text-emerald-300 bg-emerald-950/20';
                      else if (l.startsWith('-')) col = 'text-rose-300 bg-rose-950/20';
                      return <div key={idx} className={col}>{l}</div>;
                    })}
                  </div>
                )}

                {approvalStatus === 'pending' ? (
                  <div className="flex items-center gap-2 pt-1">
                    <button
                      onClick={async () => {
                        if (parsedData.task_id) {
                          await AIApproveTask(parsedData.task_id);
                          setApprovalStatus('approved');
                          if (onApprovalComplete) onApprovalComplete();
                        }
                      }}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition-colors cursor-pointer"
                    >
                      <Check className="w-3.5 h-3.5" />
                      <span>{t('ai.message.approveAndRun')}</span>
                    </button>

                    <button
                      onClick={async () => {
                        if (parsedData.task_id) {
                          await AIRejectTask(parsedData.task_id);
                          setApprovalStatus('rejected');
                          if (onApprovalComplete) onApprovalComplete();
                        }
                      }}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-700 hover:bg-slate-800 text-slate-300 hover:text-white text-xs transition-colors cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5" />
                      <span>{t('ai.message.reject')}</span>
                    </button>
                  </div>
                ) : (
                  <div className="text-xs font-semibold py-1">
                    {approvalStatus === 'approved' ? (
                      <span className="text-emerald-400 flex items-center gap-1.5">
                        <Check className="w-4 h-4" /> {t('ai.message.taskApproved')}
                      </span>
                    ) : (
                      <span className="text-rose-400 flex items-center gap-1.5">
                        <X className="w-4 h-4" /> {t('ai.message.taskRejected')}
                      </span>
                    )}
                  </div>
                )}
              </div>
            ) : (
              <div className="rounded-xl border border-slate-800/90 bg-slate-900/50 overflow-hidden shadow-xs">
                <button
                  type="button"
                  onClick={() => setIsToolOpen(!isToolOpen)}
                  className="w-full flex items-center justify-between px-3.5 py-2 hover:bg-slate-850/80 transition cursor-pointer text-left select-none group"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="font-mono text-emerald-400 text-xs font-medium truncate">
                      {displayName}
                    </span>
                    {message.tool_call_id && (
                      <span className="text-[10px] text-slate-500 font-mono hidden sm:inline truncate">
                        ({message.tool_call_id.slice(0, 8)})
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1.5 text-slate-400 group-hover:text-slate-200">
                    <span className="text-[11px] text-slate-500 font-sans">
                      {isToolOpen ? (t('common.collapse') || 'Свернуть') : (t('common.details') || 'Подробнее')}
                    </span>
                    {isToolOpen ? <ChevronDown className="w-4 h-4 text-slate-400" /> : <ChevronRight className="w-4 h-4 text-slate-400" />}
                  </div>
                </button>

                {isToolOpen && (
                  <div className="border-t border-slate-800/80 p-3 bg-slate-950/80">
                    {renderToolOutput(parsedData)}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  const parseInline = (text: string): React.ReactNode => {
    // Process code, bold, strikethrough, italic
    const regex = /(`[^`]+`|\*\*[^*]+\*\*|~~[^~]+~~|\*[^*]+\*|_[^_]+_)/g;
    const tokens = text.split(regex);
    return tokens.map((token, i) => {
      if (token.startsWith('`') && token.endsWith('`') && token.length >= 2) {
        return (
          <code
            key={i}
            className="px-1.5 py-0.5 rounded bg-slate-800 text-indigo-300 font-mono text-[11px] border border-slate-700/50 select-text"
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

  // Parse Markdown sections: code blocks, tables, headings, lists, bold, blockquotes
  const renderFormattedContent = (content: string) => {
    const parts = content.split(/(```[\s\S]*?```)/g);
    let codeBlockIdx = 0;

    return parts.map((part, index) => {
      if (part.startsWith('```') && part.endsWith('```')) {
        const lines = part.slice(3, -3).trim().split('\n');
        const lang = lines[0].trim().toLowerCase() || 'bash';
        const code = lines.slice(1).join('\n');
        const currentIdx = codeBlockIdx++;

        return (
          <div key={index} className="my-3 rounded-lg overflow-hidden border border-slate-800 bg-slate-950 font-mono text-xs select-text">
            <div className="flex items-center justify-between px-3 py-1.5 bg-slate-900/90 border-b border-slate-800 text-slate-400 text-[11px] select-none">
              <span className="flex items-center gap-1.5 font-semibold text-slate-300">
                <Terminal className="w-3.5 h-3.5 text-indigo-400" />
                {lang}
              </span>
              <button
                onClick={() => handleCopy(code, currentIdx)}
                className="flex items-center gap-1 px-2 py-0.5 rounded hover:bg-slate-800 text-slate-400 hover:text-white transition-colors cursor-pointer"
              >
                {copiedCodeIndex === currentIdx ? (
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
            </div>
            <pre className="p-3 overflow-x-auto text-slate-200 leading-relaxed font-mono select-text">
              <code>{code}</code>
            </pre>
          </div>
        );
      }

      // Process regular prose: split by line and parse tables, headings, lists
      const lines = part.split('\n');
      const elements: React.ReactNode[] = [];
      let i = 0;

      while (i < lines.length) {
        const line = lines[i];

        // Check if markdown table begins
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
              className="my-3 overflow-x-auto rounded-xl border border-slate-800 bg-slate-950/80 shadow-xs select-text"
            >
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="bg-slate-900/90 border-b border-slate-800 text-slate-200">
                    {headerCells.map((h, colIdx) => (
                      <th
                        key={colIdx}
                        style={{ textAlign: (alignments[colIdx] || 'left') as any }}
                        className="px-3.5 py-2 font-semibold text-[11px] uppercase tracking-wider text-slate-200"
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
                          className="px-3.5 py-2 text-slate-300 text-xs leading-relaxed select-text font-normal"
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

        // Headings
        if (line.startsWith('### ')) {
          elements.push(
            <h3 key={i} className="text-sm font-bold text-white mt-3 mb-1 select-text">
              {parseInline(line.slice(4))}
            </h3>
          );
          i++;
          continue;
        }
        if (line.startsWith('## ')) {
          elements.push(
            <h2 key={i} className="text-base font-bold text-white mt-4 mb-2 select-text">
              {parseInline(line.slice(3))}
            </h2>
          );
          i++;
          continue;
        }
        if (line.startsWith('# ')) {
          elements.push(
            <h1 key={i} className="text-lg font-bold text-white mt-4 mb-2 select-text">
              {parseInline(line.slice(2))}
            </h1>
          );
          i++;
          continue;
        }

        // Numbered lists
        const numMatch = line.match(/^(\d+)\.\s+(.*)$/);
        if (numMatch) {
          elements.push(
            <div key={i} className="flex gap-2 text-slate-300 ml-2 items-start select-text">
              <span className="font-semibold text-indigo-400 select-none text-xs shrink-0 min-w-[16px]">
                {numMatch[1]}.
              </span>
              <div className="flex-1 leading-relaxed">{parseInline(numMatch[2])}</div>
            </div>
          );
          i++;
          continue;
        }

        // Bullet lists
        if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('• ')) {
          elements.push(
            <div key={i} className="flex gap-2 text-slate-300 ml-2 items-start select-text">
              <span className="text-indigo-400 select-none shrink-0">•</span>
              <div className="flex-1 leading-relaxed">{parseInline(line.slice(2))}</div>
            </div>
          );
          i++;
          continue;
        }

        // Blockquotes
        if (line.startsWith('> ')) {
          elements.push(
            <blockquote key={i} className="pl-3 py-1 my-1 border-l-2 border-indigo-500/60 text-slate-400 italic text-xs select-text">
              {parseInline(line.slice(2))}
            </blockquote>
          );
          i++;
          continue;
        }

        // Empty lines
        if (line.trim() === '') {
          elements.push(<div key={i} className="h-2" />);
          i++;
          continue;
        }

        // Regular paragraph
        elements.push(
          <p key={i} className="text-slate-300 leading-relaxed select-text">
            {parseInline(line)}
          </p>
        );
        i++;
      }

      return (
        <div key={index} className="space-y-1.5 select-text">
          {elements}
        </div>
      );
    });
  };

  return (
    <div className={`py-4 px-4 transition-colors select-text ${isUser ? 'bg-slate-900/30' : 'bg-slate-950/40'}`}>
      <div className="max-w-4xl xl:max-w-5xl 2xl:max-w-6xl mx-auto flex gap-4 select-text">
        {/* Avatar */}
        <div
          className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 shadow-md select-none ${
            isUser
              ? 'bg-gradient-to-br from-blue-600 to-indigo-600 text-white shadow-blue-500/20'
              : 'bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-purple-500/20'
          }`}
        >
          {isUser ? <User className="w-4 h-4" /> : <Bot className="w-4 h-4" />}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0 select-text">
          <div className="flex items-center justify-between gap-2 mb-1.5 select-none">
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-white">
                {isUser ? t('ai.message.youUser') : t('ai.message.assistant')}
              </span>
              {message.prompt_tokens > 0 && (
                <span className="text-[10px] text-slate-500 bg-slate-800 px-1.5 py-0.2 rounded font-mono">
                  {message.prompt_tokens + message.completion_tokens} tok
                </span>
              )}
            </div>

            <button
              type="button"
              onClick={handleCopyMessage}
              className="flex items-center gap-1 px-2 py-0.5 rounded hover:bg-slate-800 text-slate-500 hover:text-slate-300 text-[11px] transition-colors cursor-pointer select-none"
              title={t('ai.message.copy') || 'Копировать'}
            >
              {isCopiedMessage ? (
                <>
                  <Check className="w-3.5 h-3.5 text-emerald-400" />
                  <span className="text-emerald-400">{t('ai.message.copied')}</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span className="hidden sm:inline">{t('ai.message.copy')}</span>
                </>
              )}
            </button>
          </div>

          <div className="text-xs space-y-2 select-text cursor-text">
            {renderFormattedContent(maskText(message.content))}
            {isStreaming && (
              <span className="inline-block w-2 h-3.5 bg-indigo-400 animate-pulse ml-0.5 align-middle rounded-sm select-none" />
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

