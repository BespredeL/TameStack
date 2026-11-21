import React from 'react';
import { Button } from './Button';
import { Inbox } from 'lucide-react';

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title?: string;
  description?: string;
  action?:
    | React.ReactNode
    | {
        label: string;
        onClick: () => void;
        icon?: React.ReactNode;
        variant?: 'primary' | 'secondary';
      };
  className?: string;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title = 'Нет данных',
  description,
  action,
  className = '',
}) => {
  return (
    <div
      className={`flex flex-col items-center justify-center p-8 text-center bg-surface-850/60 border border-slate-800/80 rounded-2xl ${className}`}
    >
      <div className="w-12 h-12 rounded-2xl bg-slate-900/80 border border-slate-800 flex items-center justify-center text-slate-400 mb-3 shadow-inner">
        {icon || <Inbox className="w-6 h-6 text-slate-500" />}
      </div>
      <h4 className="text-sm font-bold text-white mb-1 tracking-tight">{title}</h4>
      {description && <p className="text-xs text-slate-400 max-w-sm mb-4 leading-relaxed">{description}</p>}
      {action && (
        React.isValidElement(action) ? (
          action
        ) : (
          <Button
            variant={(action as any).variant || 'primary'}
            size="sm"
            icon={(action as any).icon}
            onClick={(action as any).onClick}
          >
            {(action as any).label}
          </Button>
        )
      )}
    </div>
  );
};
