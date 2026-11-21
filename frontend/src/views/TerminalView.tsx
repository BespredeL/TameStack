import React, { useState, useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';
import {
  Terminal as TerminalIcon,
  Plus,
  X,
  Trash2,
  AlertTriangle,
  Play,
  Sparkles,
  ClipboardPaste,
  Copy,
  CheckSquare,
} from 'lucide-react';
import { ServerRecord } from '../types';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { PageToolbar, Button, Card, EmptyState } from '../components/ui';
import { TerminalAIPanel } from '../components/terminal/TerminalAIPanel';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { useServer } from '../context/ServerContext';
import {
  StartTerminalSession,
  WriteTerminalInput,
  ResizeTerminal,
  CloseTerminalSession,
} from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff, ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime';

const getClipboardText = async (): Promise<string> => {
  try {
    if (typeof ClipboardGetText === 'function') {
      const text = await ClipboardGetText();
      if (text) return text;
    }
  } catch (e) {
    // fallback
  }
  try {
    if (navigator?.clipboard?.readText) {
      return await navigator.clipboard.readText();
    }
  } catch (e) {
    console.warn('Clipboard read error:', e);
  }
  return '';
};

const setClipboardText = async (text: string): Promise<void> => {
  try {
    if (typeof ClipboardSetText === 'function') {
      await ClipboardSetText(text);
      return;
    }
  } catch (e) {
    // fallback
  }
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    }
  } catch (e) {
    console.warn('Clipboard write error:', e);
  }
};

interface TerminalTab {
  id: string; // Session ID
  serverId: string;
  serverName: string;
  serverHost: string;
}

interface TerminalViewProps {
  servers: ServerRecord[];
  onOpenAddServer: () => void;
  onNavigateToAI?: (serverId?: string, prompt?: string) => void;
}

