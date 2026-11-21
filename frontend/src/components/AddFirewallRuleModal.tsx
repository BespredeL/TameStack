import React, { useState, useEffect } from 'react';
import { Shield, AlertTriangle, CheckCircle, ShieldAlert } from 'lucide-react';
import { ServerRecord } from '../types';
import { AddFirewallRule, UpdateFirewallRule } from '../../wailsjs/go/main/App';
import { useTranslation } from '../i18n/LanguageContext';
import { ModalShell } from './common/ModalShell';
import { Button, Input, Select } from './ui';

interface AddFirewallRuleModalProps {
  isOpen: boolean;
  onClose: () => void;
  server: ServerRecord;
  initialRule?: any | null;
  onRuleAdded: (task: any) => void;
  onRuleUpdated?: (task: any) => void;
}

export const AddFirewallRuleModal: React.FC<AddFirewallRuleModalProps> = ({
  isOpen,
  onClose,
  server,
  initialRule,
  onRuleAdded,
  onRuleUpdated,
}) => {
  const { t } = useTranslation();
  const [port, setPort] = useState('');
  const [protocol, setProtocol] = useState<'tcp' | 'udp' | 'any'>('tcp');
  const [action, setAction] = useState<'ALLOW' | 'DENY' | 'REJECT' | 'LIMIT'>('ALLOW');
  const [fromIP, setFromIP] = useState('any');
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isEditing = Boolean(initialRule);

  useEffect(() => {
    if (initialRule) {
      setPort(initialRule.port || '');
      const proto = (initialRule.protocol || 'tcp').toLowerCase();
      setProtocol(proto === 'udp' || proto === 'any' ? proto : 'tcp');
      const act = (initialRule.action || 'ALLOW').toUpperCase();
      setAction(act === 'DENY' || act === 'REJECT' || act === 'LIMIT' ? act : 'ALLOW');
      setFromIP(initialRule.from_ip || 'any');
      setComment(initialRule.comment || '');
    } else {
      setPort('');
      setProtocol('tcp');
      setAction('ALLOW');
      setFromIP('any');
      setComment('');
    }
    setError(null);
  }, [isOpen, initialRule]);

  if (!isOpen) return null;

  const isDangerousSSHBlock =
    (action === 'DENY' || action === 'REJECT') &&
    (port === String(server.port) || (server.port === 0 && port === '22')) &&
    (fromIP.toLowerCase() === 'any' || fromIP.trim() === '');

  const quickPresets = [
    { label: 'HTTP (80)', port: '80', proto: 'tcp' as const },
    { label: 'HTTPS (443)', port: '443', proto: 'tcp' as const },
    { label: 'SSH (22)', port: '22', proto: 'tcp' as const },
    { label: 'MySQL (3306)', port: '3306', proto: 'tcp' as const },
    { label: 'Postgres (5432)', port: '5432', proto: 'tcp' as const },
    { label: 'Redis (6379)', port: '6379', proto: 'tcp' as const },
    { label: 'Node/Next (3000)', port: '3000', proto: 'tcp' as const },
  ];

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!port.trim()) {
      setError(t('security.firewallModal.portRequired'));
      return;
    }

    if (isDangerousSSHBlock) {
      setError(t('security.firewallModal.sshPortBlockedError', { port: server.port || 22 }));
      return;
    }

    try {
      setIsSubmitting(true);
      if (isEditing && initialRule) {
        const task = await UpdateFirewallRule({
          server_id: server.id,
          rule_num: initialRule.number || 0,
          old_port: initialRule.port || '',
          old_proto: initialRule.protocol || '',
          port: port.trim(),
          protocol,
          action,
          from_ip: fromIP.trim(),
          comment: comment.trim(),
        } as any);

        onRuleUpdated?.(task);
      } else {
        const task = await AddFirewallRule({
          server_id: server.id,
          port: port.trim(),
          protocol,
          action,
          from_ip: fromIP.trim(),
          comment: comment.trim(),
        } as any);

        onRuleAdded(task);
      }
      onClose();
    } catch (err: unknown) {
      setError(t('security.firewallModal.saveRuleError', { error: err?.toString() || '' }));
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ModalShell
      onClose={onClose}
      size="md"
      icon={<Shield className="w-5 h-5" />}
      iconClassName="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 shrink-0"
      title={isEditing ? t('security.firewallModal.titleEdit') : t('security.firewallModal.titleAdd')}
      subtitle={t('security.firewallModal.subtitle', { name: server.name, host: server.host })}
    >
      <form onSubmit={handleSubmit} className="p-6 space-y-4">
        {error && (
          <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-lg text-rose-400 text-xs flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {isDangerousSSHBlock && (
          <div className="p-3 bg-rose-500/15 border border-rose-500/40 rounded-xl text-xs text-rose-300 flex items-start gap-2.5 shadow-md">
            <ShieldAlert className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
            <div>
              <div className="font-bold text-rose-200">{t('security.firewallModal.lockoutWarningTitle')}</div>
              <p className="mt-0.5 text-rose-300/90 leading-relaxed">
                {t('security.firewallModal.lockoutWarningDesc', { port: server.port || 22 })}
              </p>
            </div>
          </div>
        )}

        {/* Quick Presets */}
        <div>
          <label className="block text-xs font-medium text-slate-400 mb-1.5">
            {t('security.firewallModal.quickPresetsLabel')}
          </label>
          <div className="flex flex-wrap gap-1.5">
            {quickPresets.map((preset) => (
              <button
                key={preset.label}
                type="button"
                onClick={() => {
                  setPort(preset.port);
                  setProtocol(preset.proto);
                }}
                className={`px-2.5 py-1 text-xs rounded-lg border transition cursor-pointer ${
                  port === preset.port
                    ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300 font-medium'
                    : 'bg-slate-900 border-slate-800 text-slate-300 hover:bg-slate-800'
                }`}
              >
                {preset.label}
              </button>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {/* Port */}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('security.firewallModal.portOrRangeLabel')} <span className="text-rose-400">*</span>
            </label>
            <Input
              type="text"
              value={port}
              onChange={(e) => setPort(e.target.value)}
              placeholder={t('security.firewallModal.portPlaceholder')}
              required
            />
          </div>

          {/* Protocol */}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('security.firewallModal.protocolLabel')}
            </label>
            <Select
              value={protocol}
              onChange={(e) => setProtocol(e.target.value as 'tcp' | 'udp' | 'any')}
            >
              <option value="tcp" className="bg-slate-900 text-white">TCP</option>
              <option value="udp" className="bg-slate-900 text-white">UDP</option>
              <option value="any" className="bg-slate-900 text-white">{t('security.firewallModal.protoAny')}</option>
            </Select>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {/* Action */}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('security.firewallModal.actionLabel')}
            </label>
            <Select
              value={action}
              onChange={(e) => setAction(e.target.value as 'ALLOW' | 'DENY' | 'REJECT' | 'LIMIT')}
            >
              <option value="ALLOW" className="bg-slate-900 text-emerald-400 font-semibold">{t('security.firewallModal.actionAllow')}</option>
              <option value="DENY" className="bg-slate-900 text-rose-400 font-semibold">{t('security.firewallModal.actionDeny')}</option>
              <option value="REJECT" className="bg-slate-900 text-rose-400 font-semibold">{t('security.firewallModal.actionReject')}</option>
              <option value="LIMIT" className="bg-slate-900 text-amber-400 font-semibold">{t('security.firewallModal.actionLimit')}</option>
            </Select>
          </div>

          {/* From IP */}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              {t('security.firewallModal.fromIpLabel')}
            </label>
            <Input
              type="text"
              value={fromIP}
              onChange={(e) => setFromIP(e.target.value)}
              placeholder={t('security.firewallModal.fromIpPlaceholder')}
            />
          </div>
        </div>

        {/* Comment */}
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            {t('security.firewallModal.commentLabel')}
          </label>
          <Input
            type="text"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder={t('security.firewallModal.commentPlaceholder')}
          />
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800 mt-6">
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={onClose}
            disabled={isSubmitting}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={isSubmitting || isDangerousSSHBlock}
            loading={isSubmitting}
            icon={<CheckCircle className="w-4 h-4" />}
            className="bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-900/20"
          >
            {isSubmitting
              ? (isEditing ? t('security.firewallModal.savingRule') : t('security.firewallModal.applyingRule'))
              : (isEditing ? t('security.firewallModal.saveRuleBtn') : t('security.firewallModal.createRuleBtn'))}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
};
