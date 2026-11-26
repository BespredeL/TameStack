import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../../utils/errors';

import {
  Check,
  AlertCircle,
  RefreshCw,
  Key,
  Globe,
  Shield,
  Sparkles,
  Bot,
  FileCode,
  RotateCcw,
  Cpu,
} from 'lucide-react';
import { AIProviderConfigDTO } from '../../types';
import { useTranslation } from '../../i18n/LanguageContext';
import { useAlert } from '../../context/AlertContext';
import {
  AIListProviderConfigs,
  AISaveProviderConfig,
  AITestProvider,
  AIListModels,
  AIGetGlobalSettings,
  AISaveGlobalSettings,
  AIGetDefaultSystemPrompt,
} from '../../../wailsjs/go/main/App';
import { ModalShell } from '../common/ModalShell';
import { Input, Button, Tabs } from '../ui';

interface AISettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved?: () => void;
}

export const AISettingsModal: React.FC<AISettingsModalProps> = ({ isOpen, onClose, onSaved }) => {
  const { t, language } = useTranslation();
  const alert = useAlert();
  type ActiveTab = 'providers' | 'prompt';
  const [activeTab, setActiveTab] = useState<ActiveTab>('providers');

  const [configs, setConfigs] = useState<AIProviderConfigDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [testingType, setTestingType] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<Record<string, { success: boolean; msg: string }>>({});

  // Active provider edit state
  type ProviderType = 'ollama' | 'openai_compatible' | 'llamacpp' | 'openai' | 'gemini' | 'anthropic';
  const [selectedType, setSelectedType] = useState<ProviderType>('openai_compatible');
  const [baseURL, setBaseURL] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [defaultModel, setDefaultModel] = useState('');
  const [isDefault, setIsDefault] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);

  // Available models fetched for current provider
  const [fetchedModels, setFetchedModels] = useState<string[]>([]);
  const [isFetchingModels, setIsFetchingModels] = useState(false);

  // System prompt & privacy state
  const [systemPrompt, setSystemPrompt] = useState('');
  const [cloudPrivacyAliasing, setCloudPrivacyAliasing] = useState(true);
  const [promptSaveSuccess, setPromptSaveSuccess] = useState(false);
  const [isSavingPrompt, setIsSavingPrompt] = useState(false);

  const providerDefaults: Record<
    ProviderType,
    { name: string; label: string; url: string; needsKey: boolean; isLocal: boolean; keyHint: string; desc: string; sampleModel: string }
  > = {
    ollama: {
      name: t('ai.settings.ollamaName'),
      label: 'Ollama',
      url: 'http://127.0.0.1:11434',
      needsKey: false,
      isLocal: true,
      keyHint: t('ai.settings.ollamaKeyHint'),
      desc: t('ai.settings.ollamaDesc'),
      sampleModel: 'qwen2.5-coder:7b',
    },
    openai_compatible: {
      name: t('ai.settings.unslothName'),
      label: 'Unsloth / vLLM',
      url: 'http://127.0.0.1:8000/v1',
      needsKey: true,
      isLocal: true,
      keyHint: t('ai.settings.unslothKeyHint'),
      desc: t('ai.settings.unslothDesc'),
      sampleModel: 'unsloth/Qwen2.5-Coder-7B-Instruct',
    },
    llamacpp: {
      name: t('ai.settings.llamacppName'),
      label: 'llama.cpp',
      url: 'http://127.0.0.1:8080',
      needsKey: false,
      isLocal: true,
      keyHint: t('ai.settings.llamacppKeyHint'),
      desc: t('ai.settings.llamacppDesc'),
      sampleModel: 'default',
    },
    openai: {
      name: 'OpenAI API',
      label: 'OpenAI',
      url: 'https://api.openai.com/v1',
      needsKey: true,
      isLocal: false,
      keyHint: t('ai.settings.openaiKeyHint'),
      desc: t('ai.settings.openaiDesc'),
      sampleModel: 'gpt-4o-mini',
    },
    gemini: {
      name: 'Google Gemini',
      label: 'Gemini',
      url: 'https://generativelanguage.googleapis.com/v1beta',
      needsKey: true,
      isLocal: false,
      keyHint: 'Google AI Studio API Key (AIzaSy...)',
      desc: t('ai.settings.geminiDesc'),
      sampleModel: 'gemini-1.5-flash',
    },
    anthropic: {
      name: 'Anthropic Claude',
      label: 'Anthropic',
      url: 'https://api.anthropic.com/v1',
      needsKey: true,
      isLocal: false,
      keyHint: t('ai.settings.anthropicKeyHint'),
      desc: t('ai.settings.anthropicDesc'),
      sampleModel: 'claude-3-5-sonnet-20241022',
    },
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [list, globalSettings] = await Promise.all([
        AIListProviderConfigs() as Promise<AIProviderConfigDTO[]>,
        AIGetGlobalSettings() as Promise<Record<string, string>>,
      ]);

      setConfigs(list || []);

      // If selectedType exists in config, populate inputs
      const current = (list || []).find((c) => c.provider_type === selectedType);
      if (current) {
        setBaseURL(current.base_url);
        setIsDefault(current.is_default);
        setDefaultModel(current.default_model || globalSettings?.['default_model'] || providerDefaults[selectedType].sampleModel);
      } else {
        setBaseURL(providerDefaults[selectedType].url);
        setIsDefault(false);
        setDefaultModel(globalSettings?.['default_model'] || providerDefaults[selectedType].sampleModel);
      }
      setApiKey('');
      setCloudPrivacyAliasing(globalSettings?.['cloud_privacy_aliasing'] !== 'false');

      const langPromptKey = `system_prompt_${language}`;
      if (globalSettings && globalSettings[langPromptKey]) {
        setSystemPrompt(globalSettings[langPromptKey]);
      } else if (globalSettings && globalSettings['system_prompt']) {
        setSystemPrompt(globalSettings['system_prompt']);
      } else {
        const def = await AIGetDefaultSystemPrompt(language);
        setSystemPrompt(def);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      loadData();
    }
  }, [isOpen, selectedType, language]);

  const handleFetchModels = async () => {
    setIsFetchingModels(true);
    try {
      const specs = await AIListModels(selectedType);
      if (specs && specs.length > 0) {
        const ids = specs.map((s: any) => s.id);
        setFetchedModels(ids);
        if (!defaultModel && ids.length > 0) {
          setDefaultModel(ids[0]);
        }
      }
    } catch (err: unknown) {
      console.warn('Failed to fetch models:', err);
    } finally {
      setIsFetchingModels(false);
    }
  };

  if (!isOpen) return null;

  const handleSaveProvider = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const current = configs.find((c) => c.provider_type === selectedType);
      await AISaveProviderConfig({
        id: current ? current.id : '',
        provider_type: selectedType,
        name: providerDefaults[selectedType].name,
        base_url: baseURL,
        api_key: apiKey,
        is_enabled: true,
        is_default: isDefault,
        default_model: defaultModel.trim(),
        system_prompt: '',
      });

      // If set as default, also update global default settings
      if (isDefault) {
        await AISaveGlobalSettings({
          default_provider: selectedType,
          default_model: defaultModel.trim(),
        });
      }

      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 2500);
      await loadData();
      if (onSaved) onSaved();
    } catch (err: unknown) {
      alert(t('ai.settings.saveError', { error: getErrorMessage(err) }));
    } finally {
      setLoading(false);
    }
  };

  const handleSavePrompt = async () => {
    setIsSavingPrompt(true);
    try {
      await AISaveGlobalSettings({
        [`system_prompt_${language}`]: systemPrompt.trim(),
        system_prompt: systemPrompt.trim(),
      });
      setPromptSaveSuccess(true);
      setTimeout(() => setPromptSaveSuccess(false), 2500);
      if (onSaved) onSaved();
    } catch (err: unknown) {
      alert(t('ai.settings.savePromptError', { error: getErrorMessage(err) }));
    } finally {
      setIsSavingPrompt(false);
    }
  };

  const handleResetPromptToDefault = async () => {
    try {
      const defaultText = await AIGetDefaultSystemPrompt(language);
      setSystemPrompt(defaultText);
    } catch (e) {
      console.error(e);
    }
  };

  const handleTest = async (pType: string) => {
    setTestingType(pType);
    try {
      await AITestProvider(pType);
      setTestResult((prev) => ({
        ...prev,
        [pType]: { success: true, msg: t('ai.settings.connectionEstablished') },
      }));
    } catch (err: unknown) {
      setTestResult((prev) => ({
        ...prev,
        [pType]: { success: false, msg: getErrorMessage(err) },
      }));
    } finally {
      setTestingType(null);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="4xl"
      className="max-h-[92vh]"
      icon={<Sparkles className="w-4 h-4" />}
      iconClassName="w-8 h-8 rounded-lg bg-indigo-500/20 text-indigo-400 flex items-center justify-center border border-indigo-500/30 shrink-0"
      title={t('ai.settings.title')}
      subtitle={t('ai.settings.subtitle')}
    >
      {/* Navigation Tabs */}
      <Tabs
        variant="line"
        activeTab={activeTab}
        onChange={(id) => setActiveTab(id as any)}
        className="px-6 bg-surface-900 shrink-0"
        tabs={[
          {
            id: 'providers',
            label: t('ai.settings.tabProviders'),
            icon: <Bot className="w-4 h-4 shrink-0" />,
          },
          {
            id: 'prompt',
            label: t('ai.settings.tabSystemPrompt'),
            icon: <FileCode className="w-4 h-4 shrink-0" />,
          },
        ]}
      />

      {/* Modal Body */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {activeTab === 'providers' && (
          <>
            {/* Provider Select Tabs */}
            <div className="grid grid-cols-3 sm:grid-cols-6 gap-2">
              {(['ollama', 'openai_compatible', 'llamacpp', 'openai', 'gemini', 'anthropic'] as const).map((type) => {
                const active = selectedType === type;
                const configured = configs.find((c) => c.provider_type === type);
                const isCurrentDefault = configured?.is_default;
                return (
                  <button
                    key={type}
                    type="button"
                    onClick={() => {
                      setSelectedType(type);
                      setFetchedModels([]);
                    }}
                    className={`flex flex-col items-center justify-center p-2 rounded-lg border text-xs font-medium transition-all relative ${
                      active
                        ? 'bg-indigo-600/20 border-indigo-500 text-white shadow-md'
                        : 'bg-slate-850 border-slate-800 text-slate-400 hover:border-slate-700 hover:text-slate-200'
                    }`}
                  >
                    <span className="font-semibold text-center truncate max-w-full">
                      {providerDefaults[type].label}
                    </span>
                    <span className="text-[10px] mt-0.5 text-slate-400">
                      {providerDefaults[type].isLocal ? t('ai.settings.localBadge') : t('ai.settings.cloudBadge')}
                    </span>
                    {isCurrentDefault && (
                      <span className="absolute top-1 right-1 w-2 h-2 rounded-full bg-emerald-400 ring-2 ring-emerald-500/30" title={t('ai.settings.primaryProviderTooltip')} />
                    )}
                    {configured && !isCurrentDefault && (
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-500 mt-1" title={t('ai.settings.configuredTooltip')} />
                    )}
                  </button>
                );
              })}
            </div>

            {/* Form */}
            <form onSubmit={handleSaveProvider} className="space-y-4 bg-slate-950/60 p-4 rounded-xl border border-slate-800/80">
              <div className="flex flex-col gap-1 pb-2 border-b border-slate-800 text-xs">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-white">{providerDefaults[selectedType].name}</span>
                  <span className="text-slate-400 flex items-center gap-1">
                    <Shield className="w-3.5 h-3.5 text-emerald-400" />
                    {t('ai.settings.keysEncryptedNotice')}
                  </span>
                </div>
                <p className="text-[11px] text-slate-400">{providerDefaults[selectedType].desc}</p>
              </div>

              {/* Base URL */}
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1 flex items-center gap-1.5">
                  <Globe className="w-3.5 h-3.5 text-indigo-400" />
                  {t('ai.settings.baseUrlLabel')}
                </label>
                <Input
                  type="text"
                  value={baseURL}
                  onChange={(e) => setBaseURL(e.target.value)}
                  placeholder={providerDefaults[selectedType].url}
                  className="font-mono"
                  required
                />
              </div>

              {/* API Key */}
              {(providerDefaults[selectedType].needsKey || selectedType === 'llamacpp') && (
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1 flex items-center justify-between">
                    <span className="flex items-center gap-1.5">
                      <Key className="w-3.5 h-3.5 text-amber-400" />
                      {t('ai.settings.apiKeyLabel')} {selectedType === 'llamacpp' ? t('ai.settings.optionalSuffix') : ''}
                    </span>
                    <span className="text-[11px] text-slate-400">{providerDefaults[selectedType].keyHint}</span>
                  </label>
                  <Input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={
                      configs.find((c) => c.provider_type === selectedType)?.has_api_key
                        ? t('ai.settings.keySavedPlaceholder')
                        : providerDefaults[selectedType].keyHint
                    }
                    className="font-mono"
                  />
                </div>
              )}

              {/* Default Model */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-xs font-medium text-slate-300 flex items-center gap-1.5">
                    <Cpu className="w-3.5 h-3.5 text-cyan-400" />
                    {t('ai.settings.defaultModelLabel')}
                  </label>
                  <Button
                    variant="ghost"
                    size="xs"
                    onClick={handleFetchModels}
                    loading={isFetchingModels}
                    icon={<RefreshCw className="w-3 h-3 text-indigo-400" />}
                    className="text-[11px] text-indigo-400 hover:text-indigo-300 p-0 h-auto"
                  >
                    {isFetchingModels ? t('ai.settings.fetchingModels') : t('ai.settings.loadModelsBtn')}
                  </Button>
                </div>

                <div className="relative">
                  <Input
                    type="text"
                    list="provider-models-list"
                    value={defaultModel}
                    onChange={(e) => setDefaultModel(e.target.value)}
                    placeholder={providerDefaults[selectedType].sampleModel}
                    className="font-mono"
                  />
                  <datalist id="provider-models-list">
                    {fetchedModels.map((m) => (
                      <option key={m} value={m} />
                    ))}
                  </datalist>
                </div>
                <p className="text-[11px] text-slate-400 mt-1">
                  {t('ai.settings.modelAutoSelectHint', { provider: providerDefaults[selectedType].label })}
                </p>
              </div>

              {/* Default Provider checkbox */}
              <div className="flex items-center gap-2 pt-1">
                <input
                  type="checkbox"
                  id="isDefault"
                  checked={isDefault}
                  onChange={(e) => setIsDefault(e.target.checked)}
                  className="rounded border-slate-700 bg-slate-900 text-indigo-600 focus:ring-0"
                />
                <label htmlFor="isDefault" className="text-xs text-slate-300 select-none cursor-pointer">
                  {t('ai.settings.makePrimaryCheckbox')}
                </label>
              </div>

              {/* Actions */}
              <div className="flex items-center justify-between pt-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => handleTest(selectedType)}
                  loading={testingType === selectedType}
                  icon={<RefreshCw className="w-3.5 h-3.5" />}
                >
                  {t('ai.settings.testConnectionBtn')}
                </Button>

                <Button
                  type="submit"
                  variant="primary"
                  size="sm"
                  loading={loading}
                  icon={saveSuccess ? <Check className="w-3.5 h-3.5" /> : undefined}
                >
                  {saveSuccess ? t('ai.settings.savedBadge') : t('ai.settings.saveParamsBtn')}
                </Button>
              </div>

              {/* Test Status feedback */}
              {testResult[selectedType] && (
                <div
                  className={`mt-2 p-2.5 rounded-lg text-xs flex items-start gap-2 border ${
                    testResult[selectedType].success
                      ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                      : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                  }`}
                >
                  {testResult[selectedType].success ? (
                    <Check className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
                  ) : (
                    <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  )}
                  <div>
                    <div className="font-semibold">{testResult[selectedType].success ? t('ai.settings.testSuccess') : t('ai.settings.testError')}</div>
                    <div className="text-[11px] opacity-90 mt-0.5">{testResult[selectedType].msg}</div>
                  </div>
                </div>
              )}
            </form>
          </>
        )}

        {activeTab === 'prompt' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <FileCode className="w-4 h-4 text-indigo-400" />
                  {t('ai.settings.systemPromptHeading')}
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  {t('ai.settings.systemPromptDesc')}
                </p>
              </div>
              <Button
                variant="secondary"
                size="xs"
                className="whitespace-nowrap shrink-0"
                onClick={handleResetPromptToDefault}
                icon={<RotateCcw className="w-3.5 h-3.5" />}
                title={t('ai.settings.restoreDefaultPromptTooltip')}
              >
                {t('ai.settings.defaultPromptBtn')}
              </Button>
            </div>

            <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800">
              <textarea
                rows={13}
                value={systemPrompt}
                onChange={(e) => setSystemPrompt(e.target.value)}
                placeholder={t('ai.settings.systemPromptPlaceholder')}
                className="w-full bg-transparent text-xs font-mono text-slate-200 focus:outline-none resize-y leading-relaxed"
              />
            </div>

            <div className="p-3.5 rounded-xl bg-slate-950/70 border border-slate-800 flex items-start justify-between gap-4">
              <div className="space-y-1">
                <div className="text-xs font-semibold text-white flex items-center gap-2">
                  <Shield className="w-3.5 h-3.5 text-emerald-400" />
                  {t('ai.settings.cloudMaskTitle')}
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">
                  {t('ai.settings.cloudMaskDesc')}
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer shrink-0 mt-0.5">
                <input
                  type="checkbox"
                  checked={cloudPrivacyAliasing}
                  onChange={async (e) => {
                    const next = e.target.checked;
                    setCloudPrivacyAliasing(next);
                    try {
                      await AISaveGlobalSettings({ cloud_privacy_aliasing: next ? 'true' : 'false' });
                    } catch (err: unknown) {
                      alert(getErrorMessage(err));
                    }
                  }}
                  className="sr-only peer"
                />
                <div className="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-emerald-600 border border-slate-700" />
              </label>
            </div>

            <div className="p-3 rounded-lg bg-slate-950/50 border border-slate-800/80 text-[11px] text-slate-400 space-y-1">
              <p className="text-slate-300 font-semibold">{t('ai.settings.contextNoteHeading')}</p>
              <p>
                {t('ai.settings.contextNoteDesc')}
              </p>
            </div>

            <div className="flex items-center justify-end gap-3 pt-2">
              <Button
                variant="primary"
                size="md"
                onClick={handleSavePrompt}
                loading={isSavingPrompt}
                icon={promptSaveSuccess ? <Check className="w-4 h-4 text-emerald-300" /> : undefined}
              >
                {promptSaveSuccess ? t('ai.settings.promptSavedBadge') : t('ai.settings.savePromptBtn')}
              </Button>
            </div>
          </div>
        )}
      </div>
    </ModalShell>
  );
};
