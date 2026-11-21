import React, { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { useTranslation } from '../../i18n/LanguageContext';

export type ModalSize =
  | 'sm' | 'md' | 'lg' | 'xl'
  | '2xl' | '3xl' | '4xl' | '5xl' | '6xl' | '7xl' | 'full';

const sizeClasses: Record<ModalSize, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
  '4xl': 'max-w-4xl',
  '5xl': 'max-w-5xl',
  '6xl': 'max-w-6xl',
  '7xl': 'max-w-7xl',
  full: 'max-w-full',
};

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

// Tracks open shells so Esc and the focus trap only act on the topmost dialog.
const modalStack: string[] = [];

interface ModalShellProps {
  onClose: () => void;
  size?: ModalSize;
  title?: React.ReactNode;
  subtitle?: React.ReactNode;
  icon?: React.ReactNode;
  iconClassName?: string;
  headerExtra?: React.ReactNode;
  headerActions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  closeOnBackdrop?: boolean;
  closeOnEsc?: boolean;
  hideCloseButton?: boolean;
  zIndex?: string;
  label?: string;
}

export const ModalShell: React.FC<ModalShellProps> = ({
  onClose,
  size = 'lg',
  title,
  subtitle,
  icon,
  iconClassName,
  headerExtra,
  headerActions,
  children,
  className = '',
  closeOnBackdrop = false,
  closeOnEsc = true,
  hideCloseButton = false,
  zIndex = 'z-50',
  label,
}) => {
  const { t } = useTranslation();
  const id = useId();
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement>(null);

  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const closeOnEscRef = useRef(closeOnEsc);
  closeOnEscRef.current = closeOnEsc;

  useEffect(() => {
    modalStack.push(id);
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    if (panel && !panel.contains(document.activeElement)) {
      panel.focus();
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (modalStack[modalStack.length - 1] !== id) return;
      if (e.key === 'Escape' && closeOnEscRef.current) {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key === 'Tab' && panel) {
        const nodes = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
          (el) => el.getClientRects().length > 0
        );
        if (nodes.length === 0) {
          e.preventDefault();
          panel.focus();
          return;
        }
        const first = nodes[0];
        const last = nodes[nodes.length - 1];
        const active = document.activeElement;
        if (e.shiftKey && (active === first || active === panel)) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && active === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };

    document.addEventListener('keydown', onKeyDown, true);
    return () => {
      document.removeEventListener('keydown', onKeyDown, true);
      const idx = modalStack.indexOf(id);
      if (idx >= 0) modalStack.splice(idx, 1);
      if (previouslyFocused && document.contains(previouslyFocused)) {
        previouslyFocused.focus();
      }
    };
  }, [id]);

  const hasHeader = title != null || icon != null || headerActions != null || !hideCloseButton;

  const content = (
    <div
      className={`fixed inset-0 top-0 left-0 right-0 bottom-0 w-screen h-screen ${zIndex} flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm overscroll-contain`}
      onClick={(e) => {
        if (closeOnBackdrop && e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title != null ? titleId : undefined}
        aria-label={title == null ? label : undefined}
        tabIndex={-1}
        className={`bg-surface-900 border border-slate-800 rounded-2xl shadow-2xl w-full ${sizeClasses[size]} flex flex-col overflow-hidden overscroll-contain outline-none ${className}`}
      >
        {hasHeader && (
          <div className="flex items-center justify-between gap-3 px-6 py-4 border-b border-slate-800 bg-slate-950/40 shrink-0">
            <div className="flex items-center gap-3 min-w-0">
              {icon != null && (
                <div
                  className={
                    iconClassName ??
                    'w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0'
                  }
                >
                  {icon}
                </div>
              )}
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  {title != null && (
                    <h2 id={titleId} className="text-base font-bold text-white truncate">
                      {title}
                    </h2>
                  )}
                  {headerExtra}
                </div>
                {subtitle != null && (
                  <div className="text-xs text-slate-400 mt-0.5 truncate">{subtitle}</div>
                )}
              </div>
            </div>
            {(headerActions != null || !hideCloseButton) && (
              <div className="flex items-center gap-2 shrink-0">
                {headerActions}
                {!hideCloseButton && (
                  <button
                    type="button"
                    onClick={onClose}
                    aria-label={t('common.close')}
                    className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
                  >
                    <X className="w-5 h-5" />
                  </button>
                )}
              </div>
            )}
          </div>
        )}
        {children}
      </div>
    </div>
  );

  if (typeof document !== 'undefined' && document.body) {
    return createPortal(content, document.body);
  }

  return content;
};