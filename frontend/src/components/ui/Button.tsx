import React from 'react';
import { Loader2 } from 'lucide-react';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'warning' | 'success' | 'outline';
export type ButtonSize = 'xs' | 'sm' | 'md' | 'lg' | 'icon-sm' | 'icon-md';

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  icon?: React.ReactNode;
  iconRight?: React.ReactNode;
  static?: boolean;
}

const variantStyles: Record<ButtonVariant, string> = {
  primary:
    'bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 text-white shadow-md shadow-indigo-600/25 border border-indigo-500/30 focus-visible:ring-indigo-500',
  secondary:
    'bg-slate-800 hover:bg-slate-700 active:bg-slate-850 text-slate-200 border border-slate-700/80 shadow-xs focus-visible:ring-slate-500',
  ghost:
    'bg-transparent hover:bg-slate-800/60 active:bg-slate-800 text-slate-300 hover:text-white border border-transparent focus-visible:ring-slate-500',
  danger:
    'bg-rose-500/10 hover:bg-rose-500/20 active:bg-rose-500/30 text-rose-300 border border-rose-500/30 shadow-xs focus-visible:ring-rose-500',
  warning:
    'bg-amber-500/10 hover:bg-amber-500/20 active:bg-amber-500/30 text-amber-300 border border-amber-500/30 shadow-xs focus-visible:ring-amber-500',
  success:
    'bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white shadow-md shadow-emerald-600/25 border border-emerald-500/30 focus-visible:ring-emerald-500',
  outline:
    'bg-transparent hover:bg-slate-800/40 text-slate-300 hover:text-white border border-slate-700 focus-visible:ring-slate-500',
};

const sizeStyles: Record<ButtonSize, string> = {
  xs: 'h-7 px-2.5 text-[11px] rounded-lg gap-1.5',
  sm: 'h-8 px-3 text-xs rounded-lg gap-1.5',
  md: 'h-9 px-4 text-xs font-semibold rounded-xl gap-2',
  lg: 'h-10 px-5 text-sm font-semibold rounded-xl gap-2.5',
  'icon-sm': 'h-8 w-8 p-0 rounded-lg justify-center',
  'icon-md': 'h-9 w-9 p-0 rounded-xl justify-center',
};

// Optical padding adjustment: 2px less padding on icon side to balance weight
const getOpticalPadding = (size: ButtonSize, hasIconLeft: boolean, hasIconRight: boolean, hasChildren: boolean) => {
  if (!hasChildren) return '';
  if (hasIconLeft && !hasIconRight) {
    switch (size) {
      case 'xs': return 'ps-2 pe-2.5';
      case 'sm': return 'ps-2.5 pe-3';
      case 'md': return 'ps-3 pe-4';
      case 'lg': return 'ps-3.5 pe-5';
      default: return '';
    }
  }
  if (hasIconRight && !hasIconLeft) {
    switch (size) {
      case 'xs': return 'ps-2.5 pe-2';
      case 'sm': return 'ps-3 pe-2.5';
      case 'md': return 'ps-4 pe-3';
      case 'lg': return 'ps-5 pe-3.5';
      default: return '';
    }
  }
  return '';
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'secondary',
      size = 'md',
      loading = false,
      disabled = false,
      icon,
      iconRight,
      static: isStatic = false,
      children,
      className = '',
      type = 'button',
      ...props
    },
    ref
  ) => {
    const isDisabled = disabled || loading;
    const hasIconLeft = Boolean(icon || loading);
    const hasIconRight = Boolean(iconRight && !loading);
    const opticalPadding = getOpticalPadding(size, hasIconLeft, hasIconRight, Boolean(children));
    const tapScale = !isStatic && !isDisabled ? 'motion-safe:enabled:active:scale-[0.96]' : '';

    return (
      <button
        ref={ref}
        type={type}
        disabled={isDisabled}
        className={`inline-flex items-center justify-center font-medium select-none transition-colors duration-150 ease-out motion-safe:transition-transform outline-none focus-visible:ring-2 focus-visible:ring-offset-1 focus-visible:ring-offset-surface-900 disabled:opacity-50 disabled:pointer-events-none cursor-pointer ${variantStyles[variant]} ${sizeStyles[size]} ${opticalPadding} ${tapScale} ${className}`}
        {...props}
      >
        {loading ? (
          <Loader2 className="w-3.5 h-3.5 animate-spin shrink-0 text-current" />
        ) : (
          icon && <span className="shrink-0 flex items-center">{icon}</span>
        )}
        {children}
        {!loading && iconRight && <span className="shrink-0 flex items-center">{iconRight}</span>}
      </button>
    );
  }
);

Button.displayName = 'Button';
