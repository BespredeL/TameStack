import React from 'react';

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  icon?: React.ReactNode;
  iconRight?: React.ReactNode;
  error?: string | boolean;
  sizeVariant?: 'sm' | 'md' | 'lg';
}

const sizeStyles = {
  sm: 'h-8 px-3 text-xs rounded-lg',
  md: 'h-9 px-3.5 text-xs rounded-xl',
  lg: 'h-10 px-4 text-sm rounded-xl',
};

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      icon,
      iconRight,
      error,
      sizeVariant = 'md',
      className = '',
      disabled,
      ...props
    },
    ref
  ) => {
    return (
      <div className="relative w-full flex items-center">
        {icon && (
          <div className="absolute left-3 text-slate-400 pointer-events-none flex items-center">
            {icon}
          </div>
        )}
        <input
          ref={ref}
          disabled={disabled}
          className={`w-full bg-slate-950/80 border text-white placeholder-slate-500 font-sans transition-colors duration-150 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30 disabled:opacity-50 disabled:cursor-not-allowed ${
            error
              ? 'border-rose-500/70 focus:border-rose-500 focus:ring-rose-500/20'
              : 'border-slate-800 hover:border-slate-700'
          } ${sizeStyles[sizeVariant]} ${icon ? 'pl-9' : ''} ${iconRight ? 'pr-9' : ''} ${className}`}
          {...props}
        />
        {iconRight && (
          <div className="absolute right-3 text-slate-400 flex items-center">
            {iconRight}
          </div>
        )}
      </div>
    );
  }
);

Input.displayName = 'Input';
