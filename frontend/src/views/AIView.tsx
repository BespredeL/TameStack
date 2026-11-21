import React, { useState, useEffect, useRef, useMemo } from 'react';
import {
  Sparkles,
  Plus,
  Trash2,
  Send,
  Square,
  Server,
  Cpu,
  Settings,
  Shield,
  Layers,
  Wrench,
  Bot,
  Activity,
  PanelLeftClose,
  PanelLeftOpen,
} from 'lucide-react';
import {
  ServerRecord,
  AIConversationRecord,
  AIMessageRecord,
  AIProviderConfigDTO,
  AIToolDefinition,
  AIStreamChunkPayload,
  AIStreamCompletePayload,
  AIStreamErrorPayload,
} from '../types';
import {
  AIListConversations,
  AICreateConversation,
  AIDeleteConversation,
  AIListMessages,
  AIStreamMessage,
  AIListProviderConfigs,
  AIListTools,
  AIListModels,
  AIGetGlobalSettings,
  AIGetDisabledTools,
  AISetDisabledTools,
} from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { MessageItem } from '../components/ai/MessageItem';
import { AISettingsModal } from '../components/ai/AISettingsModal';
import { AgentsManagerModal } from '../components/ai/AgentsManagerModal';
import { ToolsManagerModal } from '../components/ai/ToolsManagerModal';
import { Button } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useAlert } from '../context/AlertContext';
import { useConfirm } from '../context/ConfirmContext';
import { getErrorMessage } from '../utils/errors';
import { useServer } from '../context/ServerContext';

interface AIViewProps {
  servers: ServerRecord[];
  initialContext?: { serverId?: string; prompt?: string } | null;
  onClearInitialContext?: () => void;
}

