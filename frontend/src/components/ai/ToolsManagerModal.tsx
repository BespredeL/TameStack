import React, { useState, useMemo, useEffect } from 'react';
import {
  Wrench,
  Shield,
  ShieldAlert,
  Server,
  Globe,
  Database,
  Sparkles,
  Layers,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { Button, Badge, SearchInput, Tabs, EmptyState } from '../ui';
import { AIToolDefinition } from '../../types';
import { useTranslation } from '../../i18n/LanguageContext';
import {
  ToolCategory,
  TOOL_CATEGORIES,
  getToolCategory,
  isToolMutating,
  getReadOnlyToolNames,
} from '../../utils/aiToolCategories';

interface ToolsManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  tools: AIToolDefinition[];
  disabledTools: string[];
  onSave: (disabledTools: string[]) => Promise<void> | void;
}

export const ToolsManagerModal: React.FC<ToolsManagerModalProps> = ({
  isOpen,
  onClose,
  tools,
  disabledTools: initialDisabled,
  onSave,
}) => {
  const { t } = useTranslation();
  const [disabledSet, setDisabledSet] = useState<Set<string>>(new Set(initialDisabled));
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState<ToolCategory>('all');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setDisabledSet(new Set(initialDisabled));
      setSearch('');
      setActiveCategory('all');
    }
  }, [isOpen, initialDisabled]);

  const toggleTool = (toolName: string) => {
    setDisabledSet((prev) => {
      const next = new Set(prev);
      if (next.has(toolName)) {
        next.delete(toolName);
      } else {
        next.add(toolName);
      }
      return next;
    });
  };

  // Preset handlers
  const handleEnableAll = () => {
    setDisabledSet(new Set());
  };

  const handleDisableAll = () => {
    setDisabledSet(new Set(tools.map((tool) => tool.name)));
  };

  const handleReadOnlyOnly = () => {
    const readOnlyNames = new Set(getReadOnlyToolNames(tools));
    // Disable any tool that is NOT read-only
    const newDisabled = new Set<string>();
    for (const tool of tools) {
      if (!readOnlyNames.has(tool.name)) {
        newDisabled.add(tool.name);
      }
    }
    setDisabledSet(newDisabled);
  };

  const getToolDescription = (tool: AIToolDefinition) => {
    const key = `ai.toolDescriptions.${tool.name}`;
    const localized = t(key);
    return localized !== key ? localized : tool.description || '';
  };

  const filteredTools = useMemo(() => {
    return tools.filter((tool) => {
      // Category match
      if (activeCategory !== 'all') {
        const cat = getToolCategory(tool.name);
        if (cat !== activeCategory) return false;
      }
      // Search match
      if (search.trim()) {
        const q = search.trim().toLowerCase();
        const matchesName = tool.name.toLowerCase().includes(q);
        const desc = getToolDescription(tool).toLowerCase();
        const rawDesc = (tool.description || '').toLowerCase();
        if (!matchesName && !desc.includes(q) && !rawDesc.includes(q)) return false;
      }
      return true;
    });
  }, [tools, activeCategory, search, t]);

  const activeCount = tools.length - disabledSet.size;

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(Array.from(disabledSet));
      onClose();
    } finally {
      setSaving(false);
    }
  };

  if (!isOpen) return null;

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      icon={<Wrench className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center justify-center shrink-0"
      title={t('ai.toolsManager.title')}
      subtitle={t('ai.toolsManager.subtitle')}
    >
      <div className="p-6 space-y-4 flex flex-col max-h-[75vh]">
        {/* Top Controls: Search & Presets */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="flex-1 max-w-sm">
            <SearchInput
              value={search}
              onChangeValue={setSearch}
              placeholder={t('ai.toolsManager.searchPlaceholder')}
              sizeVariant="sm"
            />
          </div>

          <div className="flex items-center gap-1.5 shrink-0 flex-wrap">
            <Button
              variant="secondary"
              size="xs"
              onClick={handleEnableAll}
              title={t('ai.toolsManager.presetAll')}
            >
              {t('ai.toolsManager.presetAll')}
            </Button>
            <Button
              variant="secondary"
              size="xs"
              onClick={handleReadOnlyOnly}
              title={t('ai.toolsManager.presetReadOnly')}
            >
              {t('ai.toolsManager.presetReadOnly')}
            </Button>
            <Button
              variant="secondary"
              size="xs"
              onClick={handleDisableAll}
              title={t('ai.toolsManager.presetNone')}
            >
              {t('ai.toolsManager.presetNone')}
            </Button>
          </div>
        </div>

        {/* Category Tabs */}
        <div className="shrink-0 overflow-x-auto pb-1">
          <Tabs
            variant="pill"
            size="sm"
            activeTab={activeCategory}
            onChange={(id) => setActiveCategory(id as ToolCategory)}
            tabs={TOOL_CATEGORIES.map((cat) => ({
              id: cat.id,
              label: t(cat.labelKey),
            }))}
          />
        </div>

        {/* Tool Cards List */}
        <div className="flex-1 overflow-y-auto space-y-2 pr-1 min-h-[220px]">
          {filteredTools.length === 0 ? (
            <div className="py-8 text-center text-slate-500 text-xs">
              {t('ai.toolsManager.noResults')}
            </div>
          ) : (
            filteredTools.map((tool) => {
              const isEnabled = !disabledSet.has(tool.name);
              const isMutating = isToolMutating(tool);

              return (
                <div
                  key={tool.name}
                  onClick={() => toggleTool(tool.name)}
                  className={`p-3.5 rounded-xl border transition-all cursor-pointer flex items-start justify-between gap-4 select-none ${
                    isEnabled
                      ? 'bg-surface-850 border-slate-800 hover:border-slate-700 shadow-sm'
                      : 'bg-slate-950/40 border-slate-900 opacity-60 hover:opacity-80'
                  }`}
                >
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-mono text-xs font-semibold text-white">
                        {tool.name}
                      </span>
                      {isMutating ? (
                        <Badge variant="warning" size="xs">
                          <span className="flex items-center gap-1">
                            <ShieldAlert className="w-3 h-3 text-amber-400" />
                            <span>{t('ai.toolsManager.badgeMutating')}</span>
                          </span>
                        </Badge>
                      ) : (
                        <Badge variant="success" size="xs">
                          <span className="flex items-center gap-1">
                            <Shield className="w-3 h-3 text-emerald-400" />
                            <span>{t('ai.toolsManager.badgeReadOnly')}</span>
                          </span>
                        </Badge>
                      )}
                    </div>

                    <p className="text-xs text-slate-400 leading-relaxed">
                      {getToolDescription(tool) || '—'}
                    </p>
                  </div>

                  {/* Toggle Switch */}
                  <div className="shrink-0 pt-0.5">
                    <button
                      type="button"
                      role="switch"
                      aria-checked={isEnabled}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleTool(tool.name);
                      }}
                      className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                        isEnabled ? 'bg-indigo-600' : 'bg-slate-800'
                      }`}
                    >
                      <span
                        aria-hidden="true"
                        className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out ${
                          isEnabled ? 'translate-x-4' : 'translate-x-0'
                        }`}
                      />
                    </button>
                  </div>
                </div>
              );
            })
          )}
        </div>

        {/* Modal Footer */}
        <div className="pt-4 border-t border-slate-800 flex items-center justify-between text-xs text-slate-400">
          <span className="font-medium">
            {t('ai.toolsManager.activeCount', { active: activeCount, total: tools.length })}
          </span>

          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={onClose} disabled={saving}>
              {t('ai.toolsManager.closeBtn')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleSave}
              loading={saving}
            >
              {t('ai.toolsManager.saveBtn')}
            </Button>
          </div>
        </div>
      </div>
    </ModalShell>
  );
};
