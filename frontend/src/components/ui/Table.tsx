import React from 'react';
import { Loader2 } from 'lucide-react';

export const TableContainer: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <div
    className={`bg-surface-850 border border-slate-800 rounded-2xl overflow-hidden shadow-sm overflow-x-auto ${className}`}
    {...props}
  >
    {children}
  </div>
);

export const Table: React.FC<React.TableHTMLAttributes<HTMLTableElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <table className={`w-full text-left border-collapse text-xs ${className}`} {...props}>
    {children}
  </table>
);

export const TableHeader: React.FC<React.HTMLAttributes<HTMLTableSectionElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <thead
    className={`bg-slate-950/60 text-slate-400 font-semibold uppercase tracking-wider border-b border-slate-800 ${className}`}
    {...props}
  >
    {children}
  </thead>
);

export const TableBody: React.FC<React.HTMLAttributes<HTMLTableSectionElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <tbody className={`divide-y divide-slate-800/60 text-slate-200 ${className}`} {...props}>
    {children}
  </tbody>
);

export const TableRow: React.FC<React.HTMLAttributes<HTMLTableRowElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <tr className={`hover:bg-slate-800/35 transition-colors duration-100 ${className}`} {...props}>
    {children}
  </tr>
);

export const TableHead: React.FC<React.ThHTMLAttributes<HTMLTableCellElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <th className={`px-4 py-3 text-[11px] font-semibold text-slate-400 uppercase select-none ${className}`} {...props}>
    {children}
  </th>
);

export const TableCell: React.FC<React.TdHTMLAttributes<HTMLTableCellElement>> = ({
  className = '',
  children,
  ...props
}) => (
  <td className={`px-4 py-3 font-normal text-slate-200 align-middle ${className}`} {...props}>
    {children}
  </td>
);

export const TableEmpty: React.FC<{
  colSpan: number;
  message?: string;
  icon?: React.ReactNode;
}> = ({ colSpan, message = 'Данные не найдены', icon }) => (
  <tr>
    <td colSpan={colSpan} className="px-6 py-12 text-center text-slate-500">
      <div className="flex flex-col items-center justify-center gap-2">
        {icon && <div className="text-slate-600 mb-1">{icon}</div>}
        <span className="text-xs">{message}</span>
      </div>
    </td>
  </tr>
);

export const TableLoading: React.FC<{
  colSpan: number;
  message?: string;
}> = ({ colSpan, message = 'Загрузка данных...' }) => (
  <tr>
    <td colSpan={colSpan} className="px-6 py-12 text-center text-slate-400">
      <div className="flex items-center justify-center gap-2.5">
        <Loader2 className="w-4 h-4 animate-spin text-indigo-400" />
        <span className="text-xs">{message}</span>
      </div>
    </td>
  </tr>
);