export const AIView: React.FC<AIViewProps> = ({
  servers,
  initialContext,
  onClearInitialContext,
}) => {
  const { t } = useTranslation();
  const alert = useAlert();
  const confirm = useConfirm();
  const { selectedServerId: globalServerId } = useServer();

  const [conversations, setConversations] = useState<AIConversationRecord[]>([]);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [messages, setMessages] = useState<AIMessageRecord[]>([]);
  const [inputPrompt, setInputPrompt] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isAgentsOpen, setIsAgentsOpen] = useState(false);
  const [isToolsModalOpen, setIsToolsModalOpen] = useState(false);
  const [disabledTools, setDisabledTools] = useState<string[]>([]);

  // Responsive & resizable sidebar state
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(() => {
    const saved = localStorage.getItem('tamestack_ai_sidebar_open');
    return saved !== null ? saved === 'true' : true;
  });

  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    const saved = localStorage.getItem('tamestack_ai_sidebar_width');
    const parsed = saved ? parseInt(saved, 10) : 280;
    return isNaN(parsed) ? 280 : Math.max(200, Math.min(480, parsed));
  });

  const [isResizing, setIsResizing] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const toggleSidebar = () => {
    setIsSidebarOpen((prev) => {
      const next = !prev;
      localStorage.setItem('tamestack_ai_sidebar_open', String(next));
      return next;
    });
  };

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect();
        const offset = e.clientX - rect.left;
        const maxW = Math.min(500, Math.floor(rect.width * 0.45));
        const clamped = Math.max(200, Math.min(maxW, offset));
        setSidebarWidth(clamped);
      }
    };

    const handleMouseUp = () => {
      if (isResizing) {
        setIsResizing(false);
        setSidebarWidth((w) => {
          localStorage.setItem('tamestack_ai_sidebar_width', String(w));
          return w;
        });
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
  }, [isResizing]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => {
      if (el.clientWidth < 760) setIsSidebarOpen(false);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  // Available Tools and Providers metadata
  const [tools, setTools] = useState<AIToolDefinition[]>([]);
  const [providers, setProviders] = useState<AIProviderConfigDTO[]>([]);

  const toolNameMap = useMemo(() => {
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
        } catch {}
      }
    });
    return map;
  }, [messages]);

  const visibleMessages = useMemo(() => {
    return messages.filter((m) => {
      // Hide empty assistant messages that only invoked tools without text content
      if (m.role === 'assistant' && (!m.content || !m.content.trim())) {
        return false;
      }
      return true;
    });
  }, [messages]);

  // Selected server and model for the active conversation
  const [selectedServerId, setSelectedServerId] = useState<string>(globalServerId || '');
  const [selectedProvider, setSelectedProvider] = useState<string>('ollama');
  const [selectedModel, setSelectedModel] = useState<string>('qwen2.5-coder:7b');
  const [availableModels, setAvailableModels] = useState<string[]>([]);

  useEffect(() => {
    if (!activeConvId && globalServerId && !selectedServerId) {
      setSelectedServerId(globalServerId);
    }
  }, [globalServerId, activeConvId]);

  const fetchModelsForProvider = async (provider: string) => {
    try {
      const specs = await AIListModels(provider);
      if (specs && specs.length > 0) {
        const ids = specs.map((s: any) => s.id);
        setAvailableModels(ids);
      }
    } catch {
      // Keep existing models if offline or ping failed
    }
  };

  useEffect(() => {
    fetchModelsForProvider(selectedProvider);
  }, [selectedProvider]);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const streamConvRef = useRef<string | null>(null);
  const conversationsRef = useRef<AIConversationRecord[]>([]);
  const isSendingRef = useRef<boolean>(false);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      const scrollH = textareaRef.current.scrollHeight;
      textareaRef.current.style.height = `${Math.min(Math.max(scrollH, 48), 180)}px`;
    }
  }, [inputPrompt]);

  useEffect(() => {
    if (initialContext) {
      if (initialContext.serverId) {
        setSelectedServerId(initialContext.serverId);
      }
      if (initialContext.prompt) {
        setInputPrompt(initialContext.prompt);
        setTimeout(() => textareaRef.current?.focus(), 100);
      }
      onClearInitialContext?.();
    }
  }, [initialContext]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, streamingContent]);

  // Load initial data
  const loadConversations = async () => {
    try {
      const convList = (await AIListConversations()) as AIConversationRecord[];
      const list = convList || [];
      conversationsRef.current = list;
      setConversations(list);
      if (list.length > 0 && !activeConvId) {
        selectConversation(list[0].id, list);
      }
    } catch (e) {
      console.error('Failed to load conversations:', e);
    }
  };

  const loadMetadata = async () => {
    try {
      const [tList, pList, globalSettings, disabledList] = await Promise.all([
        AIListTools(),
        AIListProviderConfigs(),
        AIGetGlobalSettings() as Promise<Record<string, string>>,
        AIGetDisabledTools(),
      ]);
      setTools((tList as AIToolDefinition[]) || []);
      setDisabledTools((disabledList as string[]) || []);
      const provList = (pList as AIProviderConfigDTO[]) || [];
      setProviders(provList);

      // Determine default provider and model from settings or active configs
      const defConfig = provList.find((p) => p.is_default) || provList[0];
      const targetProv = globalSettings?.['default_provider'] || defConfig?.provider_type || 'ollama';
      const targetModel =
        globalSettings?.['default_model'] ||
        defConfig?.default_model ||
        (targetProv === 'openai' ? 'gpt-4o-mini' : targetProv === 'openai_compatible' ? 'unsloth/Qwen2.5-Coder-7B-Instruct' : 'qwen2.5-coder:7b');

      if (!activeConvId) {
        setSelectedProvider(targetProv);
        setSelectedModel(targetModel);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleSaveDisabledTools = async (newDisabled: string[]) => {
    try {
      await AISetDisabledTools(newDisabled);
      setDisabledTools(newDisabled);
    } catch (e) {
      alert(getErrorMessage(e));
      throw e;
    }
  };

  const activeToolsCount = Math.max(0, tools.length - disabledTools.length);

  useEffect(() => {
    loadConversations();
    loadMetadata();
  }, []);

  // Setup streaming event listeners
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
        const id = data?.conversation_id || streamConvRef.current;
        if (id) {
          loadMessages(id);
        }
      }
    });

    const unError = EventsOn('ai:stream:error', (data: AIStreamErrorPayload) => {
      setIsStreaming(false);
      if (!data || data.conversation_id === streamConvRef.current) {
        alert(t('ai.errorPrefix', { error: data?.error || 'unknown' }));
      }
    });

    return () => {
      // Unsubscribe listeners on unmount
      if (typeof unChunk === 'function') unChunk();
      if (typeof unComplete === 'function') unComplete();
      if (typeof unError === 'function') unError();
      EventsOff('ai:stream:chunk', 'ai:stream:complete', 'ai:stream:error');
    };
  }, []);

  const loadMessages = async (convID: string) => {
    try {
      const msgs = (await AIListMessages(convID)) as AIMessageRecord[];
      setMessages(msgs || []);
    } catch (e) {
      console.error('Failed to load messages:', e);
    }
  };

  const selectConversation = async (convID: string, list?: AIConversationRecord[]) => {
    setActiveConvId(convID);
    setStreamingContent('');
    setIsStreaming(false);
    await loadMessages(convID);

    const conv = (list || conversationsRef.current).find((c) => c.id === convID);
    if (conv) {
      setSelectedServerId(conv.server_id || '');
      setSelectedProvider(conv.selected_provider || 'ollama');
      setSelectedModel(conv.selected_model || 'qwen2.5-coder:7b');
    }
  };

  const handleCreateNewConversation = async () => {
    try {
      const title = t('ai.newChatDefaultTitle', { time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) });
      const srvID = selectedServerId || (servers.length > 0 ? servers[0].id : null);
      const newConv = (await AICreateConversation(
        title,
        srvID,
        '',
        selectedProvider,
        selectedModel
      )) as AIConversationRecord;

      if (newConv) {
        setConversations((prev) => {
          const next = [newConv, ...prev];
          conversationsRef.current = next;
          return next;
        });
        selectConversation(newConv.id, [newConv, ...conversationsRef.current]);
      }
    } catch (e) {
      console.error('Failed to create conversation:', e);
    }
  };

  const handleDeleteConversation = async (e: React.MouseEvent, convID: string) => {
    e.stopPropagation();
    if (!(await confirm(t('ai.deleteChatConfirm'), { danger: true, confirmLabel: t('common.delete') }))) return;
    try {
      await AIDeleteConversation(convID);
      const filtered = conversations.filter((c) => c.id !== convID);
      setConversations(filtered);
      if (activeConvId === convID) {
        if (filtered.length > 0) {
          selectConversation(filtered[0].id);
        } else {
          setActiveConvId(null);
          setMessages([]);
        }
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleSendMessage = async () => {
    const prompt = inputPrompt.trim();
    if (!prompt || isSendingRef.current || isStreaming) return;

    isSendingRef.current = true;
    setInputPrompt('');
    setIsStreaming(true);
    setStreamingContent('');

    let targetConvID = activeConvId;
    try {
      if (!targetConvID) {
        const title = prompt.slice(0, 30) + (prompt.length > 30 ? '...' : '');
        const srvID = selectedServerId || (servers.length > 0 ? servers[0].id : null);
        const newConv = (await AICreateConversation(
          title,
          srvID,
          '',
          selectedProvider,
          selectedModel
        )) as AIConversationRecord;
        setConversations((prev) => {
          const next = [newConv, ...prev];
          conversationsRef.current = next;
          return next;
        });
        targetConvID = newConv.id;
        setActiveConvId(newConv.id);
      }
      streamConvRef.current = targetConvID;

      // Optimistic user message in UI
      const optimisticUserMsg: AIMessageRecord = {
        id: `temp_${Date.now()}`,
        conversation_id: targetConvID,
        role: 'user',
        content: prompt,
        prompt_tokens: 0,
        completion_tokens: 0,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, optimisticUserMsg]);

      await AIStreamMessage(targetConvID, prompt);
    } catch (err: unknown) {
      setIsStreaming(false);
      alert(t('ai.sendError', { error: getErrorMessage(err) }));
    } finally {
      isSendingRef.current = false;
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      if (e.nativeEvent.isComposing || e.repeat) {
        return;
      }
      e.preventDefault();
      handleSendMessage();
    }
  };

  const activeConv = conversations.find((c) => c.id === activeConvId);

  return (
    <div
      ref={containerRef}
      className="h-full w-full flex-1 flex min-h-0 overflow-hidden bg-slate-950 text-slate-100 relative"
    >
      {/* Sidebar: Conversations List */}
      <div
        style={{ width: isSidebarOpen ? `${sidebarWidth}px` : 0 }}
        className={`bg-slate-900 border-r border-slate-800 flex flex-col shrink-0 overflow-hidden relative select-none ${
          isResizing ? '' : 'transition-[width] duration-150 ease-out'
        }`}
      >
        <div style={{ width: `${sidebarWidth}px` }} className="flex flex-col h-full overflow-hidden shrink-0">
          {/* Header Actions */}
          <div className="p-3 border-b border-slate-800 flex items-center gap-2 shrink-0">
            <Button
              variant="primary"
              size="sm"
              onClick={handleCreateNewConversation}
              icon={<Plus className="w-3.5 h-3.5 shrink-0" />}
              className="flex-1 justify-center truncate"
            >
              <span className="truncate">{t('ai.newChat')}</span>
            </Button>
            <Button
              variant="secondary"
              size="icon-sm"
              onClick={() => setIsAgentsOpen(true)}
              icon={<Bot className="w-3.5 h-3.5 text-indigo-400" />}
              title={t('ai.agentsTooltip')}
            />
            <Button
              variant="secondary"
              size="icon-sm"
              onClick={() => setIsSettingsOpen(true)}
              icon={<Settings className="w-3.5 h-3.5" />}
              title={t('ai.settingsTooltip')}
            />
          </div>

          {/* Conversation Items */}
          <div className="flex-1 min-h-0 overflow-y-auto p-2 space-y-1">
            {conversations.length === 0 ? (
              <div className="text-center py-10 px-4 text-xs text-slate-500">
                {t('ai.emptyChatsDesc')}
              </div>
            ) : (
              conversations.map((conv) => {
                const active = conv.id === activeConvId;
                const boundServer = servers.find((s) => s.id === conv.server_id);
                return (
                  <div
                    key={conv.id}
                    onClick={() => selectConversation(conv.id)}
                    className={`group relative flex items-start gap-2.5 p-2.5 rounded-lg text-xs cursor-pointer transition-all ${
                      active
                        ? 'bg-slate-800 text-white shadow-sm border border-slate-700'
                        : 'text-slate-400 hover:bg-slate-850 hover:text-slate-200'
                    }`}
                  >
                    <Bot className={`w-4 h-4 mt-0.5 shrink-0 ${active ? 'text-indigo-400' : 'text-slate-500'}`} />
                    <div className="flex-1 min-w-0 pr-6">
                      <div className="font-medium truncate">{conv.title}</div>
                      <div className="text-[10px] text-slate-500 flex items-center gap-1.5 mt-0.5">
                        {boundServer ? (
                          <span className="truncate flex items-center gap-1 text-slate-400">
                            <Server className="w-2.5 h-2.5 text-indigo-400 shrink-0" />
                            <span className="truncate">{boundServer.name}</span>
                          </span>
                        ) : (
                          <span>{t('ai.globalContext')}</span>
                        )}
                      </div>
                    </div>
                    <button
                      onClick={(e) => handleDeleteConversation(e, conv.id)}
                      className="opacity-0 group-hover:opacity-100 absolute right-2 top-2.5 p-1 rounded hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition-opacity"
                      title={t('ai.deleteChatTooltip')}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                );
              })
            )}
          </div>

          {/* Footer info */}
          <div className="p-2.5 border-t border-slate-800/80 text-[11px] text-slate-400 bg-slate-950/40 flex items-center justify-between gap-2 shrink-0">
            <button
              type="button"
              onClick={() => setIsToolsModalOpen(true)}
              className="flex items-center gap-1.5 px-2 py-1 -ml-1 rounded-lg text-slate-300 hover:text-white hover:bg-slate-800/70 border border-transparent hover:border-slate-700/60 transition truncate group"
              title={t('ai.toolsManager.manageTooltip') || 'Настроить инструменты ИИ'}
            >
              <Wrench className={`w-3.5 h-3.5 shrink-0 group-hover:rotate-12 transition-transform ${activeToolsCount === 0 ? 'text-amber-400' : 'text-emerald-400'}`} />
              <span className="truncate font-medium">
                {activeToolsCount === 0
                  ? (t('ai.toolsManager.allDisabled') || 'Инструменты отключены')
                  : t('ai.toolsCount', { count: activeToolsCount })}
              </span>
              {disabledTools.length > 0 && (
                <span className="px-1.5 py-0.5 text-[10px] rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30 shrink-0 font-mono">
                  {t('ai.toolsManager.disabledBadge', { count: disabledTools.length })}
                </span>
              )}
            </button>
            <span className="flex items-center gap-1 text-slate-500 shrink-0" title="Human-in-the-Loop Security Gate">
              <Shield className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
              <span>HITL</span>
            </span>
          </div>
        </div>

        {/* Sidebar Resizer Handle */}
        {isSidebarOpen && (
          <div
            onMouseDown={(e) => {
              e.preventDefault();
              setIsResizing(true);
            }}
            className={`absolute top-0 right-0 bottom-0 w-1.5 cursor-col-resize select-none z-20 hover:bg-indigo-500/50 active:bg-indigo-600 transition-colors ${
              isResizing ? 'bg-indigo-500' : 'bg-transparent'
            }`}
            title={t('ai.resizeTooltip')}
          />
        )}
      </div>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0 h-full overflow-hidden bg-slate-950">
        {/* Top Control Bar */}
        <div className="min-h-[52px] px-3 sm:px-6 py-2 border-b border-slate-800 bg-slate-900/50 flex flex-wrap items-center justify-between gap-3 shrink-0">
          <div className="flex items-center gap-2.5 min-w-0">
            <button
              onClick={toggleSidebar}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0"
              title={isSidebarOpen ? t('ai.toggleSidebarTooltipHide') : t('ai.toggleSidebarTooltipShow')}
              aria-label={t('ai.toggleSidebarAria')}
            >
              {isSidebarOpen ? <PanelLeftClose className="w-4 h-4" /> : <PanelLeftOpen className="w-4 h-4" />}
            </button>
            <div className="w-7 h-7 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
              <Sparkles className="w-4 h-4" />
            </div>
            <div className="min-w-0">
              <h2 className="text-xs sm:text-sm font-bold text-white truncate max-w-xs sm:max-w-sm md:max-w-md">
                {activeConv ? activeConv.title : t('ai.defaultAssistantTitle')}
              </h2>
            </div>
          </div>

          {/* Context Selectors */}
          <div className="flex items-center gap-2 flex-wrap justify-end min-w-0">
            {/* Server Selector */}
            <div className="flex items-center gap-1.5 bg-slate-850 px-2.5 py-1 rounded-lg border border-slate-800 text-xs">
              <Server className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
              <select
                value={selectedServerId}
                disabled={Boolean(activeConvId)}
                title={activeConvId ? t('ai.serverFixedTooltip') : t('ai.serverNewTooltip')}
                onChange={(e) => setSelectedServerId(e.target.value)}
                className="bg-transparent text-slate-200 text-xs focus:outline-none cursor-pointer max-w-[9rem] sm:max-w-[11rem] truncate"
              >
                <option value="" className="bg-slate-900 text-slate-400">
                  {t('ai.allServers')}
                </option>
                {servers.map((s) => (
                  <option key={s.id} value={s.id} className="bg-slate-900 text-white">
                    {s.name} ({s.host})
                  </option>
                ))}
              </select>
            </div>

            {/* Provider Selector */}
            <div className="flex items-center gap-1.5 bg-slate-850 px-2.5 py-1 rounded-lg border border-slate-800 text-xs">
              <Cpu className="w-3.5 h-3.5 text-purple-400 shrink-0" />
              <select
                value={selectedProvider}
                disabled={Boolean(activeConvId)}
                title={activeConvId ? t('ai.providerFixedTooltip') : t('ai.providerNewTooltip')}
                onChange={(e) => {
                  const p = e.target.value;
                  setSelectedProvider(p);
                  const cfg = providers.find((c) => c.provider_type === p);
                  if (cfg && cfg.default_model) {
                    setSelectedModel(cfg.default_model);
                  } else if (p === 'openai') {
                    setSelectedModel('gpt-4o-mini');
                  } else if (p === 'openai_compatible') {
                    setSelectedModel('unsloth/Qwen2.5-Coder-7B-Instruct');
                  } else if (p === 'gemini') {
                    setSelectedModel('gemini-1.5-flash');
                  } else if (p === 'anthropic') {
                    setSelectedModel('claude-3-5-sonnet-20241022');
                  } else {
                    setSelectedModel('qwen2.5-coder:7b');
                  }
                }}
                className="bg-transparent text-slate-200 text-xs focus:outline-none cursor-pointer max-w-[9rem] sm:max-w-[11rem] truncate"
              >
                <option value="ollama" className="bg-slate-900 text-white">{t('ai.providerOllama')}</option>
                <option value="openai_compatible" className="bg-slate-900 text-white">{t('ai.providerOpenAIComp')}</option>
                <option value="llamacpp" className="bg-slate-900 text-white">llama.cpp</option>
                <option value="openai" className="bg-slate-900 text-white">OpenAI API</option>
                <option value="gemini" className="bg-slate-900 text-white">Google Gemini</option>
                <option value="anthropic" className="bg-slate-900 text-white">Anthropic Claude</option>
              </select>
            </div>

            {/* Model Name Input / Selector */}
            <div className="flex items-center gap-1.5 bg-slate-850 px-2.5 py-1 rounded-lg border border-slate-800 text-xs">
              <span className="text-[10px] text-slate-400 uppercase font-bold shrink-0">{t('ai.modelLabel')}</span>
              <input
                list="ai-models-list"
                type="text"
                value={selectedModel}
                disabled={Boolean(activeConvId)}
                title={activeConvId ? t('ai.modelFixedTooltip') : t('ai.modelNewTooltip')}
                onChange={(e) => setSelectedModel(e.target.value)}
                placeholder={t('ai.modelPlaceholder')}
                className="bg-transparent text-slate-200 text-xs focus:outline-none w-32 sm:w-36 font-mono"
              />
              <datalist id="ai-models-list">
                {availableModels.map((m) => (
                  <option key={m} value={m} />
                ))}
              </datalist>
            </div>
          </div>
        </div>

        {/* Message Stream Scroll Area */}
        <div className="flex-1 min-h-0 overflow-y-auto select-text">
          {messages.length === 0 && !streamingContent ? (
            /* Empty State with Quick Prompts */
            <div className="h-full flex flex-col items-center justify-center p-6 text-center max-w-3xl xl:max-w-4xl 2xl:max-w-5xl mx-auto w-full my-auto">
              <div className="w-14 h-14 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 mb-4 shadow-xl shadow-indigo-500/10">
                <Sparkles className="w-7 h-7" />
              </div>
              <h3 className="text-base sm:text-lg font-bold text-white mb-2">{t('ai.welcomeTitle')}</h3>
              <p className="text-xs sm:text-sm text-slate-400 mb-6 max-w-xl leading-relaxed">
                {t('ai.welcomeDesc')}
              </p>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full text-left">
                {[
                  {
                    icon: Activity,
                    title: t('ai.suggestionNginxTitle'),
                    prompt: t('ai.suggestionNginxPrompt'),
                  },
                  {
                    icon: Shield,
                    title: t('ai.suggestionPortsTitle'),
                    prompt: t('ai.suggestionPortsPrompt'),
                  },
                  {
                    icon: Cpu,
                    title: t('ai.suggestionResourcesTitle'),
                    prompt: t('ai.suggestionResourcesPrompt'),
                  },
                  {
                    icon: Layers,
                    title: t('ai.suggestionDbTitle'),
                    prompt: t('ai.suggestionDbPrompt'),
                  },
                ].map((item, i) => {
                  const Icon = item.icon;
                  return (
                    <button
                      key={i}
                      onClick={() => setInputPrompt(item.prompt)}
                      className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800 hover:border-indigo-500/50 hover:bg-slate-850 transition-all text-xs group"
                    >
                      <div className="flex items-center gap-2 font-semibold text-slate-200 group-hover:text-white mb-1">
                        <Icon className="w-4 h-4 text-indigo-400 shrink-0" />
                        <span>{item.title}</span>
                      </div>
                      <div className="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">{item.prompt}</div>
                    </button>
                  );
                })}
              </div>
            </div>
          ) : (
            <div className="divide-y divide-slate-850/60 pb-6">
              {visibleMessages.map((msg) => (
                <MessageItem
                  key={msg.id}
                  message={msg}
                  toolName={msg.tool_call_id ? toolNameMap.get(msg.tool_call_id) : undefined}
                  onApprovalComplete={() => activeConvId && loadMessages(activeConvId)}
                />
              ))}

              {/* Streaming assistant message in progress */}
              {isStreaming && (
                <MessageItem
                  message={{
                    id: 'streaming_tmp',
                    conversation_id: activeConvId || '',
                    role: 'assistant',
                    content: streamingContent || t('ai.analyzingInfrastructure'),
                    prompt_tokens: 0,
                    completion_tokens: 0,
                    created_at: new Date().toISOString(),
                  }}
                  isStreaming={true}
                />
              )}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* Bottom Input Area */}
        <div className="p-3 sm:p-4 border-t border-slate-800 bg-slate-900/40 shrink-0">
          <div className="max-w-4xl xl:max-w-5xl 2xl:max-w-6xl mx-auto w-full">
            <div className="relative rounded-xl border border-slate-700/80 bg-slate-900 focus-within:border-indigo-500 transition-colors shadow-lg">
              <textarea
                ref={textareaRef}
                value={inputPrompt}
                onChange={(e) => setInputPrompt(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={t('ai.inputPlaceholder')}
                rows={2}
                className="w-full px-4 py-3 bg-transparent text-xs text-white placeholder-slate-500 focus:outline-none resize-none leading-relaxed font-sans"
              />

              <div className="flex items-center justify-between px-3 py-2 border-t border-slate-800/60 bg-slate-950/40 rounded-b-xl text-[11px] text-slate-500">
                <span className="flex items-center gap-1.5 truncate mr-2">
                  <span className="truncate">{t('ai.inputHint')}</span>
                </span>

                <div className="flex items-center gap-2 shrink-0">
                  {isStreaming ? (
                    <button
                      type="button"
                      onClick={() => setIsStreaming(false)}
                      className="flex items-center gap-1.5 px-3 py-1 bg-rose-600 hover:bg-rose-500 text-white rounded-lg text-xs font-semibold transition-colors shrink-0"
                    >
                      <Square className="w-3 h-3 fill-current" />
                      <span>{t('ai.stop')}</span>
                    </button>
                  ) : (
                    <button
                      type="button"
                      onClick={handleSendMessage}
                      disabled={!inputPrompt.trim() || isStreaming}
                      className="flex items-center gap-1.5 px-3 py-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:hover:bg-indigo-600 text-white rounded-lg text-xs font-semibold shadow-md shadow-indigo-600/20 transition-all shrink-0"
                    >
                      <Send className="w-3 h-3" />
                      <span>{t('ai.send')}</span>
                    </button>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Settings Modal */}
      <AISettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        onSaved={() => {
          loadMetadata();
        }}
      />

      {/* Autonomous Agents Modal */}
      <AgentsManagerModal
        isOpen={isAgentsOpen}
        onClose={() => setIsAgentsOpen(false)}
      />

      {/* AI Tools Manager Modal */}
      <ToolsManagerModal
        isOpen={isToolsModalOpen}
        onClose={() => setIsToolsModalOpen(false)}
        tools={tools}
        disabledTools={disabledTools}
        onSave={handleSaveDisabledTools}
      />
    </div>
  );
};

