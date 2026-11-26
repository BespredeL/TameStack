import React, { useState } from 'react';
import { Copy, Check } from 'lucide-react';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

interface MarkdownRendererProps {
  content: string;
}

/**
 * Renders Markdown safely into React elements (headings, lists, tables,
 * blockquotes, inline bold/italic/code/links, and fenced code blocks with copy).
 */
export const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  const [copiedBlock, setCopiedBlock] = useState<number | null>(null);

  const copyCode = (code: string, idx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedBlock(idx);
    setTimeout(() => setCopiedBlock(null), 1500);
  };

  const renderInline = (text: string, keyPrefix: string): React.ReactNode[] => {
    // Match `code`, **bold**, *italic*, or [label](url)
    const tokenRegex = /(`[^`\n]+`|\*\*[^*\n]+\*\*|\*[^*\n]+\*|\[[^\]\n]+\]\(https?:\/\/[^\s)]+\))/g;
    const nodes: React.ReactNode[] = [];
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    let partIdx = 0;

    while ((match = tokenRegex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        nodes.push(text.slice(lastIndex, match.index));
      }
      const token = match[0];
      const key = `${keyPrefix}-${partIdx++}`;
      if (token.startsWith('`') && token.endsWith('`')) {
        nodes.push(
          <code key={key} className="px-1.5 py-0.5 rounded bg-slate-950/90 border border-slate-800 font-mono text-[11px] text-indigo-300">
            {token.slice(1, -1)}
          </code>
        );
      } else if (token.startsWith('**') && token.endsWith('**')) {
        nodes.push(
          <strong key={key} className="font-semibold text-white">
            {token.slice(2, -2)}
          </strong>
        );
      } else if (token.startsWith('*') && token.endsWith('*')) {
        nodes.push(
          <em key={key} className="italic text-slate-200">
            {token.slice(1, -1)}
          </em>
        );
      } else if (token.startsWith('[')) {
        const m = token.match(/^\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)$/);
        if (m) {
          const [, label, url] = m;
          nodes.push(
            <button
              key={key}
              type="button"
              onClick={() => BrowserOpenURL(url)}
              className="text-indigo-400 hover:text-indigo-300 underline underline-offset-2 cursor-pointer"
            >
              {label}
            </button>
          );
        } else {
          nodes.push(token);
        }
      }
      lastIndex = tokenRegex.lastIndex;
    }

    if (lastIndex < text.length) {
      nodes.push(text.slice(lastIndex));
    }
    return nodes;
  };

  const renderMarkdownBlock = (block: string, blockKey: string) => {
    const lines = block.split('\n');
    const elements: React.ReactNode[] = [];
    let i = 0;

    while (i < lines.length) {
      const line = lines[i];
      const trimmed = line.trim();

      if (!trimmed) {
        i++;
        continue;
      }

      // Horizontal rule
      if (/^(-{3,}|\*{3,})$/.test(trimmed)) {
        elements.push(<hr key={`${blockKey}-hr-${i}`} className="border-slate-800 my-2" />);
        i++;
        continue;
      }

      // Headings
      const headingMatch = trimmed.match(/^(#{1,4})\s+(.+)$/);
      if (headingMatch) {
        const level = headingMatch[1].length;
        const text = headingMatch[2];
        const cls =
          level === 1
            ? 'text-sm font-bold text-white mt-2 mb-1'
            : level === 2
              ? 'text-[13px] font-bold text-white mt-2 mb-1'
              : 'text-xs font-semibold text-indigo-200 mt-1.5 mb-0.5';
        elements.push(
          <div key={`${blockKey}-h-${i}`} className={cls}>
            {renderInline(text, `${blockKey}-h-${i}`)}
          </div>
        );
        i++;
        continue;
      }

      // Blockquote
      if (trimmed.startsWith('> ')) {
        const quoteLines: string[] = [];
        while (i < lines.length && lines[i].trim().startsWith('> ')) {
          quoteLines.push(lines[i].trim().slice(2));
          i++;
        }
        elements.push(
          <blockquote
            key={`${blockKey}-bq-${i}`}
            className="border-l-2 border-indigo-500/60 pl-2.5 py-0.5 my-1.5 text-slate-300 bg-slate-950/40 rounded-r"
          >
            {quoteLines.map((ql, qi) => (
              <div key={qi}>{renderInline(ql, `${blockKey}-bq-${i}-${qi}`)}</div>
            ))}
          </blockquote>
        );
        continue;
      }

      // Markdown table
      if (trimmed.includes('|') && i + 1 < lines.length && /^\s*\|?[\s:-]+\|[\s|:-]*$/.test(lines[i + 1])) {
        const parseRow = (r: string) =>
          r
            .trim()
            .replace(/^\|/, '')
            .replace(/\|$/, '')
            .split('|')
            .map((c) => c.trim());
        const headers = parseRow(trimmed);
        i += 2;
        const rows: string[][] = [];
        while (i < lines.length && lines[i].trim().includes('|')) {
          rows.push(parseRow(lines[i]));
          i++;
        }
        elements.push(
          <div key={`${blockKey}-tbl-${i}`} className="my-2 overflow-x-auto rounded-lg border border-slate-800">
            <table className="w-full text-[11px] text-left">
              <thead className="bg-slate-950 text-slate-300 border-b border-slate-800">
                <tr>
                  {headers.map((h, hi) => (
                    <th key={hi} className="px-2 py-1.5 font-semibold">
                      {renderInline(h, `${blockKey}-th-${i}-${hi}`)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/70">
                {rows.map((row, ri) => (
                  <tr key={ri}>
                    {row.map((cell, ci) => (
                      <td key={ci} className="px-2 py-1 text-slate-300">
                        {renderInline(cell, `${blockKey}-td-${i}-${ri}-${ci}`)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
        continue;
      }

      // Unordered list
      if (/^[-*+]\s+/.test(trimmed)) {
        const items: string[] = [];
        while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) {
          items.push(lines[i].replace(/^\s*[-*+]\s+/, ''));
          i++;
        }
        elements.push(
          <ul key={`${blockKey}-ul-${i}`} className="list-disc list-inside space-y-1 my-1 text-slate-200">
            {items.map((item, ii) => (
              <li key={ii} className="leading-relaxed">
                {renderInline(item, `${blockKey}-ul-${i}-${ii}`)}
              </li>
            ))}
          </ul>
        );
        continue;
      }

      // Ordered list
      if (/^\d+\.\s+/.test(trimmed)) {
        const items: string[] = [];
        while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
          items.push(lines[i].replace(/^\s*\d+\.\s+/, ''));
          i++;
        }
        elements.push(
          <ol key={`${blockKey}-ol-${i}`} className="list-decimal list-inside space-y-1 my-1 text-slate-200">
            {items.map((item, ii) => (
              <li key={ii} className="leading-relaxed">
                {renderInline(item, `${blockKey}-ol-${i}-${ii}`)}
              </li>
            ))}
          </ol>
        );
        continue;
      }

      // Regular paragraph line
      elements.push(
        <p key={`${blockKey}-p-${i}`} className="leading-relaxed my-1">
          {renderInline(line, `${blockKey}-p-${i}`)}
        </p>
      );
      i++;
    }

    return elements;
  };

  const parts = content.split(/(```[\s\S]*?```)/g);
  return (
    <div className="space-y-1 break-words">
      {parts.map((part, idx) => {
        if (part.startsWith('```') && part.endsWith('```')) {
          const inner = part.slice(3, -3);
          const firstNl = inner.indexOf('\n');
          const lang = firstNl !== -1 ? inner.slice(0, firstNl).trim() : '';
          const code = (firstNl !== -1 ? inner.slice(firstNl + 1) : inner).trim();
          return (
            <div key={idx} className="my-2 rounded-lg bg-slate-950 border border-slate-800 overflow-hidden">
              <div className="px-2.5 py-1 bg-slate-900/80 border-b border-slate-800 flex items-center justify-between text-[10px] text-slate-400">
                <span className="font-mono">{lang || 'code'}</span>
                <button
                  type="button"
                  onClick={() => copyCode(code, idx)}
                  className="inline-flex items-center gap-1 hover:text-white"
                  title="Copy code"
                >
                  {copiedBlock === idx ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                </button>
              </div>
              <pre className="p-2.5 text-[11px] font-mono text-slate-200 overflow-x-auto whitespace-pre-wrap">{code}</pre>
            </div>
          );
        }
        return <React.Fragment key={idx}>{renderMarkdownBlock(part, `b-${idx}`)}</React.Fragment>;
      })}
    </div>
  );
};
