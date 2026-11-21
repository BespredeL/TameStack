import React, { createContext, useCallback, useContext, useRef, useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { ModalShell } from '../components/common/ModalShell';
import { Button } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';

interface AlertOptions {
  title?: string;
}

type QueuedAlert = AlertOptions & { message: string };
type AlertFn = (message: string, options?: AlertOptions) => void;

const AlertContext = createContext<AlertFn | null>(null);

// Drop-in replacement for window.alert that renders the app's modal styling:
// const alert = useAlert(); alert(message);
export const useAlert = (): AlertFn => {
  const fn = useContext(AlertContext);
  if (!fn) throw new Error('useAlert must be used within AlertProvider');
  return fn;
};

export const AlertProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useTranslation();
  const [queue, setQueue] = useState<QueuedAlert[]>([]);
  const queueRef = useRef<QueuedAlert[]>([]);

  const dismiss = useCallback(() => {
    queueRef.current = queueRef.current.slice(1);
    setQueue(queueRef.current);
  }, []);

  // Alerts raised while a dialog is open are queued, so no error message is lost.
  const alert = useCallback<AlertFn>((message, options) => {
    queueRef.current = [...queueRef.current, { message, ...options }];
    setQueue(queueRef.current);
  }, []);

  const current = queue[0];

  return (
    <AlertContext.Provider value={alert}>
      {children}
      {current && (
        <ModalShell
          onClose={dismiss}
          size="sm"
          zIndex="z-[70]"
          title={current.title ?? t('common.error')}
          icon={<AlertTriangle className="w-5 h-5" />}
          iconClassName="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0"
        >
          <div className="px-6 py-5">
            <p className="text-sm text-slate-300 whitespace-pre-line leading-relaxed">{current.message}</p>
            <div className="pt-4 mt-5 flex justify-end border-t border-slate-800">
              <Button
                type="button"
                autoFocus
                variant="primary"
                size="md"
                onClick={dismiss}
              >
                {t('common.ok')}
              </Button>
            </div>
          </div>
        </ModalShell>
      )}
    </AlertContext.Provider>
  );
};
