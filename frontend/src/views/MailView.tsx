import React, { useEffect, useState } from 'react';
import {
  Mail,
  Plus,
  Trash2,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  Download,
  Copy,
  Forward,
  Globe,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import { useServer } from '../context/ServerContext';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { getErrorMessage } from '../utils/errors';
import {
  GetMailStatus,
  InstallMailStack,
  AddMailDomain,
  CreateMailbox,
  DeleteMailbox,
  SaveMailAlias,
  DeleteMailAlias,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Badge, Button, Input, PageToolbar, EmptyState } from '../components/ui';
import { ModalShell } from '../components/common/ModalShell';

interface MailViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const MailView: React.FC<MailViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const { selectedServerId, setSelectedServerId } = useServer();
  const { language } = useTranslation();
  const confirm = useConfirm();
  const isEN = language === 'en';

  const [status, setStatus] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [isInstallOpen, setIsInstallOpen] = useState(false);
  const [mailHostname, setMailHostname] = useState('');

  const [isDomainOpen, setIsDomainOpen] = useState(false);
  const [newDomain, setNewDomain] = useState('');

  const [isMailboxOpen, setIsMailboxOpen] = useState(false);
  const [newEmail, setNewEmail] = useState('');
  const [newPass, setNewPass] = useState('');

  const [isAliasOpen, setIsAliasOpen] = useState(false);
  const [aliasSrc, setAliasSrc] = useState('');
  const [aliasDst, setAliasDst] = useState('');

  const loadStatus = async () => {
    if (!selectedServerId) return;
    setLoading(true);
    setError(null);
    try {
      const res = await GetMailStatus(selectedServerId);
      setStatus(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadStatus();
  }, [selectedServerId]);

  useEffect(() => {
    const unsub = EventsOn('task:finished', (task: TaskRecord) => {
      if (task && task.server_id === selectedServerId && task.operation_type?.startsWith('mail_')) {
        loadStatus();
      }
    });
    return () => {
      if (typeof unsub === 'function') unsub();
    };
  }, [selectedServerId]);

  if (servers.length === 0) {
    return (
      <EmptyState
        icon={<Mail className="w-8 h-8 text-indigo-400" />}
        title={isEN ? 'Mail Server' : 'Почтовый сервер'}
        description={isEN ? 'Add a server to manage Postfix, Dovecot, OpenDKIM, mailboxes, and aliases.' : 'Добавьте сервер для управления Postfix, Dovecot, OpenDKIM, почтовыми ящиками и алиасами.'}
        action={<Button variant="primary" onClick={onOpenAddServer}>{isEN ? 'Add Server' : 'Добавить сервер'}</Button>}
      />
    );
  }

  const stackReady = status?.postfix_installed && status?.dovecot_installed;

  const handleInstallStack = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId) return;
    const ok = await confirm(
      isEN
        ? 'Install and configure Postfix, Dovecot, and OpenDKIM on this server?'
        : 'Установить и настроить Postfix, Dovecot и OpenDKIM на этом сервере?'
    );
    if (!ok) return;
    try {
      const task = await InstallMailStack(selectedServerId, mailHostname.trim());
      setIsInstallOpen(false);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleAddDomain = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !newDomain.trim()) return;
    try {
      const task = await AddMailDomain(selectedServerId, newDomain.trim());
      setIsDomainOpen(false);
      setNewDomain('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleCreateMailbox = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId) return;
    try {
      const task = await CreateMailbox(selectedServerId, newEmail.trim(), newPass, 0);
      setIsMailboxOpen(false);
      setNewEmail('');
      setNewPass('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleDeleteMailbox = async (email: string) => {
    if (!selectedServerId) return;
    const ok = await confirm(isEN ? `Delete mailbox ${email}?` : `Удалить почтовый ящик ${email}?`, { danger: true });
    if (!ok) return;
    try {
      const task = await DeleteMailbox(selectedServerId, email);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleSaveAlias = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId) return;
    try {
      const task = await SaveMailAlias(selectedServerId, aliasSrc.trim(), aliasDst.trim());
      setIsAliasOpen(false);
      setAliasSrc('');
      setAliasDst('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleDeleteAlias = async (source: string) => {
    if (!selectedServerId) return;
    const ok = await confirm(isEN ? `Delete alias ${source}?` : `Удалить алиас ${source}?`, { danger: true });
    if (!ok) return;
    try {
      const task = await DeleteMailAlias(selectedServerId, source);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  return (
    <div className="page space-y-6">
      <PageToolbar
        servers={servers}
        selectedServerId={selectedServerId}
        onSelectServer={setSelectedServerId}
        actions={
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={loadStatus}
              disabled={loading}
              title={isEN ? 'Refresh mail server status' : 'Обновить статус почтового сервера'}
              aria-label={isEN ? 'Refresh mail server status' : 'Обновить статус почтового сервера'}
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            {!stackReady && (
              <Button variant="primary" size="sm" onClick={() => setIsInstallOpen(true)}>
                <Download className="w-3.5 h-3.5 mr-1.5" />
                {isEN ? 'Install Mail Stack' : 'Установить почтовый стек'}
              </Button>
            )}
            {stackReady && (
              <>
                <Button variant="secondary" size="sm" onClick={() => setIsDomainOpen(true)}>
                  <Globe className="w-3.5 h-3.5 mr-1.5" />
                  {isEN ? 'Add Domain' : 'Домен'}
                </Button>
                <Button variant="primary" size="sm" onClick={() => setIsMailboxOpen(true)}>
                  <Plus className="w-3.5 h-3.5 mr-1.5" />
                  {isEN ? 'Create Mailbox' : 'Ящик'}
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setIsAliasOpen(true)}>
                  <Forward className="w-3.5 h-3.5 mr-1.5" />
                  {isEN ? 'Add Alias' : 'Алиас'}
                </Button>
              </>
            )}
          </div>
        }
      />

      {error && (
        <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Services Status */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {[
          { name: 'Postfix (SMTP)', installed: status?.postfix_installed, active: status?.postfix_active },
          { name: 'Dovecot (IMAP / POP3 / LMTP)', installed: status?.dovecot_installed, active: status?.dovecot_active },
          { name: 'OpenDKIM (Signing)', installed: status?.opendkim_installed, active: status?.opendkim_active },
        ].map((svc) => (
          <div key={svc.name} className="p-4 rounded-2xl bg-slate-900/70 border border-slate-800 flex items-center justify-between">
            <div>
              <div className="text-xs font-medium text-slate-200">{svc.name}</div>
              <div className="text-[11px] text-slate-500 mt-0.5">
                {svc.installed ? (isEN ? 'Installed' : 'Установлен') : (isEN ? 'Not installed' : 'Не установлен')}
              </div>
            </div>
            <Badge variant={svc.active ? 'success' : svc.installed ? 'warning' : 'neutral'} size="sm">
              {svc.active ? 'active' : svc.installed ? 'inactive' : 'missing'}
            </Badge>
          </div>
        ))}
      </div>

      {/* Domains & DKIM/SPF/DMARC */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-4">
        <h3 className="text-sm font-semibold text-slate-100">
          {isEN ? 'Mail Domains & DNS Records (DKIM / SPF / DMARC)' : 'Почтовые домены и DNS-записи (DKIM / SPF / DMARC)'}
        </h3>
        {(!status?.domains || status.domains.length === 0) ? (
          <p className="text-xs text-slate-500">{isEN ? 'No mail domains configured yet.' : 'Почтовые домены ещё не добавлены.'}</p>
        ) : (
          <div className="space-y-3">
            {status.domains.map((d: any) => (
              <div key={d.domain} className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 space-y-2 text-xs">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-indigo-300">{d.domain}</span>
                  <Badge variant="success" size="sm" className="inline-flex items-center gap-1">
                    <CheckCircle2 className="w-3 h-3" /> DKIM ({d.dkim_selector})
                  </Badge>
                </div>
                <div className="grid grid-cols-1 gap-1.5 font-mono text-[11px] text-slate-300">
                  <div className="flex items-center justify-between gap-2 bg-slate-900 px-2.5 py-1.5 rounded-lg">
                    <span className="truncate">SPF (TXT @): {d.spf_record}</span>
                    <button type="button" onClick={() => navigator.clipboard.writeText(d.spf_record)} className="text-slate-400 hover:text-white" title={isEN ? 'Copy SPF record' : 'Скопировать SPF-запись'} aria-label={isEN ? 'Copy SPF record' : 'Скопировать SPF-запись'}><Copy className="w-3.5 h-3.5" /></button>
                  </div>
                  <div className="flex items-center justify-between gap-2 bg-slate-900 px-2.5 py-1.5 rounded-lg">
                    <span className="truncate">DMARC (TXT _dmarc): {d.dmarc_record}</span>
                    <button type="button" onClick={() => navigator.clipboard.writeText(d.dmarc_record)} className="text-slate-400 hover:text-white" title={isEN ? 'Copy DMARC record' : 'Скопировать DMARC-запись'} aria-label={isEN ? 'Copy DMARC record' : 'Скопировать DMARC-запись'}><Copy className="w-3.5 h-3.5" /></button>
                  </div>
                  {d.dkim_record && (
                    <div className="flex items-start justify-between gap-2 bg-slate-900 px-2.5 py-1.5 rounded-lg">
                      <span className="break-all">DKIM (mail._domainkey): {d.dkim_record}</span>
                      <button type="button" onClick={() => navigator.clipboard.writeText(d.dkim_record)} className="text-slate-400 hover:text-white shrink-0" title={isEN ? 'Copy DKIM record' : 'Скопировать DKIM-запись'} aria-label={isEN ? 'Copy DKIM record' : 'Скопировать DKIM-запись'}><Copy className="w-3.5 h-3.5" /></button>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Mailboxes & Aliases */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-3">
          <h3 className="text-sm font-semibold text-slate-100">{isEN ? 'Mailboxes' : 'Почтовые ящики'}</h3>
          {(!status?.mailboxes || status.mailboxes.length === 0) ? (
            <p className="text-xs text-slate-500">{isEN ? 'No mailboxes created.' : 'Почтовые ящики не созданы.'}</p>
          ) : (
            <div className="divide-y divide-slate-800">
              {status.mailboxes.map((m: any) => (
                <div key={m.email} className="py-2.5 flex items-center justify-between text-xs">
                  <div>
                    <div className="font-medium text-slate-100">{m.email}</div>
                    <div className="text-[11px] font-mono text-slate-500">{m.maildir}</div>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleDeleteMailbox(m.email)}
                    title={isEN ? 'Delete mailbox' : 'Удалить почтовый ящик'}
                    aria-label={isEN ? 'Delete mailbox' : 'Удалить почтовый ящик'}
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-5 space-y-3">
          <h3 className="text-sm font-semibold text-slate-100">{isEN ? 'Forwarding Aliases' : 'Алиасы (переадресация)'}</h3>
          {(!status?.aliases || status.aliases.length === 0) ? (
            <p className="text-xs text-slate-500">{isEN ? 'No aliases configured.' : 'Алиасы не настроены.'}</p>
          ) : (
            <div className="divide-y divide-slate-800">
              {status.aliases.map((a: any) => (
                <div key={a.source} className="py-2.5 flex items-center justify-between text-xs">
                  <div className="font-mono text-slate-200">
                    {a.source} <span className="text-slate-500">→</span> {a.destination}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleDeleteAlias(a.source)}
                    title={isEN ? 'Delete alias' : 'Удалить алиас'}
                    aria-label={isEN ? 'Delete alias' : 'Удалить алиас'}
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {isInstallOpen && (
        <ModalShell onClose={() => setIsInstallOpen(false)} title={isEN ? 'Install Mail Stack' : 'Установка почтового сервера'} icon={<Download className="w-5 h-5" />}>
          <form onSubmit={handleInstallStack}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Mail Hostname (FQDN, optional)' : 'Hostname почтового сервера (FQDN, опционально)'}</label>
                <Input value={mailHostname} onChange={(e) => setMailHostname(e.target.value)} placeholder="mail.example.com" />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsInstallOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Install' : 'Установить'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {isDomainOpen && (
        <ModalShell onClose={() => setIsDomainOpen(false)} title={isEN ? 'Add Mail Domain' : 'Добавить почтовый домен'} icon={<Globe className="w-5 h-5" />}>
          <form onSubmit={handleAddDomain}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Domain' : 'Домен'}</label>
                <Input value={newDomain} onChange={(e) => setNewDomain(e.target.value)} placeholder="example.com" required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsDomainOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Add Domain' : 'Добавить'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {isMailboxOpen && (
        <ModalShell onClose={() => setIsMailboxOpen(false)} title={isEN ? 'Create Mailbox' : 'Создать почтовый ящик'} icon={<Mail className="w-5 h-5" />}>
          <form onSubmit={handleCreateMailbox}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">Email</label>
                <Input type="email" value={newEmail} onChange={(e) => setNewEmail(e.target.value)} placeholder="info@example.com" required />
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Password (min 8 chars)' : 'Пароль (мин. 8 символов)'}</label>
                <Input type="password" value={newPass} onChange={(e) => setNewPass(e.target.value)} minLength={8} required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsMailboxOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Create' : 'Создать'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {isAliasOpen && (
        <ModalShell onClose={() => setIsAliasOpen(false)} title={isEN ? 'Add Mail Alias' : 'Добавить алиас'} icon={<Forward className="w-5 h-5" />}>
          <form onSubmit={handleSaveAlias}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Source Email' : 'Исходный адрес'}</label>
                <Input type="email" value={aliasSrc} onChange={(e) => setAliasSrc(e.target.value)} placeholder="support@example.com" required />
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Destination Email' : 'Куда пересылать'}</label>
                <Input type="email" value={aliasDst} onChange={(e) => setAliasDst(e.target.value)} placeholder="admin@example.com" required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsAliasOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Save' : 'Сохранить'}</Button>
            </div>
          </form>
        </ModalShell>
      )}
    </div>
  );
};
