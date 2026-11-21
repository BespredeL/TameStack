import React, { useState, useEffect } from 'react';
import { getErrorMessage } from '../utils/errors';

import { ShieldAlert, ShieldCheck, Server, CheckCircle2, AlertTriangle, Edit3 } from 'lucide-react';
import { AddServer, UpdateServer, InspectHostKey, AcceptHostKey, TriggerDiagnostics } from '../../wailsjs/go/main/App';
import { HostKeyInspection, ServerRecord } from '../types';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Input, Select, Button } from './ui';

interface AddServerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onServerAdded: () => void;
  serverToEdit?: ServerRecord | null;
}

export const AddServerModal: React.FC<AddServerModalProps> = ({
  isOpen,
  onClose,
  onServerAdded,
  serverToEdit,
}) => {
  const { t } = useTranslation();
  const isEditMode = Boolean(serverToEdit);

  const [name, setName] = useState('');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(22);
  const [username, setUsername] = useState('root');
  const [authType, setAuthType] = useState<'password' | 'key' | 'key_passphrase'>('key');
  const [secret, setSecret] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [sudoPassword, setSudoPassword] = useState('');
  const [groupName, setGroupName] = useState('Production');
  const [tagsInput, setTagsInput] = useState('web, vps');

  // Security & HostKey Verification state
  const [isInspecting, setIsInspecting] = useState(false);
  const [inspection, setInspection] = useState<HostKeyInspection | null>(null);
  const [isKeyTrusted, setIsKeyTrusted] = useState(false);
  const [inspectError, setInspectError] = useState<string | null>(null);

  // Saving state
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (serverToEdit) {
      setName(serverToEdit.name || '');
      setHost(serverToEdit.host || '');
      setPort(serverToEdit.port || 22);
      setUsername(serverToEdit.username || 'root');
      setAuthType((serverToEdit.auth_type as 'password' | 'key' | 'key_passphrase') || 'key');
      setSecret('');
      setPassphrase('');
      setSudoPassword('');
      setGroupName(serverToEdit.group_name || 'Production');
      setTagsInput(serverToEdit.tags?.join(', ') || '');
      setIsKeyTrusted(true);
      setInspection(null);
      setInspectError(null);
      setSaveError(null);
    } else {
      setName('');
      setHost('');
      setPort(22);
      setUsername('root');
      setAuthType('key');
      setSecret('');
      setPassphrase('');
      setSudoPassword('');
      setGroupName('Production');
      setTagsInput('web, vps');
      setIsKeyTrusted(false);
      setInspection(null);
      setInspectError(null);
      setSaveError(null);
    }
  }, [serverToEdit, isOpen]);

  useEffect(() => {
    if (!isEditMode || !serverToEdit) return;
    const changed = host.trim() !== serverToEdit.host.trim() || Number(port) !== serverToEdit.port;
    if (changed) {
      setIsKeyTrusted(false);
      setInspection(null);
    }
  }, [host, port, isEditMode, serverToEdit]);

  if (!isOpen) return null;

  const hasAddressChanged =
    isEditMode &&
    serverToEdit &&
    (host.trim() !== serverToEdit.host.trim() || Number(port) !== serverToEdit.port);

  const handleInspectHostKey = async () => {
    if (!host) {
      setInspectError(t('servers.addModal.invalidIpOrDomain'));
      return;
    }
    setIsInspecting(true);
    setInspectError(null);
    setInspection(null);

    try {
      const res = await InspectHostKey(host, port);
      setInspection(res as HostKeyInspection);
      if (res.status === 'known') {
        setIsKeyTrusted(true);
      } else {
        setIsKeyTrusted(false);
      }
    } catch (err: unknown) {
      setInspectError(getErrorMessage(err));
    } finally {
      setIsInspecting(false);
    }
  };

  const handleTrustKey = async () => {
    if (!inspection || !inspection.raw_entry) return;
    try {
      await AcceptHostKey(inspection.raw_entry);
      setIsKeyTrusted(true);
    } catch (err: unknown) {
      setInspectError(t('servers.addModal.saveHostKeyError', { error: getErrorMessage(err) }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!host) {
      setSaveError(t('servers.addModal.hostRequired'));
      return;
    }

    if (!isEditMode && !secret) {
      setSaveError(t('servers.addModal.authRequired'));
      return;
    }

    if (!isKeyTrusted && inspection?.status !== 'known') {
      setSaveError(t('servers.addModal.checkKeyRequired'));
      return;
    }

    setIsSaving(true);
    setSaveError(null);

    const tags = tagsInput
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0);

    try {
      if (isEditMode && serverToEdit) {
        await UpdateServer({
          id: serverToEdit.id,
          name: name || host,
          host,
          port: Number(port),
          username: username || 'root',
          auth_type: authType,
          secret,
          passphrase,
          sudo_password: sudoPassword,
          ssh_host_key: inspection?.fingerprint || serverToEdit.ssh_host_key || '',
          group_name: groupName || 'Default',
          tags,
          is_favorite: serverToEdit.is_favorite,
        } as any);
      } else {
        const added = await AddServer({
          id: '',
          name: name || host,
          host,
          port: Number(port),
          username: username || 'root',
          auth_type: authType,
          secret,
          passphrase,
          sudo_password: sudoPassword,
          ssh_host_key: inspection?.fingerprint || '',
          group_name: groupName || 'Default',
          tags,
          is_favorite: false,
        } as any);

        // Automatically trigger initial diagnostics in background
        try {
          await TriggerDiagnostics(added.id);
        } catch (diagErr) {
          console.warn('Initial diagnostics trigger failed:', diagErr);
        }
      }

      onServerAdded();
      onClose();
    } catch (err: unknown) {
      setSaveError(getErrorMessage(err));
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="2xl"
      className="max-h-[90vh]"
      icon={isEditMode ? <Edit3 className="w-5 h-5" /> : <Server className="w-5 h-5" />}
      iconClassName={`p-2 rounded-lg border shrink-0 ${isEditMode ? 'bg-cyan-600/20 text-cyan-400 border-cyan-500/30' : 'bg-emerald-600/20 text-emerald-400 border-emerald-500/30'}`}
      title={isEditMode ? t('servers.addModal.titleEdit') : t('servers.addModal.titleAdd')}
      subtitle={isEditMode ? t('servers.addModal.editSubtitle', { name: serverToEdit?.name || '' }) : t('servers.addModal.addSubtitle')}
    >
      {/* Modal Form Body */}
      <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-5">
        {saveError && (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 shrink-0" />
            <span>{saveError}</span>
          </div>
        )}

        {hasAddressChanged && (
          <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-300 text-xs flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 shrink-0 text-amber-400" />
            <span>
              <strong>{t('common.warning')}:</strong> {t('servers.addModal.ipChangedWarning')}
            </span>
          </div>
        )}

        {/* Basic Connection Info */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.serverNameLabel')}</label>
            <Input
              type="text"
              placeholder={t('servers.addModal.serverNamePlaceholder')}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.serverGroupLabel')}</label>
            <Input
              type="text"
              placeholder={t('servers.addModal.serverGroupPlaceholder')}
              value={groupName}
              onChange={(e) => setGroupName(e.target.value)}
            />
          </div>
        </div>

        <div className="grid grid-cols-3 gap-4">
          <div className="col-span-2">
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.ipHostLabel')}</label>
            <Input
              type="text"
              required
              placeholder={t('servers.addModal.ipHostPlaceholder')}
              value={host}
              onChange={(e) => {
                setHost(e.target.value);
                if (isEditMode && e.target.value.trim() !== serverToEdit?.host.trim()) {
                  setIsKeyTrusted(false);
                }
              }}
              className="font-mono"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.sshPortLabel')}</label>
            <Input
              type="number"
              value={port}
              onChange={(e) => {
                const p = parseInt(e.target.value) || 22;
                setPort(p);
                if (isEditMode && p !== serverToEdit?.port) {
                  setIsKeyTrusted(false);
                }
              }}
              className="font-mono"
            />
          </div>
        </div>

        {/* SSH Host Key Verification Section */}
        <div className="p-4 rounded-xl bg-slate-950/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              <span className="text-xs font-bold text-slate-200">
                {isEditMode ? t('servers.addModal.step1KeyEdit') : t('servers.addModal.step1Key')}
              </span>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleInspectHostKey}
              loading={isInspecting}
              disabled={!host}
            >
              {t('servers.addModal.checkHostKey')}
            </Button>
          </div>

          {inspectError && (
            <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
              {inspectError}
            </div>
          )}

          {inspection && (
            <div className="space-y-2 pt-2 border-t border-slate-800/80">
              <div className="flex items-center justify-between">
                <span className="text-xs text-slate-400">
                  {t('servers.addModal.keyType')} <span className="font-mono text-slate-200">{inspection.key_type}</span>
                </span>
                <div className="flex items-center gap-1.5">
                  {inspection.status === 'known' && (
                    <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-400 text-[11px] font-semibold border border-emerald-500/30 flex items-center gap-1">
                      <CheckCircle2 className="w-3 h-3" /> {t('servers.addModal.trustedInKnownHosts')}
                    </span>
                  )}
                  {inspection.status === 'new' && (
                    <span className="px-2 py-0.5 rounded bg-amber-500/20 text-amber-300 text-[11px] font-semibold border border-amber-500/30 flex items-center gap-1">
                      <AlertTriangle className="w-3 h-3" /> {t('servers.addModal.newHostKey')}
                    </span>
                  )}
                  {inspection.status === 'mismatch' && (
                    <span className="px-2 py-0.5 rounded bg-rose-500/20 text-rose-300 text-[11px] font-bold border border-rose-500/40 flex items-center gap-1">
                      <ShieldAlert className="w-3 h-3" /> {t('servers.addModal.mismatchHostKey')}
                    </span>
                  )}
                </div>
              </div>

              <div className="p-2 bg-slate-900 rounded font-mono text-[11px] text-slate-300 break-all select-all">
                Fingerprint: {inspection.fingerprint}
              </div>

              {inspection.status === 'new' && !isKeyTrusted && (
                <div className="flex items-center justify-between pt-1">
                  <span className="text-[11px] text-amber-400">{t('servers.addModal.trustFingerprintQuestion')}</span>
                  <Button
                    variant="success"
                    size="sm"
                    onClick={handleTrustKey}
                  >
                    {t('servers.addModal.trustAndAddBtn')}
                  </Button>
                </div>
              )}
            </div>
          )}

          {isKeyTrusted && (
            <div className="text-[11px] text-emerald-400 flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5" /> {t('servers.addModal.hostKeyVerifiedAndTrusted')}
            </div>
          )}
        </div>

        {/* Authentication Section */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-200">
              {isEditMode ? t('servers.addModal.step2CredsEdit') : t('servers.addModal.step2Creds')}
            </span>
            {isEditMode && (
              <span className="text-[11px] text-slate-400 italic">
                {t('servers.addModal.leaveEmptyHint')}
              </span>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.sshUser')}</label>
              <Input
                type="text"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.authMethod')}</label>
              <Select
                value={authType}
                onChange={(e) => setAuthType(e.target.value as 'password' | 'key' | 'key_passphrase')}
              >
                <option value="key">{t('servers.addModal.authKey')}</option>
                <option value="key_passphrase">{t('servers.addModal.authKeyPass')}</option>
                <option value="password">{t('servers.addModal.authPassword')}</option>
              </Select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-300 mb-1">
              {authType === 'password' ? t('servers.addModal.passwordLabel') : t('servers.addModal.keyLabel')}
            </label>
            {authType === 'password' ? (
              <Input
                type="password"
                required={!isEditMode}
                placeholder={
                  isEditMode
                    ? t('servers.addModal.passwordLeaveEmpty')
                    : t('servers.addModal.passwordEnter')
                }
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
              />
            ) : (
              <textarea
                rows={4}
                required={!isEditMode}
                placeholder={
                  isEditMode
                    ? t('servers.addModal.keyLeaveEmpty')
                    : '-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----'
                }
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                className="w-full px-3.5 py-2.5 bg-slate-950/80 border border-slate-800 hover:border-slate-700 rounded-xl text-xs font-mono text-slate-200 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30 transition resize-none"
              />
            )}
          </div>

          {authType === 'key_passphrase' && (
            <div>
              <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.keyPassphraseLabel')}</label>
              <Input
                type="password"
                placeholder={isEditMode ? t('servers.addModal.keyPassphrasePlaceholderEdit') : t('servers.addModal.keyPassphrasePlaceholder')}
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
              />
            </div>
          )}

          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-xs font-semibold text-slate-300">
                {t('servers.addModal.sudoPasswordLabel')}
              </label>
              <span className="text-[10px] text-slate-500">
                {authType === 'password' ? t('servers.addModal.sudoDefaultPassword') : t('servers.addModal.sudoOptional')}
              </span>
            </div>
            <Input
              type="password"
              placeholder={
                authType === 'password'
                  ? (isEditMode ? t('servers.addModal.sudoPasswordLeaveEmptyEdit') : t('servers.addModal.sudoPasswordLeaveEmpty'))
                  : (isEditMode ? t('servers.addModal.sudoPasswordLeaveEmptyEdit') : t('servers.addModal.sudoPasswordEnter'))
              }
              value={sudoPassword}
              onChange={(e) => setSudoPassword(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-300 mb-1">{t('servers.addModal.tagsLabel')}</label>
          <Input
            type="text"
            placeholder={t('servers.addModal.tagsPlaceholder')}
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
          />
        </div>
      </form>

      {/* Modal Footer */}
      <div className="px-6 py-4 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
        <div className="text-xs text-slate-400">
          {isKeyTrusted ? (
            <span className="text-emerald-400 flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5" /> {t('servers.addModal.readyToSave')}
            </span>
          ) : (
            <span>{t('servers.addModal.needCheckKey')}</span>
          )}
        </div>

        <div className="flex items-center gap-3">
          <Button
            variant="secondary"
            size="md"
            onClick={onClose}
          >
            {t('common.cancel')}
          </Button>
          <Button
            variant={isEditMode ? 'primary' : 'success'}
            size="md"
            onClick={handleSubmit}
            loading={isSaving}
            disabled={!isKeyTrusted && inspection?.status !== 'known'}
          >
            {isEditMode ? t('servers.addModal.saveChanges') : t('servers.addModal.connectAndSave')}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
};
