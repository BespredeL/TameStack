import React, { useState, useMemo } from 'react';
import { Sliders, Shield, Plus, Trash2, Edit2, CheckCircle2, XCircle, AlertTriangle, Sparkles, BookOpen } from 'lucide-react';
import { ModalShell } from '../common/ModalShell';
import { security } from '../../../wailsjs/go/models';
import { useTranslation } from '../../i18n/LanguageContext';
import { Input, Button, Select } from '../ui';

interface SignaturesModalProps {
  onClose: () => void;
  signatures: security.FileSecuritySignature[];
  isLoading: boolean;
  onSaveSignature: (sig: security.FileSecuritySignature) => Promise<void>;
  onDeleteSignature: (id: string) => Promise<void>;
  onToggleSignature: (id: string, enabled: boolean) => Promise<void>;
}

export const SignaturesModal: React.FC<SignaturesModalProps> = ({
  onClose,
  signatures,
  isLoading,
  onSaveSignature,
  onDeleteSignature,
  onToggleSignature,
}) => {
  const { t } = useTranslation();

  const [activeTab, setActiveTab] = useState<'custom' | 'builtin'>('custom');

  // Form state
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [pattern, setPattern] = useState('');
  const [threatLevel, setThreatLevel] = useState<string>('malicious');
  const [description, setDescription] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [testString, setTestString] = useState('');

  // Live regex tester calculation
  const regexTestResult = useMemo(() => {
    if (!pattern.trim()) {
      return { status: 'empty', message: '' };
    }
    let re: RegExp;
    try {
      re = new RegExp(pattern, 'i');
    } catch (e: any) {
      return { status: 'invalid', message: e?.message || t('fileScanner.regexInvalid') };
    }
    if (!testString) {
      return { status: 'valid', message: '' };
    }
    if (re.test(testString)) {
      return { status: 'match', message: t('fileScanner.regexMatchFound') };
    }
    return { status: 'no-match', message: t('fileScanner.regexNoMatch') };
  }, [pattern, testString, t]);

  const handleApplyPreset = (presetName: string, presetPattern: string, presetLevel: string, presetDesc: string) => {
    setName(presetName);
    setPattern(presetPattern);
    setThreatLevel(presetLevel);
    setDescription(presetDesc);
  };

  const handleStartEdit = (sig: security.FileSecuritySignature) => {
    setEditingId(sig.id);
    setName(sig.name);
    setPattern(sig.pattern);
    setThreatLevel(sig.threat_level || 'malicious');
    setDescription(sig.description || '');
    setActiveTab('custom');
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setName('');
    setPattern('');
    setThreatLevel('malicious');
    setDescription('');
    setTestString('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !pattern.trim()) return;

    try {
      new RegExp(pattern);
    } catch (err: any) {
      return;
    }

    setIsSaving(true);
    try {
      const sig = new security.FileSecuritySignature({
        id: editingId || '',
        server_id: '',
        name: name.trim(),
        pattern: pattern.trim(),
        threat_level: threatLevel,
        description: description.trim(),
        enabled: true,
        created_at: '',
        updated_at: '',
      });
      await onSaveSignature(sig);
      handleCancelEdit();
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="4xl"
      className="max-h-[88vh]"
      icon={<Sliders className="w-5 h-5 text-indigo-400" />}
      iconClassName=""
      title={t('fileScanner.signaturesModalTitle')}
      subtitle={t('fileScanner.signaturesSubtitle')}
    >
      <div className="flex border-b border-slate-800 px-6 pt-3 bg-slate-900/40">
        <button
          type="button"
          onClick={() => setActiveTab('custom')}
          className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'custom'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Sliders className="w-3.5 h-3.5" />
          <span>{t('fileScanner.customSignaturesTab')} ({signatures.length})</span>
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('builtin')}
          className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors flex items-center gap-1.5 ${
            activeTab === 'builtin'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <BookOpen className="w-3.5 h-3.5" />
          <span>{t('fileScanner.builtinSignaturesTab')}</span>
        </button>
      </div>

      <div className="p-6 overflow-y-auto space-y-6 flex-1">
        {activeTab === 'builtin' ? (
          <div className="space-y-4">
            <div className="p-4 rounded-xl border border-rose-900/30 bg-rose-950/10 space-y-2">
              <div className="flex items-center gap-2 text-rose-400 font-semibold text-xs">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                <span>{t('fileScanner.builtinWebshellsTitle')}</span>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed">
                {t('fileScanner.builtinWebshellsDesc')}
              </p>
              <div className="bg-slate-950 p-2.5 rounded-lg border border-slate-800 font-mono text-[11px] text-rose-300/80 break-all select-all">
                eval\s*\(.*(base64_decode|gzinflate|str_rot13...)|assert\s*\(\$_(POST|GET)|system\s*\(\$_(POST|GET)...
              </div>
            </div>

            <div className="p-4 rounded-xl border border-amber-900/30 bg-amber-950/10 space-y-2">
              <div className="flex items-center gap-2 text-amber-400 font-semibold text-xs">
                <Shield className="w-4 h-4 shrink-0" />
                <span>{t('fileScanner.builtinDoubleExtTitle')}</span>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed">
                {t('fileScanner.builtinDoubleExtDesc')}
              </p>
            </div>

            <div className="p-4 rounded-xl border border-amber-900/30 bg-amber-950/10 space-y-2">
              <div className="flex items-center gap-2 text-amber-400 font-semibold text-xs">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                <span>{t('fileScanner.builtinDangerousPermsTitle')}</span>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed">
                {t('fileScanner.builtinDangerousPermsDesc')}
              </p>
            </div>

            <div className="p-4 rounded-xl border border-rose-900/30 bg-rose-950/10 space-y-2">
              <div className="flex items-center gap-2 text-rose-400 font-semibold text-xs">
                <Shield className="w-4 h-4 shrink-0" />
                <span>{t('fileScanner.builtinImageInjectionTitle')}</span>
              </div>
              <p className="text-xs text-slate-300 leading-relaxed">
                {t('fileScanner.builtinImageInjectionDesc')}
              </p>
            </div>
          </div>
        ) : (
          <>
            {/* Form to Add / Edit Signature */}
            <form onSubmit={handleSubmit} className="p-4 bg-slate-950/60 rounded-xl border border-slate-800 space-y-4">
              <div className="flex items-center justify-between">
                <h4 className="text-xs font-bold text-slate-200 flex items-center gap-1.5">
                  <Sparkles className="w-4 h-4 text-indigo-400" />
                  <span>{editingId ? t('fileScanner.editSignatureBtn') : t('fileScanner.addSignatureBtn')}</span>
                </h4>
                {editingId && (
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    onClick={handleCancelEdit}
                  >
                    {t('common.cancel')}
                  </Button>
                )}
              </div>

              {/* Quick Preset Buttons */}
              <div className="space-y-1.5">
                <div className="text-[11px] text-slate-400">{t('fileScanner.quickPresets')}</div>
                <div className="flex flex-wrap gap-1.5">
                  <button
                    type="button"
                    onClick={() =>
                      handleApplyPreset(
                        'Crypto Miner',
                        '(stratum\\+tcp|monerohash|xmrig|minergate|cryptonight)',
                        'malicious',
                        'Обнаружение скрытых майнинг-пулов и бинарных скриптов майнеров'
                      )
                    }
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700/60 transition-colors"
                  >
                    {t('fileScanner.presetCryptoMiner')}
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      handleApplyPreset(
                        'Python Reverse Shell',
                        '(socket\\.socket|pty\\.spawn|subprocess\\.Popen.*\\/bin\\/(ba)?sh)',
                        'malicious',
                        'Поиск сетевых реверс-шеллов на Python'
                      )
                    }
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700/60 transition-colors"
                  >
                    {t('fileScanner.presetPyReverseShell')}
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      handleApplyPreset(
                        'Base64 + Strrev / Rot13',
                        '(strrev\\s*\\(|str_rot13\\s*\\()\\s*[\'"][A-Za-z0-9+/=]{20,}',
                        'suspicious',
                        'Обфусцированные пейлоады с переворотом строк или rot13'
                      )
                    }
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700/60 transition-colors"
                  >
                    {t('fileScanner.presetObfuscatedPhp')}
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      handleApplyPreset(
                        'Bash Reverse Shell',
                        '(\\$\\{[^}]+\\}){4,}|(base64\\s+-d\\s*\\|\\s*(sh|bash))',
                        'malicious',
                        'Обфусцированные команды в командной оболочке bash'
                      )
                    }
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700/60 transition-colors"
                  >
                    {t('fileScanner.presetBashReverseShell')}
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div className="sm:col-span-2">
                  <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.signatureNameLabel')}</label>
                  <Input
                    type="text"
                    sizeVariant="sm"
                    placeholder={t('fileScanner.signatureNamePlaceholder')}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.signatureThreatLevelLabel')}</label>
                  <Select
                    sizeVariant="sm"
                    value={threatLevel}
                    onChange={(e) => setThreatLevel(e.target.value)}
                  >
                    <option value="malicious">{t('fileScanner.threatMalicious')}</option>
                    <option value="suspicious">{t('fileScanner.threatSuspicious')}</option>
                  </Select>
                </div>
              </div>

              <div>
                <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.signaturePatternLabel')}</label>
                <Input
                  type="text"
                  sizeVariant="sm"
                  placeholder={t('fileScanner.signaturePatternPlaceholder')}
                  value={pattern}
                  onChange={(e) => setPattern(e.target.value)}
                  className="font-mono text-xs"
                  required
                />
                {regexTestResult.status === 'invalid' && (
                  <p className="text-[11px] text-rose-400 mt-1 flex items-center gap-1">
                    <XCircle className="w-3.5 h-3.5 shrink-0" />
                    <span>{regexTestResult.message}</span>
                  </p>
                )}
              </div>

              {/* Interactive Live Regex Tester */}
              <div className="p-3 bg-slate-900/60 rounded-xl border border-slate-800/80 space-y-2">
                <div className="flex items-center justify-between text-[11px]">
                  <span className="text-slate-400 font-medium flex items-center gap-1.5">
                    <Sparkles className="w-3 h-3 text-indigo-400" />
                    <span>{t('fileScanner.liveRegexTester')}</span>
                  </span>
                  {regexTestResult.status === 'match' && (
                    <span className="text-emerald-400 font-semibold flex items-center gap-1">
                      <CheckCircle2 className="w-3.5 h-3.5" />
                      <span>{regexTestResult.message}</span>
                    </span>
                  )}
                  {regexTestResult.status === 'no-match' && (
                    <span className="text-slate-400 flex items-center gap-1">
                      <span>{regexTestResult.message}</span>
                    </span>
                  )}
                </div>
                <Input
                  type="text"
                  sizeVariant="sm"
                  placeholder={t('fileScanner.testStringPlaceholder')}
                  value={testString}
                  onChange={(e) => setTestString(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>

              <div>
                <label className="text-[11px] text-slate-400 block mb-1">{t('fileScanner.signatureDescriptionLabel')}</label>
                <Input
                  type="text"
                  sizeVariant="sm"
                  placeholder={t('fileScanner.signatureDescriptionPlaceholder')}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>

              <div className="flex items-center gap-2">
                <Button
                  type="submit"
                  variant="primary"
                  size="sm"
                  loading={isSaving}
                  disabled={!name.trim() || !pattern.trim() || regexTestResult.status === 'invalid'}
                  icon={<Plus className="w-3.5 h-3.5" />}
                >
                  {editingId ? t('fileScanner.saveSignatureBtn') : t('fileScanner.addSignatureBtn')}
                </Button>
                {editingId && (
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={handleCancelEdit}
                  >
                    {t('common.cancel')}
                  </Button>
                )}
              </div>
            </form>

            {/* List of Custom Signatures */}
            <div className="space-y-3">
              <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">
                {t('fileScanner.activeSignaturesHeader', { count: signatures.length })}
              </h4>

              {signatures.length === 0 ? (
                <div className="p-8 text-center border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs">
                  {t('fileScanner.emptySignaturesText')}
                </div>
              ) : (
                <div className="divide-y divide-slate-800 border border-slate-800 rounded-xl bg-slate-950 overflow-hidden">
                  {signatures.map((sig) => (
                    <div key={sig.id} className="p-3.5 flex items-start justify-between gap-4">
                      <div className="min-w-0 space-y-1.5 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-xs font-semibold text-white">
                            {sig.name}
                          </span>
                          <span
                            className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${
                              sig.threat_level === 'malicious'
                                ? 'bg-rose-950/80 text-rose-300 border border-rose-800/60'
                                : 'bg-amber-950/80 text-amber-300 border border-amber-800/60'
                            }`}
                          >
                            {sig.threat_level === 'malicious'
                              ? t('fileScanner.threatMalicious')
                              : t('fileScanner.threatSuspicious')}
                          </span>
                          <label className="inline-flex items-center gap-1.5 text-[11px] text-slate-400 cursor-pointer ml-2">
                            <input
                              type="checkbox"
                              checked={sig.enabled}
                              onChange={(e) => onToggleSignature(sig.id, e.target.checked)}
                              className="rounded border-slate-700 bg-slate-900 text-indigo-600 focus:ring-0 cursor-pointer"
                            />
                            <span>{sig.enabled ? t('common.enabled') : t('common.disabled')}</span>
                          </label>
                        </div>

                        <div className="font-mono text-xs text-indigo-300/90 bg-slate-900/80 px-2 py-1 rounded border border-slate-800/80 break-all select-all">
                          {sig.pattern}
                        </div>

                        {sig.description && (
                          <p className="text-[11px] text-slate-400">{sig.description}</p>
                        )}
                      </div>

                      <div className="flex items-center gap-1 shrink-0 pt-0.5">
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => handleStartEdit(sig)}
                          className="text-slate-400 hover:text-white"
                          title={t('fileScanner.editSignatureBtn')}
                          icon={<Edit2 className="w-3.5 h-3.5" />}
                        />
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => onDeleteSignature(sig.id)}
                          className="text-rose-400 hover:text-rose-300 hover:bg-rose-950/30"
                          title={t('common.delete')}
                          icon={<Trash2 className="w-3.5 h-3.5" />}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </ModalShell>
  );
};
