import React from 'react';
import { ChevronDown } from 'lucide-react';

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  sizeVariant?: 'sm' | 'md' | 'lg';
  icon?: React.ReactNode;
}

const sizeStyles = {
  sm: 'h-8 pl-3 pr-8 text-xs rounded-lg',
  md: 'h-9 pl-3.5 pr-9 text-xs rounded-xl font-medium',
  lg: 'h-10 pl-4 pr-10 text-sm rounded-xl',
};

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  (
    {
      sizeVariant = 'md',
      icon,
      className = '',
      children,
      disabled,
      ...props
    },
    ref
  ) => {
    return (
      <div className="relative inline-flex items-center w-full">
        {icon && (
          <div className="absolute left-3 text-slate-400 pointer-events-none flex items-center z-10">
            {icon}
          </div>
        )}
        <select
          ref={ref}
          disabled={disabled}
          className={`w-full bg-slate-950/80 border border-slate-800 hover:border-slate-700 text-white font-sans transition-colors duration-150 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30 appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed ${
            sizeStyles[sizeVariant]
          } ${icon ? 'pl-9' : ''} ${className}`}
          {...props}
        >
          {children}
        </select>
        <ChevronDown className="absolute right-3 w-3.5 h-3.5 text-slate-400 pointer-events-none" />
      </div>
    );
  }
);

Select.displayName = 'Select';
