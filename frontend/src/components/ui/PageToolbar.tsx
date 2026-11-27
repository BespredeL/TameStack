import React from 'react';
import { ServerRecord } from '../../types';
import { ChevronDown } from 'lucide-react';
import { usePrivacy } from '../../context/PrivacyContext';

export interface PageToolbarProps {
  icon?: React.ReactNode;
  iconClassName?: string;
  servers?: ServerRecord[];
  selectedServerId?: string;
  onSelectServer?: (serverId: string) => void;
  serverLabel?: string;
  title?: React.ReactNode;
  subtitle?: React.ReactNode;
  search?: React.ReactNode;
  filters?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}

export const PageToolbar: React.FC<PageToolbarProps> = ({
  icon,
  iconClassName = 'w-10 h-10 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 flex items-center justify-center shrink-0',
  servers,
  selectedServerId,
  onSelectServer,
  serverLabel = 'Сервер',
  title,
  subtitle,
  search,
  filters,
  actions,
  className = '',
}) => {
  const { maskServerName, maskHost } = usePrivacy();
  return (
    <div
      className={`flex flex-col gap-3.5 xl:flex-row xl:items-center xl:justify-between bg-surface-850 p-4 rounded-2xl border border-slate-800 shadow-sm ${className}`}
    >
      {/* Left section: Icon + (Server selector or Title) */}
      <div className="flex flex-wrap items-center gap-3 min-w-0">
        {icon && <div className={`shrink-0 flex items-center justify-center ${iconClassName}`}>{icon}</div>}

        {servers && onSelectServer ? (
          <div>
            {serverLabel && (
              <span className="text-[10px] font-semibold text-slate-400 block uppercase tracking-wider mb-1">
                {serverLabel}
              </span>
            )}
            <div className="relative inline-flex items-center">
              <select
                value={selectedServerId}
                onChange={(e) => onSelectServer(e.target.value)}
                className="bg-slate-950/80 border border-slate-800 hover:border-slate-700 rounded-xl text-xs font-semibold text-white pl-3 pr-8 h-8 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30 cursor-pointer appearance-none max-w-full"
              >
                {servers.map((s) => (
                  <option key={s.id} value={s.id} className="bg-slate-900 text-white">
                    {maskServerName(s.name)} ({maskHost(s.host)})
                  </option>
                ))}
              </select>
              <ChevronDown className="w-3.5 h-3.5 text-slate-400 absolute right-2.5 pointer-events-none" />
            </div>
          </div>
        ) : (
          title && (
            <div>
              <div className="text-sm sm:text-base font-bold text-white tracking-tight">{title}</div>
              {subtitle && <div className="text-xs text-slate-400">{subtitle}</div>}
            </div>
          )
        )}

        {filters && <div className="flex items-center gap-2">{filters}</div>}
      </div>

      {/* Right section: Search & Actions */}
      {(search || actions) && (
        <div className="flex flex-wrap items-center gap-2.5 w-full xl:w-auto xl:justify-end">
          {search && <div className="flex-1 sm:flex-initial">{search}</div>}
          {actions && <div className="flex items-center gap-2 flex-wrap">{actions}</div>}
        </div>
      )}
    </div>
  );
};
