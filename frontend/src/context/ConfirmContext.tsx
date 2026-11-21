import React, { createContext, useCallback, useContext, useRef, useState } from 'react';
import { AlertTriangle, HelpCircle } from 'lucide-react';
import { ModalShell } from '../components/common/ModalShell';
import { Button } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';

interface ConfirmOptions {
  title?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

type ConfirmRequest = ConfirmOptions & { message: string };
type ConfirmFn = (message: string, options?: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

// Drop-in replacement for window.confirm that renders the app's modal styling:
// const confirm = useConfirm(); if (!(await confirm(message))) return;
export const useConfirm = (): ConfirmFn => {
  const fn = useContext(ConfirmContext);
  if (!fn) throw new Error('useConfirm must be used within ConfirmProvider');
  return fn;
};

interface PendingRequest extends ConfirmRequest {
  resolve: (confirmed: boolean) => void;
}

export const ConfirmProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useTranslation();
  const [pending, setPending] = useState<PendingRequest | null>(null);
  const pendingRef = useRef<PendingRequest | null>(null);

  const settle = useCallback((confirmed: boolean) => {
    const current = pendingRef.current;
    pendingRef.current = null;
    setPending(null);
    current?.resolve(confirmed);
  }, []);

  const confirm = useCallback<ConfirmFn>((message, options) => {
    // Only one dialog at a time; a concurrent request resolves as declined so
    // callers can never deadlock waiting for a hidden dialog.
    if (pendingRef.current) return Promise.resolve(false);
    return new Promise<boolean>((resolve) => {
      const request: PendingRequest = { message, ...options, resolve };
      pendingRef.current = request;
      setPending(request);
    });
  }, []);

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {pending && (
        <ModalShell
          onClose={() => settle(false)}
          size="sm"
          zIndex="z-[70]"
          title={pending.title ?? t('common.warning')}
          icon={pending.danger ? <AlertTriangle className="w-5 h-5" /> : <HelpCircle className="w-5 h-5" />}
          iconClassName={
            pending.danger
              ? 'w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0'
              : undefined
          }
        >
          <div className="px-6 py-5">
            <p className="text-sm text-slate-300 whitespace-pre-line leading-relaxed">{pending.message}</p>
            <div className="pt-4 mt-5 flex justify-end gap-3 border-t border-slate-800">
              <Button
                type="button"
                variant="ghost"
                size="md"
                onClick={() => settle(false)}
              >
                {pending.cancelLabel ?? t('common.cancel')}
              </Button>
              <Button
                type="button"
                autoFocus
                variant={pending.danger ? 'danger' : 'primary'}
                size="md"
                onClick={() => settle(true)}
              >
                {pending.confirmLabel ?? t('common.confirm')}
              </Button>
            </div>
          </div>
        </ModalShell>
      )}
    </ConfirmContext.Provider>
  );
};
