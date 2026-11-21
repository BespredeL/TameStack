import React from 'react';

export type BadgeVariant =
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'neutral'
  | 'indigo'
  | 'cyan'
  | 'purple'
  | 'primary'
  | 'brand';

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  dot?: boolean;
  pulse?: boolean;
  mono?: boolean;
  size?: 'xs' | 'sm';
}

const variantStyles: Record<BadgeVariant, { bg: string; dot: string }> = {
  success: {
    bg: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/25',
    dot: 'bg-emerald-400',
  },
  warning: {
    bg: 'bg-amber-500/10 text-amber-300 border-amber-500/25',
    dot: 'bg-amber-400',
  },
  danger: {
    bg: 'bg-rose-500/10 text-rose-300 border-rose-500/25',
    dot: 'bg-rose-400',
  },
  info: {
    bg: 'bg-sky-500/10 text-sky-300 border-sky-500/25',
    dot: 'bg-sky-400',
  },
  indigo: {
    bg: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30',
    dot: 'bg-indigo-400',
  },
  primary: {
    bg: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30',
    dot: 'bg-indigo-400',
  },
  brand: {
    bg: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30',
    dot: 'bg-indigo-400',
  },
  cyan: {
    bg: 'bg-cyan-500/10 text-cyan-300 border-cyan-500/25',
    dot: 'bg-cyan-400',
  },
  purple: {
    bg: 'bg-purple-500/10 text-purple-300 border-purple-500/25',
    dot: 'bg-purple-400',
  },
  neutral: {
    bg: 'bg-slate-800 text-slate-400 border-slate-700/60',
    dot: 'bg-slate-400',
  },
};

export const Badge: React.FC<BadgeProps> = ({
  variant = 'neutral',
  dot = false,
  pulse = false,
  mono = false,
  size = 'sm',
  children,
  className = '',
  ...props
}) => {
  const v = variantStyles[variant] || variantStyles.neutral;
  const sizeClass = size === 'xs' ? 'px-2 py-0.2 text-[10px]' : 'px-2.5 py-0.5 text-[11px]';

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full font-semibold border shrink-0 ${v.bg} ${sizeClass} ${
        mono ? 'font-mono' : ''
      } ${className}`}
      {...props}
    >
      {dot && (
        <span
          className={`w-1.5 h-1.5 rounded-full shrink-0 ${v.dot} ${pulse ? 'animate-pulse' : ''}`}
        />
      )}
      {children}
    </span>
  );
};
