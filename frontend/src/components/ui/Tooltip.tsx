import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

export interface TooltipProps {
  content: string;
  side?: 'top' | 'bottom' | 'left' | 'right';
  children: React.ReactElement;
}

/**
 * Explicit Tooltip wrapper that sets data-tooltip attributes for GlobalTooltip.
 */
export const Tooltip: React.FC<TooltipProps> = ({ content, side, children }) => {
  return React.cloneElement(children, {
    'data-tooltip': content,
    ...(side ? { 'data-tooltip-side': side } : {}),
  });
};

interface ActiveTooltip {
  text: string;
  rect: DOMRect;
  side: 'top' | 'bottom' | 'left' | 'right' | 'auto';
}

const SHOW_DELAY_MS = 150;
const WARM_WINDOW_MS = 250;

/**
 * GlobalTooltip intercepts native HTML `title` attributes and `data-tooltip`
 * across the entire application, replacing browser default tooltips with a
 * consistent, readable, viewport-clamped floating tooltip.
 */
export const GlobalTooltip: React.FC = () => {
  const [active, setActive] = useState<ActiveTooltip | null>(null);
  const [coords, setCoords] = useState<{ top: number; left: number; ready: boolean }>({
    top: 0,
    left: 0,
    ready: false,
  });

  const tipRef = useRef<HTMLDivElement | null>(null);
  const activeElRef = useRef<HTMLElement | null>(null);
  const showTimerRef = useRef<number | null>(null);
  const lastClosedAtRef = useRef<number>(0);

  const clearTimer = () => {
    if (showTimerRef.current !== null) {
      window.clearTimeout(showTimerRef.current);
      showTimerRef.current = null;
    }
  };

  const hideTooltip = () => {
    clearTimer();
    if (activeElRef.current) {
      lastClosedAtRef.current = Date.now();
      activeElRef.current = null;
    }
    setActive(null);
    setCoords((c) => (c.ready ? { ...c, ready: false } : c));
  };

  useEffect(() => {
    const findTooltipTarget = (start: EventTarget | null): HTMLElement | null => {
      let node = start instanceof Element ? start : null;
      while (node) {
        if (node instanceof HTMLElement) {
          if (node.tagName === 'OPTION') return null;
          if (node.closest('[data-no-tooltip="true"]')) {
            if (node.hasAttribute('title')) {
              node.removeAttribute('title');
            }
            return null;
          }
          const nativeTitle = node.getAttribute('title');
          if (nativeTitle && nativeTitle.trim() !== '') {
            node.setAttribute('data-tooltip', nativeTitle);
            node.removeAttribute('title');
          }
          const tip = node.getAttribute('data-tooltip');
          if (tip && tip.trim() !== '') {
            return node;
          }
        }
        node = node.parentElement;
      }
      return null;
    };

    const normalizeText = (s: string) => s.replace(/\s+/g, ' ').trim().toLowerCase();

    const showForElement = (el: HTMLElement) => {
      const text = el.getAttribute('data-tooltip')?.trim();
      if (!text) return;
      // Skip redundant tooltips that merely repeat the element's own visible label.
      const visibleText = el.textContent?.trim() || '';
      if (visibleText !== '' && normalizeText(visibleText) === normalizeText(text)) {
        return;
      }
      const sideAttr = el.getAttribute('data-tooltip-side') as ActiveTooltip['side'] | null;
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) return;

      activeElRef.current = el;
      const isWarm = Date.now() - lastClosedAtRef.current < WARM_WINDOW_MS;
      clearTimer();

      const commit = () => {
        if (!el.isConnected) return;
        setActive({
          text,
          rect: el.getBoundingClientRect(),
          side: sideAttr || 'auto',
        });
      };

      if (isWarm) {
        commit();
      } else {
        showTimerRef.current = window.setTimeout(commit, SHOW_DELAY_MS);
      }
    };

    const onMouseOver = (e: MouseEvent) => {
      const target = findTooltipTarget(e.target);
      if (!target) {
        if (activeElRef.current) hideTooltip();
        return;
      }
      if (target === activeElRef.current) return;
      showForElement(target);
    };

    const onMouseOut = (e: MouseEvent) => {
      if (!activeElRef.current) return;
      const related = e.relatedTarget as Node | null;
      if (related && activeElRef.current.contains(related)) return;
      const nextTarget = findTooltipTarget(related);
      if (nextTarget === activeElRef.current) return;
      hideTooltip();
    };

    const onFocusIn = (e: FocusEvent) => {
      const target = findTooltipTarget(e.target);
      if (target) showForElement(target);
    };

    const onFocusOut = () => hideTooltip();
    const onMouseDown = () => hideTooltip();
    const onScroll = () => hideTooltip();
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') hideTooltip();
    };

    document.addEventListener('mouseover', onMouseOver, true);
    document.addEventListener('mouseout', onMouseOut, true);
    document.addEventListener('focusin', onFocusIn, true);
    document.addEventListener('focusout', onFocusOut, true);
    document.addEventListener('mousedown', onMouseDown, true);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('keydown', onKeyDown, true);

    return () => {
      clearTimer();
      document.removeEventListener('mouseover', onMouseOver, true);
      document.removeEventListener('mouseout', onMouseOut, true);
      document.removeEventListener('focusin', onFocusIn, true);
      document.removeEventListener('focusout', onFocusOut, true);
      document.removeEventListener('mousedown', onMouseDown, true);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('keydown', onKeyDown, true);
    };
  }, []);

  useLayoutEffect(() => {
    if (!active || !tipRef.current) return;
    const tip = tipRef.current;
    const tipW = tip.offsetWidth;
    const tipH = tip.offsetHeight;
    const { rect, side } = active;
    const pad = 8;
    const gap = 8;

    let top = 0;
    let left = 0;

    const placeRight =
      side === 'right' || (side === 'auto' && rect.left < 76 && rect.width <= 64);

    if (placeRight) {
      top = rect.top + rect.height / 2 - tipH / 2;
      left = rect.right + gap;
    } else if (side === 'left') {
      top = rect.top + rect.height / 2 - tipH / 2;
      left = rect.left - tipW - gap;
    } else if (side === 'bottom' || (side === 'auto' && rect.top - tipH - gap < pad)) {
      top = rect.bottom + gap;
      left = rect.left + rect.width / 2 - tipW / 2;
    } else {
      top = rect.top - tipH - gap;
      left = rect.left + rect.width / 2 - tipW / 2;
    }

    left = Math.max(pad, Math.min(left, window.innerWidth - tipW - pad));
    top = Math.max(pad, Math.min(top, window.innerHeight - tipH - pad));

    setCoords({ top, left, ready: true });
  }, [active]);

  if (!active || typeof document === 'undefined') return null;

  return createPortal(
    <div
      ref={tipRef}
      role="tooltip"
      style={{
        top: `${coords.top}px`,
        left: `${coords.left}px`,
        opacity: coords.ready ? 1 : 0,
      }}
      className="fixed z-[9999] pointer-events-none max-w-xs px-2.5 py-1.5 rounded-lg bg-slate-950/95 border border-slate-700/80 text-slate-100 text-xs font-medium leading-relaxed whitespace-pre-line shadow-2xl shadow-black/70 backdrop-blur-md transition-opacity duration-100"
    >
      {active.text}
    </div>,
    document.body
  );
};
