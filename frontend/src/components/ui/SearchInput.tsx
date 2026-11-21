import React from 'react';
import { Search, X } from 'lucide-react';

export interface SearchInputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  value: string;
  onChangeValue?: (value: string) => void;
  onClear?: () => void;
  sizeVariant?: 'sm' | 'md';
  shortcutHint?: string;
}

export const SearchInput: React.FC<SearchInputProps> = ({
  value,
  onChangeValue,
  onChange,
  onClear,
  sizeVariant = 'md',
  placeholder = 'Поиск...',
  shortcutHint,
  className = '',
  ...props
}) => {
  const isSm = sizeVariant === 'sm';

  const handleClear = () => {
    if (onChangeValue) onChangeValue('');
    if (onClear) onClear();
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (onChange) onChange(e);
    if (onChangeValue) onChangeValue(e.target.value);
  };

  return (
    <div className={`relative flex items-center min-w-0 w-full ${className}`}>
      <Search className={`absolute left-3 text-slate-400 pointer-events-none shrink-0 ${isSm ? 'w-3.5 h-3.5' : 'w-4 h-4'}`} />
      <input
        type="text"
        value={value}
        onChange={handleChange}
        placeholder={placeholder}
        className={`w-full pl-9 bg-slate-950/80 border border-slate-800 hover:border-slate-700 text-white placeholder-slate-500 font-sans transition-colors duration-150 outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/30 ${
          isSm ? 'h-8 text-xs rounded-lg' : 'h-9 text-xs rounded-xl'
        } ${value ? 'pr-8' : shortcutHint ? 'pr-14' : 'pr-3'}`}
        {...props}
      />
      {value ? (
        <button
          type="button"
          onClick={handleClear}
          aria-label="Очистить поиск"
          className="absolute right-2.5 p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <X className="w-3 h-3" />
        </button>
      ) : shortcutHint ? (
        <kbd className="absolute right-2 px-1.5 py-0.5 text-[10px] font-mono text-slate-500 bg-slate-900 border border-slate-800 rounded pointer-events-none">
          {shortcutHint}
        </kbd>
      ) : null}
    </div>
  );
};
