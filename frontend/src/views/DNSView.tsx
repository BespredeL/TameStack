import React, { useEffect, useState } from 'react';
import { Globe2, Plus, Trash2, RefreshCw, Download, Save, AlertCircle } from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import { useServer } from '../context/ServerContext';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { getErrorMessage } from '../utils/errors';
import {
  GetDNSStatus,
  InstallBIND9,
  CreateDNSZone,
  GetDNSZone,
  SaveDNSZoneRecords,
  DeleteDNSZone,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Badge, Button, Input, Select, PageToolbar, EmptyState } from '../components/ui';
import { ModalShell } from '../components/common/ModalShell';

interface DNSRecordItem {
  name: string;
  ttl: number;
  type: string;
  priority: number;
  value: string;
}

interface DNSViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const DNSView: React.FC<DNSViewProps> = ({
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

  const [selectedDomain, setSelectedDomain] = useState<string | null>(null);
  const [records, setRecords] = useState<DNSRecordItem[]>([]);

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newDomain, setNewDomain] = useState('');
  const [newServerIP, setNewServerIP] = useState('');

  const loadStatus = async () => {
    if (!selectedServerId) return;
    setLoading(true);
    setError(null);
    try {
      const res = await GetDNSStatus(selectedServerId);
      setStatus(res);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const loadZone = async (domain: string) => {
    if (!selectedServerId) return;
    setSelectedDomain(domain);
    setError(null);
    try {
      const detail = await GetDNSZone(selectedServerId, domain);
      setRecords((detail?.records || []) as DNSRecordItem[]);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  useEffect(() => {
    setSelectedDomain(null);
    setRecords([]);
    loadStatus();
  }, [selectedServerId]);

  useEffect(() => {
    const unsub = EventsOn('task:finished', (task: TaskRecord) => {
      if (task && task.server_id === selectedServerId && task.operation_type?.startsWith('dns_')) {
        loadStatus();
        if (selectedDomain) loadZone(selectedDomain);
      }
    });
    return () => {
      if (typeof unsub === 'function') unsub();
    };
  }, [selectedServerId, selectedDomain]);

  if (servers.length === 0) {
    return (
      <EmptyState
        icon={<Globe2 className="w-8 h-8 text-indigo-400" />}
        title={isEN ? 'DNS Server (BIND9)' : 'DNS-сервер (BIND9)'}
        description={isEN ? 'Add a server to manage local BIND9 DNS zones and records.' : 'Добавьте сервер для управления локальными зонами BIND9 и DNS-записями.'}
        action={<Button variant="primary" onClick={onOpenAddServer}>{isEN ? 'Add Server' : 'Добавить сервер'}</Button>}
      />
    );
  }

  const handleInstall = async () => {
    if (!selectedServerId) return;
    const ok = await confirm(
      isEN
        ? 'Install and start BIND9 DNS server on this host?'
        : 'Установить и запустить DNS-сервер BIND9 на этом сервере?'
    );
    if (!ok) return;
    try {
      const task = await InstallBIND9(selectedServerId);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleCreateZone = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !newDomain.trim()) return;
    try {
      const task = await CreateDNSZone(selectedServerId, newDomain.trim(), newServerIP.trim());
      setIsCreateOpen(false);
      setNewDomain('');
      setNewServerIP('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleDeleteZone = async (domain: string) => {
    if (!selectedServerId) return;
    const ok = await confirm(isEN ? `Delete DNS zone ${domain}?` : `Удалить DNS-зону ${domain}?`, { danger: true });
    if (!ok) return;
    try {
      const task = await DeleteDNSZone(selectedServerId, domain);
      if (selectedDomain === domain) {
        setSelectedDomain(null);
        setRecords([]);
      }
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleSaveRecords = async () => {
    if (!selectedServerId || !selectedDomain) return;
    const ok = await confirm(
      isEN
        ? `Save zone "${selectedDomain}" and reload BIND9?`
        : `Сохранить записи зоны «${selectedDomain}» и перезагрузить BIND9?`
    );
    if (!ok) return;
    try {
      const task = await SaveDNSZoneRecords(selectedServerId, selectedDomain, records);
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
            <Badge variant={status?.active ? 'success' : status?.installed ? 'warning' : 'neutral'} size="sm">
              BIND9: {status?.active ? 'active' : status?.installed ? 'inactive' : 'not installed'}
            </Badge>
            <Button
              variant="secondary"
              size="sm"
              onClick={loadStatus}
              disabled={loading}
              title={isEN ? 'Refresh DNS zones and status' : 'Обновить список зон и статус BIND9'}
              aria-label={isEN ? 'Refresh DNS zones and status' : 'Обновить список зон и статус BIND9'}
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            {!status?.installed ? (
              <Button variant="primary" size="sm" onClick={handleInstall}>
                <Download className="w-3.5 h-3.5 mr-1.5" />
                {isEN ? 'Install BIND9' : 'Установить BIND9'}
              </Button>
            ) : (
              <Button variant="primary" size="sm" onClick={() => setIsCreateOpen(true)}>
                <Plus className="w-3.5 h-3.5 mr-1.5" />
                {isEN ? 'New Zone' : 'Новая зона'}
              </Button>
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

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="bg-slate-900/70 border border-slate-800 rounded-2xl p-4 space-y-3">
          <h3 className="text-sm font-semibold text-slate-100">{isEN ? 'DNS Zones' : 'DNS-зоны'}</h3>
          {(!status?.zones || status.zones.length === 0) ? (
            <p className="text-xs text-slate-500">{isEN ? 'No zones configured.' : 'Зоны не созданы.'}</p>
          ) : (
            <div className="space-y-1.5">
              {status.zones.map((z: any) => (
                <div
                  key={z.domain}
                  className={`flex items-center justify-between px-3 py-2 rounded-xl border text-xs cursor-pointer ${
                    selectedDomain === z.domain
                      ? 'bg-indigo-500/15 border-indigo-500/40 text-indigo-200'
                      : 'bg-slate-950/60 border-slate-800 text-slate-300 hover:bg-slate-800/50'
                  }`}
                  onClick={() => loadZone(z.domain)}
                >
                  <span className="font-medium truncate">{z.domain}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteZone(z.domain);
                    }}
                    title={isEN ? 'Delete DNS zone' : 'Удалить DNS-зону'}
                    aria-label={isEN ? 'Delete DNS zone' : 'Удалить DNS-зону'}
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="lg:col-span-2 bg-slate-900/70 border border-slate-800 rounded-2xl p-4 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-100">
              {selectedDomain ? `${isEN ? 'Zone Records:' : 'Записи зоны:'} ${selectedDomain}` : (isEN ? 'Select a zone' : 'Выберите зону')}
            </h3>
            {selectedDomain && (
              <div className="flex items-center gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setRecords([...records, { name: '@', ttl: 3600, type: 'A', priority: 0, value: '' }])}
                >
                  <Plus className="w-3.5 h-3.5 mr-1" />
                  {isEN ? 'Add Record' : 'Запись'}
                </Button>
                <Button variant="primary" size="sm" onClick={handleSaveRecords}>
                  <Save className="w-3.5 h-3.5 mr-1.5" />
                  {isEN ? 'Save Zone' : 'Сохранить'}
                </Button>
              </div>
            )}
          </div>

          {selectedDomain && (
            <div className="space-y-2">
              {records.map((rec, idx) => (
                <div key={idx} className="grid grid-cols-12 gap-2 items-center text-xs">
                  <div className="col-span-3">
                    <Input value={rec.name} onChange={(e) => {
                      const next = [...records];
                      next[idx] = { ...rec, name: e.target.value };
                      setRecords(next);
                    }} placeholder="@" className="font-mono" />
                  </div>
                  <div className="col-span-2">
                    <Select sizeVariant="sm" value={rec.type} onChange={(e) => {
                      const next = [...records];
                      next[idx] = { ...rec, type: e.target.value };
                      setRecords(next);
                    }}>
                      {['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA'].map((t) => (
                        <option key={t} value={t}>{t}</option>
                      ))}
                    </Select>
                  </div>
                  <div className="col-span-2">
                    <Input type="number" value={rec.type === 'MX' ? rec.priority : rec.ttl} onChange={(e) => {
                      const next = [...records];
                      const val = Number(e.target.value) || 0;
                      next[idx] = rec.type === 'MX' ? { ...rec, priority: val } : { ...rec, ttl: val };
                      setRecords(next);
                    }} className="font-mono" title={rec.type === 'MX' ? 'Priority' : 'TTL'} />
                  </div>
                  <div className="col-span-4">
                    <Input value={rec.value} onChange={(e) => {
                      const next = [...records];
                      next[idx] = { ...rec, value: e.target.value };
                      setRecords(next);
                    }} placeholder="1.2.3.4" className="font-mono" />
                  </div>
                  <div className="col-span-1 text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setRecords(records.filter((_, i) => i !== idx))}
                      title={isEN ? 'Remove record' : 'Удалить запись'}
                      aria-label={isEN ? 'Remove record' : 'Удалить запись'}
                    >
                      <Trash2 className="w-3.5 h-3.5 text-rose-400" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {isCreateOpen && (
        <ModalShell onClose={() => setIsCreateOpen(false)} title={isEN ? 'Create DNS Zone' : 'Создать DNS-зону'} icon={<Globe2 className="w-5 h-5" />}>
          <form onSubmit={handleCreateZone}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Domain' : 'Домен'}</label>
                <Input value={newDomain} onChange={(e) => setNewDomain(e.target.value)} placeholder="example.com" required />
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Server IP (optional)' : 'IP-адрес сервера (опционально)'}</label>
                <Input value={newServerIP} onChange={(e) => setNewServerIP(e.target.value)} placeholder="1.2.3.4" />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsCreateOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Create Zone' : 'Создать'}</Button>
            </div>
          </form>
        </ModalShell>
      )}
    </div>
  );
};
