import React, { useRef, useState, useEffect, useCallback } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

export interface TabItem {
  id: string;
  label: React.ReactNode;
  icon?: React.ReactNode;
  badge?: React.ReactNode;
  title?: string;
}

export interface TabsProps {
  tabs: TabItem[];
  activeTab: string;
  onChange: (id: string) => void;
  variant?: 'pill' | 'line';
  size?: 'sm' | 'md';
  className?: string;
}

export const Tabs: React.FC<TabsProps> = ({
  tabs,
  activeTab,
  onChange,
  variant = 'pill',
  size = 'md',
  className = '',
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const checkScroll = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const hasOverflow = el.scrollWidth > el.clientWidth + 2;
    setCanScrollLeft(hasOverflow && el.scrollLeft > 4);
    setCanScrollRight(hasOverflow && el.scrollLeft < el.scrollWidth - el.clientWidth - 4);
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    checkScroll();

    const onWheel = (e: WheelEvent) => {
      if (e.deltaY !== 0 && el.scrollWidth > el.clientWidth) {
        e.preventDefault();
        el.scrollLeft += e.deltaY;
        checkScroll();
      }
    };

    el.addEventListener('wheel', onWheel, { passive: false });
    el.addEventListener('scroll', checkScroll, { passive: true });
    window.addEventListener('resize', checkScroll);

    return () => {
      el.removeEventListener('wheel', onWheel);
      el.removeEventListener('scroll', checkScroll);
      window.removeEventListener('resize', checkScroll);
    };
  }, [checkScroll, tabs]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const activeBtn = el.querySelector<HTMLElement>('[data-active="true"]');
    if (activeBtn) {
      activeBtn.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
    }
    checkScroll();
  }, [activeTab, checkScroll]);

  const scrollByAmount = (amount: number) => {
    if (containerRef.current) {
      containerRef.current.scrollBy({ left: amount, behavior: 'smooth' });
    }
  };

  const renderBadge = (badge: React.ReactNode, isActive: boolean) => {
    if (badge === null || badge === undefined || badge === false) return null;
    if (typeof badge === 'number' || typeof badge === 'string') {
      if (variant === 'line') {
        return (
          <span
            className={`px-1.5 py-0.5 rounded-full text-[10px] font-mono tabular-nums leading-none ${
              isActive
                ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30'
                : 'bg-slate-800 text-slate-400 border border-slate-700/50'
            }`}
          >
            {badge}
          </span>
        );
      }
      return (
        <span
          className={`px-1.5 py-0.5 rounded-full text-[10px] font-mono tabular-nums leading-none ${
            isActive ? 'bg-indigo-700/80 text-white' : 'bg-slate-800 text-slate-400'
          }`}
        >
          {badge}
        </span>
      );
    }
    return badge;
  };

  if (variant === 'line') {
    const pad = size === 'sm' ? 'px-2.5 py-2 text-xs' : 'px-3.5 py-2.5 text-xs';
    return (
      <div className={`relative flex items-center border-b border-slate-800 ${className}`}>
        {canScrollLeft && (
          <button
            type="button"
            onClick={() => scrollByAmount(-140)}
            className="absolute left-1 z-10 p-1 bg-surface-900/95 border border-slate-700/80 rounded-md text-slate-300 hover:text-white shadow-md transition cursor-pointer"
            aria-label="Scroll left"
          >
            <ChevronLeft className="w-3.5 h-3.5" />
          </button>
        )}

        <div
          ref={containerRef}
          className="flex items-center gap-1.5 overflow-x-auto no-scrollbar scroll-smooth w-full"
        >
          {tabs.map((tab) => {
            const isActive = tab.id === activeTab;
            return (
              <button
                key={tab.id}
                type="button"
                data-active={isActive ? 'true' : 'false'}
                title={tab.title}
                onClick={() => onChange(tab.id)}
                className={`flex items-center gap-1.5 ${pad} font-semibold border-b-2 transition-colors duration-150 ease-out whitespace-nowrap cursor-pointer shrink-0 outline-none focus:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500/50 ${
                  isActive
                    ? 'border-indigo-500 text-white'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-700'
                }`}
              >
                {tab.icon && <span className={isActive ? 'text-indigo-400' : 'text-slate-500'}>{tab.icon}</span>}
                <span>{tab.label}</span>
                {renderBadge(tab.badge, isActive)}
              </button>
            );
          })}
        </div>

        {canScrollRight && (
          <button
            type="button"
            onClick={() => scrollByAmount(140)}
            className="absolute right-1 z-10 p-1 bg-surface-900/95 border border-slate-700/80 rounded-md text-slate-300 hover:text-white shadow-md transition cursor-pointer"
            aria-label="Scroll right"
          >
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    );
  }

  const btnPad = size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-xs';
  return (
    <div className={`relative flex items-center ${className}`}>
      {canScrollLeft && (
        <button
          type="button"
          onClick={() => scrollByAmount(-140)}
          className="absolute left-1 z-10 p-1 bg-slate-900/95 border border-slate-700/80 rounded-md text-slate-300 hover:text-white shadow-md transition cursor-pointer"
          aria-label="Scroll left"
        >
          <ChevronLeft className="w-3.5 h-3.5" />
        </button>
      )}

      <div
        ref={containerRef}
        className="flex items-center gap-1.5 p-1 bg-slate-950/80 border border-slate-800 rounded-xl overflow-x-auto no-scrollbar scroll-smooth w-full"
      >
        {tabs.map((tab) => {
          const isActive = tab.id === activeTab;
          return (
            <button
              key={tab.id}
              type="button"
              data-active={isActive ? 'true' : 'false'}
              title={tab.title}
              onClick={() => onChange(tab.id)}
              className={`flex items-center gap-1.5 ${btnPad} rounded-lg font-semibold transition-colors duration-150 ease-out whitespace-nowrap cursor-pointer shrink-0 outline-none focus:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500/50 ${
                isActive
                  ? 'bg-indigo-600 text-white shadow-xs shadow-indigo-600/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              {tab.icon && <span className={isActive ? 'text-white' : 'text-slate-500'}>{tab.icon}</span>}
              <span>{tab.label}</span>
              {renderBadge(tab.badge, isActive)}
            </button>
          );
        })}
      </div>

      {canScrollRight && (
        <button
          type="button"
          onClick={() => scrollByAmount(140)}
          className="absolute right-1 z-10 p-1 bg-slate-900/95 border border-slate-700/80 rounded-md text-slate-300 hover:text-white shadow-md transition cursor-pointer"
          aria-label="Scroll right"
        >
          <ChevronRight className="w-3.5 h-3.5" />
        </button>
      )}
    </div>
  );
};