export const TerminalView: React.FC<TerminalViewProps> = ({
  servers,
  onOpenAddServer,
  onNavigateToAI,
}) => {
  const [tabs, setTabs] = useState<TerminalTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const { selectedServerId, setSelectedServerId } = useServer();
  const [isStarting, setIsStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; hasSelection: boolean } | null>(null);
  const [isAIPanelOpen, setIsAIPanelOpen] = useState(false);
  const [aiPanelWidth, setAiPanelWidth] = useState<number>(() => {
    const saved = localStorage.getItem('tamestack_terminal_ai_width');
    const parsed = saved ? parseInt(saved, 10) : NaN;
    return isNaN(parsed) ? 420 : Math.max(300, Math.min(800, parsed));
  });

  // References to keep active xterm instances and fit addons
  const terminalRefs = useRef<Record<string, { term: Terminal; fitAddon: FitAddon }>>({});
  const containerRefs = useRef<Record<string, HTMLDivElement | null>>({});

  // Extracts current terminal screen and scrollback buffer as clean text
  const getTerminalBufferText = (maxLines = 60): string => {
    if (!activeTabId || !terminalRefs.current[activeTabId]) return '';
    const term = terminalRefs.current[activeTabId].term;
    const buffer = term.buffer.active;
    const lines: string[] = [];
    const start = Math.max(0, buffer.length - maxLines);
    for (let i = start; i < buffer.length; i++) {
      const line = buffer.getLine(i);
      if (line) {
        lines.push(line.translateToString(true));
      }
    }
    while (lines.length > 0 && lines[0].trim() === '') lines.shift();
    while (lines.length > 0 && lines[lines.length - 1].trim() === '') lines.pop();
    return lines.join('\n');
  };

  // Inserts command text directly into the shell prompt WITHOUT executing (no \n)
  const handleInsertCommand = (cmd: string) => {
    if (!activeTabId) return;
    WriteTerminalInput(activeTabId, cmd);
    const entry = terminalRefs.current[activeTabId];
    if (entry) {
      entry.term.focus();
    }
  };

  // Automatically refit terminal viewport when AI panel toggles or resizes
  useEffect(() => {
    const t = setTimeout(() => {
      if (activeTabId && terminalRefs.current[activeTabId]) {
        try {
          terminalRefs.current[activeTabId].fitAddon.fit();
          ResizeTerminal({
            session_id: activeTabId,
            cols: terminalRefs.current[activeTabId].term.cols,
            rows: terminalRefs.current[activeTabId].term.rows,
          } as any);
        } catch (e) {}
      }
    }, 60);
    return () => clearTimeout(t);
  }, [isAIPanelOpen, aiPanelWidth, activeTabId]);

  const handlePaste = async () => {
    if (!activeTabId) return;
    const text = await getClipboardText();
    if (text) {
      await WriteTerminalInput(activeTabId, text);
      const entry = terminalRefs.current[activeTabId];
      if (entry) {
        entry.term.focus();
      }
    }
  };

  const handleCopy = async () => {
    if (!activeTabId) return;
    const entry = terminalRefs.current[activeTabId];
    if (entry && entry.term.hasSelection()) {
      const selection = entry.term.getSelection();
      if (selection) {
        await setClipboardText(selection);
      }
    }
  };

  const handleSelectAll = () => {
    if (!activeTabId) return;
    const entry = terminalRefs.current[activeTabId];
    if (entry) {
      entry.term.selectAll();
      entry.term.focus();
    }
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    if (!activeTabId) return;
    const entry = terminalRefs.current[activeTabId];
    const hasSelection = !!(entry && entry.term.hasSelection());
    setContextMenu({
      x: e.clientX,
      y: e.clientY,
      hasSelection,
    });
  };

  useEffect(() => {
    const handleGlobalClick = () => setContextMenu(null);
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setContextMenu(null);
    };
    window.addEventListener('click', handleGlobalClick);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('click', handleGlobalClick);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const handleStartSession = async (serverIdToStart?: string) => {
    const sId = serverIdToStart || selectedServerId;
    const srv = servers.find((s) => s.id === sId);
    if (!srv) return;

    setError(null);
    setIsStarting(true);

    try {
      // 1. Initial dimensions
      const cols = 100;
      const rows = 30;

      const sessionInfo = await StartTerminalSession({
        server_id: srv.id,
        cols,
        rows,
      } as any);

      const newTab: TerminalTab = {
        id: sessionInfo.session_id,
        serverId: srv.id,
        serverName: srv.name,
        serverHost: srv.host,
      };

      setTabs((prev) => [...prev, newTab]);
      setActiveTabId(sessionInfo.session_id);
    } catch (err: unknown) {
      setError(t('terminal.startError', { error: String(err) }));
    } finally {
      setIsStarting(false);
    }
  };

  const handleCloseTab = async (sessionId: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    if (!(await confirm(t('terminal.closeSessionConfirm')))) {
      return;
    }
    try {
      await CloseTerminalSession(sessionId);
    } catch (e) {}

    // Cleanup xterm instance
    const entry = terminalRefs.current[sessionId];
    if (entry) {
      entry.term.dispose();
      delete terminalRefs.current[sessionId];
    }
    delete containerRefs.current[sessionId];

    setTabs((prev) => {
      const remaining = prev.filter((t) => t.id !== sessionId);
      if (activeTabId === sessionId) {
        setActiveTabId(remaining.length > 0 ? remaining[remaining.length - 1].id : null);
      }
      return remaining;
    });
  };

  // Mount/initialize terminal on DOM container when active tab changes
  useEffect(() => {
    if (!activeTabId) return;

    const container = containerRefs.current[activeTabId];
    if (!container) return;

    let term: Terminal;
    let fitAddon: FitAddon;

    if (terminalRefs.current[activeTabId]) {
      term = terminalRefs.current[activeTabId].term;
      fitAddon = terminalRefs.current[activeTabId].fitAddon;
      setTimeout(() => {
        try {
          fitAddon.fit();
          term.focus();
        } catch (e) {}
      }, 50);
    } else {
    // Initialize new Xterm
    const savedFontSize = parseInt(localStorage.getItem('tamestack_term_fontsize') || '13', 10);
    const savedCursorStyle = (localStorage.getItem('tamestack_term_cursor') || 'block') as 'block' | 'underline' | 'bar';
    term = new Terminal({
      cursorBlink: true,
      cursorStyle: savedCursorStyle,
      fontSize: savedFontSize,
      fontFamily: 'JetBrains Mono, Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#0b1120', // deep surface
        foreground: '#e2e8f0',
        cursor: '#10b981', // emerald-500
        selectionBackground: '#1e293b',
        black: '#0f172a',
        red: '#f43f5e',
        green: '#10b981',
        yellow: '#f59e0b',
        blue: '#3b82f6',
        magenta: '#d946ef',
        cyan: '#06b6d4',
        white: '#f8fafc',
      },
    });

    fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    // Handle keyboard shortcuts (Ctrl+V paste, Ctrl+C copy, Shift+Insert)
    term.attachCustomKeyEventHandler((event: KeyboardEvent) => {
      const isCtrlOrCmd = event.ctrlKey || event.metaKey;

      // Paste: Ctrl+V, Cmd+V, Ctrl+Shift+V, Shift+Insert
      const isPaste =
        (isCtrlOrCmd && (event.key === 'v' || event.key === 'V')) ||
        (event.shiftKey && event.key === 'Insert');

      if (isPaste) {
        if (event.type === 'keydown') {
          getClipboardText().then((text) => {
            if (text && activeTabId) {
              WriteTerminalInput(activeTabId, text);
            }
          });
        }
        return false;
      }

      // Copy: Ctrl+Shift+C or Ctrl+C / Cmd+C when text is selected
      const isCopy =
        (isCtrlOrCmd && event.shiftKey && (event.key === 'c' || event.key === 'C')) ||
        (isCtrlOrCmd && (event.key === 'c' || event.key === 'C') && term.hasSelection());

      if (isCopy) {
        if (event.type === 'keydown') {
          const selection = term.getSelection();
          if (selection) {
            setClipboardText(selection);
          }
        }
        return false;
      }

      return true;
    });

    term.open(container);

    terminalRefs.current[activeTabId] = { term, fitAddon };

    // Initial resize
    setTimeout(() => {
      try {
        fitAddon.fit();
        term.focus();
        ResizeTerminal({
          session_id: activeTabId,
          cols: term.cols,
          rows: term.rows,
        } as any);
      } catch (e) {}
    }, 100);
    }

    // Container paste event (native DOM paste)
    const handlePasteEvent = (e: ClipboardEvent) => {
      e.preventDefault();
      const text = e.clipboardData?.getData('text');
      if (text && activeTabId) {
        WriteTerminalInput(activeTabId, text);
      }
    };
    container.addEventListener('paste', handlePasteEvent);

    // Listen to data from server via Wails Events
    const unData = EventsOn('terminal:data:' + activeTabId, (data: string) => {
      term.write(data);
    });

    const unExit = EventsOn('terminal:exit:' + activeTabId, () => {
      term.writeln(`\r\n\x1b[33m${t('terminal.connectionClosedMsg')}\x1b[0m`);
    });

    // Send keystrokes from terminal to Go backend
    const onDataDisposable = term.onData((data) => {
      WriteTerminalInput(activeTabId, data);
    });

    // Handle window resize
    const handleResize = () => {
      try {
        fitAddon.fit();
        ResizeTerminal({
          session_id: activeTabId,
          cols: term.cols,
          rows: term.rows,
        } as any);
      } catch (e) {}
    };

    window.addEventListener('resize', handleResize);

    return () => {
      container.removeEventListener('paste', handlePasteEvent);
      window.removeEventListener('resize', handleResize);
      onDataDisposable.dispose();
      if (typeof unData === 'function') unData();
      else EventsOff('terminal:data:' + activeTabId);

      if (typeof unExit === 'function') unExit();
      else EventsOff('terminal:exit:' + activeTabId);
    };
  }, [activeTabId]);

  useEffect(() => {
    return () => {
      const sessions = terminalRefs.current;
      Object.keys(sessions).forEach((sessionId) => {
        sessions[sessionId].term.dispose();
        CloseTerminalSession(sessionId).catch(() => {});
      });
      terminalRefs.current = {};
    };
  }, []);

  const handleClearTerminal = () => {
    if (activeTabId && terminalRefs.current[activeTabId]) {
      terminalRefs.current[activeTabId].term.clear();
      terminalRefs.current[activeTabId].term.focus();
    }
  };

  const handleQuickCommand = (cmd: string) => {
    if (activeTabId) {
      WriteTerminalInput(activeTabId, cmd + '\n');
      if (terminalRefs.current[activeTabId]) {
        terminalRefs.current[activeTabId].term.focus();
      }
    }
  };

  const { t } = useTranslation();
  const confirm = useConfirm();

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={TerminalIcon}
        description={t('empty.terminalDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  return (
    <div className="flex flex-col h-full w-full min-h-0 p-4 sm:p-6 gap-4 bg-surface-900 text-slate-100 overflow-hidden">
      {/* Top Header Bar */}
      <PageToolbar
        icon={<TerminalIcon className="w-5 h-5 text-emerald-400" />}
        iconClassName="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center justify-center shrink-0"
        servers={servers}
        selectedServerId={selectedServerId}
        onSelectServer={setSelectedServerId}
        serverLabel={t('terminal.targetServer')}
        actions={
          <>
            <Button
              variant={isAIPanelOpen ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setIsAIPanelOpen((prev) => !prev)}
              icon={<Sparkles className={`w-3.5 h-3.5 ${isAIPanelOpen ? 'text-white' : 'text-indigo-400'}`} />}
              title={t('terminal.aiCopilot.openTooltip')}
            >
              {t('terminal.aiCopilot.toggleBtn')}
            </Button>

            <Button
              variant="success"
              size="sm"
              onClick={() => handleStartSession()}
              disabled={isStarting}
              loading={isStarting}
              icon={<Plus className="w-3.5 h-3.5" />}
            >
              {isStarting ? t('terminal.connectingBtn') : t('terminal.newSessionBtn')}
            </Button>
          </>
        }
      />

      {error && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-center justify-between text-xs text-rose-300 shrink-0">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
            <span>{error}</span>
          </div>
          <button onClick={() => setError(null)} className="text-xs hover:underline text-rose-400 font-medium">
            {t('common.close')}
          </button>
        </div>
      )}

      {/* Main Terminal Card */}
      <Card
        variant="default"
        padding="none"
        className="flex-1 flex flex-col min-h-0 bg-[#0b1120] border-slate-800 shadow-sm overflow-hidden"
      >
        {/* Card Header: Session Tabs Strip + Quick Actions */}
        <div className="px-3 py-2 bg-surface-850/95 border-b border-slate-800/90 flex flex-wrap items-center justify-between gap-2 shrink-0">
          {/* Left: Session Tabs */}
          <div className="flex items-center gap-1.5 overflow-x-auto no-scrollbar scroll-smooth">
            {tabs.map((tab) => {
              const isActive = activeTabId === tab.id;
              return (
                <div
                  key={tab.id}
                  onClick={() => setActiveTabId(tab.id)}
                  className={`group flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold cursor-pointer transition-colors duration-150 ease-out whitespace-nowrap shrink-0 outline-none ${
                    isActive
                      ? 'bg-emerald-600 text-white shadow-xs shadow-emerald-600/30'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/70 border border-transparent'
                  }`}
                >
                  <TerminalIcon className={`w-3.5 h-3.5 ${isActive ? 'text-white' : 'text-slate-500 group-hover:text-emerald-400'}`} />
                  <span className="font-mono">{tab.serverName}</span>
                  <button
                    type="button"
                    onClick={(e) => handleCloseTab(tab.id, e)}
                    className={`p-0.5 rounded transition ${
                      isActive
                        ? 'text-white/70 hover:text-white hover:bg-white/20'
                        : 'text-slate-500 hover:text-rose-400 hover:bg-slate-700/60'
                    }`}
                    title={t('terminal.closeSessionTooltip')}
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
              );
            })}

            {tabs.length > 0 && (
              <button
                type="button"
                onClick={() => handleStartSession()}
                className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
                title={t('terminal.newSessionBtn')}
              >
                <Plus className="w-3.5 h-3.5" />
              </button>
            )}

            {tabs.length === 0 && (
              <span className="text-xs text-slate-500 py-1 italic pl-1">
                {t('terminal.noActiveSessions')}
              </span>
            )}
          </div>

          {/* Right: Quick Commands & Actions (when sessions active) */}
          {tabs.length > 0 && activeTabId && (
            <div className="flex items-center gap-2 ml-auto">
              <div className="hidden lg:flex items-center gap-1.5">
                <span className="text-[11px] text-slate-500 font-medium mr-0.5">{t('terminal.quickCommands')}</span>
                <button
                  type="button"
                  onClick={() => handleQuickCommand('htop')}
                  className="px-2.5 py-1 bg-slate-900/90 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 text-slate-300 hover:text-white rounded-lg text-xs font-mono transition shadow-xs"
                >
                  htop
                </button>
                <button
                  type="button"
                  onClick={() => handleQuickCommand('free -m && df -h')}
                  className="px-2.5 py-1 bg-slate-900/90 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 text-slate-300 hover:text-white rounded-lg text-xs font-mono transition shadow-xs"
                >
                  free & df
                </button>
                <button
                  type="button"
                  onClick={() => handleQuickCommand('docker ps')}
                  className="px-2.5 py-1 bg-slate-900/90 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 text-slate-300 hover:text-white rounded-lg text-xs font-mono transition shadow-xs"
                >
                  docker ps
                </button>
                <button
                  type="button"
                  onClick={() => handleQuickCommand('systemctl status nginx')}
                  className="px-2.5 py-1 bg-slate-900/90 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 text-slate-300 hover:text-white rounded-lg text-xs font-mono transition shadow-xs"
                >
                  nginx status
                </button>
              </div>

              <div className="flex items-center gap-1.5">
                <Button
                  variant="secondary"
                  size="xs"
                  onClick={handlePaste}
                  icon={<ClipboardPaste className="w-3.5 h-3.5 text-emerald-400" />}
                  title={t('terminal.pasteTooltip')}
                >
                  {t('terminal.pasteBtn')}
                </Button>

                <Button
                  variant="secondary"
                  size="xs"
                  onClick={handleClearTerminal}
                  icon={<Trash2 className="w-3 h-3 text-slate-400" />}
                  title={t('terminal.clearScreenTooltip')}
                >
                  {t('terminal.clearBtn')}
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* Card Body: Terminal Canvas + AI Panel */}
        <div className="flex-1 flex min-h-0 relative overflow-hidden">
          <div className="flex-1 flex flex-col min-h-0 relative overflow-hidden">
            {tabs.length > 0 && activeTabId ? (
              <div
                className="flex-1 px-2.5 py-1.5 overflow-hidden select-text w-full h-full"
                onContextMenu={handleContextMenu}
              >
                {tabs.map((tab) => (
                  <div
                    key={tab.id}
                    ref={(el) => (containerRefs.current[tab.id] = el)}
                    className={`w-full h-full ${activeTabId === tab.id ? 'block' : 'hidden'}`}
                  />
                ))}
              </div>
            ) : (
              <div className="flex-1 flex items-center justify-center p-8">
                <EmptyState
                  icon={<TerminalIcon className="w-6 h-6 text-emerald-400" />}
                  title={t('terminal.readyTitle')}
                  description={t('terminal.readyDesc')}
                  action={{
                    label: t('terminal.openSessionFor', {
                      server: servers.find((s) => s.id === selectedServerId)?.name || servers[0]?.name || '',
                    }),
                    onClick: () => handleStartSession(),
                    icon: <Play className="w-3.5 h-3.5" />,
                    variant: 'primary',
                  }}
                  className="m-auto border-0 bg-transparent"
                />
              </div>
            )}
          </div>

          {/* AI Side Panel */}
          {isAIPanelOpen && (
            <TerminalAIPanel
              isOpen={isAIPanelOpen}
              onClose={() => setIsAIPanelOpen(false)}
              activeTab={tabs.find((t) => t.id === activeTabId) || null}
              getTerminalBuffer={getTerminalBufferText}
              onInsertCommand={handleInsertCommand}
              width={aiPanelWidth}
              onResize={setAiPanelWidth}
            />
          )}
        </div>

        {/* Right-click Context Menu */}
        {contextMenu && (
          <div
            style={{
              top: Math.min(contextMenu.y, window.innerHeight - 170),
              left: Math.min(contextMenu.x, window.innerWidth - 180),
            }}
            className="fixed z-50 min-w-[170px] bg-slate-900/95 border border-slate-800 rounded-xl shadow-2xl py-1.5 text-xs text-slate-200 backdrop-blur-md"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              onClick={async () => {
                setContextMenu(null);
                await handleCopy();
              }}
              disabled={!contextMenu.hasSelection}
              className="w-full px-3 py-1.5 flex items-center justify-between gap-3 hover:bg-slate-800/80 disabled:opacity-40 disabled:hover:bg-transparent transition text-left cursor-pointer"
            >
              <div className="flex items-center gap-2">
                <Copy className="w-3.5 h-3.5 text-slate-400" />
                <span>{t('terminal.copyBtn')}</span>
              </div>
              <span className="text-[10px] text-slate-500 font-mono px-1 py-0.5 rounded bg-slate-800">Ctrl+C</span>
            </button>

            <button
              onClick={async () => {
                setContextMenu(null);
                await handlePaste();
              }}
              className="w-full px-3 py-1.5 flex items-center justify-between gap-3 hover:bg-slate-800/80 transition text-left cursor-pointer"
            >
              <div className="flex items-center gap-2">
                <ClipboardPaste className="w-3.5 h-3.5 text-emerald-400" />
                <span>{t('terminal.pasteBtn')}</span>
              </div>
              <span className="text-[10px] text-slate-500 font-mono px-1 py-0.5 rounded bg-slate-800">Ctrl+V</span>
            </button>

            <div className="my-1 border-t border-slate-800" />

            <button
              onClick={() => {
                setContextMenu(null);
                handleSelectAll();
              }}
              className="w-full px-3 py-1.5 flex items-center justify-between gap-3 hover:bg-slate-800/80 transition text-left cursor-pointer"
            >
              <div className="flex items-center gap-2">
                <CheckSquare className="w-3.5 h-3.5 text-slate-400" />
                <span>{t('terminal.selectAll')}</span>
              </div>
              <span className="text-[10px] text-slate-500 font-mono px-1 py-0.5 rounded bg-slate-800">Ctrl+A</span>
            </button>

            <button
              onClick={() => {
                setContextMenu(null);
                handleClearTerminal();
              }}
              className="w-full px-3 py-1.5 flex items-center justify-between gap-3 hover:bg-rose-500/10 transition text-left text-rose-300 hover:text-rose-200 cursor-pointer"
            >
              <div className="flex items-center gap-2">
                <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                <span>{t('terminal.clearBtn')}</span>
              </div>
              <span className="text-[10px] text-rose-400/70 font-mono px-1 py-0.5 rounded bg-rose-500/10">Ctrl+L</span>
            </button>
          </div>
        )}
      </Card>
    </div>
  );
};
