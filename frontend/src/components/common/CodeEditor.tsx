import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import {
  Copy,
  Check,
  WrapText,
  ChevronDown,
} from 'lucide-react';
import {
  detectLanguage,
  highlightCode,
  getLanguageLabel,
} from '../../utils/prismSetup';
import { useTranslation } from '../../i18n/LanguageContext';

export interface CodeEditorProps {
  value: string;
  onChange?: (value: string) => void;
  filePath?: string;
  language?: string;
  readOnly?: boolean;
  onSave?: () => void;
  className?: string;
}

const SUPPORTED_LANGUAGES = [
  'php',
  'python',
  'javascript',
  'typescript',
  'nginx',
  'apacheconf',
  'ini',
  'json',
  'yaml',
  'sql',
  'bash',
  'docker',
  'systemd',
  'log',
  'markup',
  'css',
  'markdown',
  'plaintext',
];

export const CodeEditor: React.FC<CodeEditorProps> = ({
  value,
  onChange,
  filePath = '',
  language: initialLang,
  readOnly = false,
  onSave,
  className = '',
}) => {
  const { t } = useTranslation();
  const [selectedLang, setSelectedLang] = useState<string>(() => {
    return initialLang || (filePath ? detectLanguage(filePath) : 'plaintext');
  });
  const [wordWrap, setWordWrap] = useState<boolean>(false);
  const [copied, setCopied] = useState<boolean>(false);
  const [cursorPos, setCursorPos] = useState<{ line: number; col: number }>({
    line: 1,
    col: 1,
  });

  const paneRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);

  // Auto-detect when filePath or initialLang changes
  useEffect(() => {
    if (initialLang) {
      setSelectedLang(initialLang);
    } else if (filePath) {
      setSelectedLang(detectLanguage(filePath));
    }
  }, [filePath, initialLang]);

  // Syntax highlighted HTML via Prism
  const highlighted = useMemo(() => {
    return highlightCode(value, selectedLang);
  }, [value, selectedLang]);

  // Total lines count
  const linesCount = useMemo(() => {
    return Math.max(1, value.split('\n').length);
  }, [value]);

  // Gutter line numbers list
  const lineNumbers = useMemo(() => {
    const nums: number[] = [];
    for (let i = 1; i <= linesCount; i++) {
      nums.push(i);
    }
    return nums;
  }, [linesCount]);

  // Handle scroll synchronization between gutter and code
  const handleScroll = useCallback(() => {
    if (paneRef.current && gutterRef.current) {
      gutterRef.current.scrollTop = paneRef.current.scrollTop;
    }
  }, []);

  // Update cursor position line & column
  const updateCursorPosition = () => {
    if (!textareaRef.current) return;
    const pos = textareaRef.current.selectionStart;
    const textBefore = value.substring(0, pos);
    const lines = textBefore.split('\n');
    setCursorPos({
      line: lines.length,
      col: lines[lines.length - 1].length + 1,
    });
  };

  // Keyboard shortcut handlers (Tab, Shift+Tab, Ctrl+S)
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // Ctrl+S / Cmd+S save shortcut
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      if (onSave) onSave();
      return;
    }

    if (readOnly) return;

    // Tab indentation
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = textareaRef.current;
      if (!textarea) return;

      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;

      if (e.shiftKey) {
        // Shift+Tab: Unindent
        const lineStart = value.lastIndexOf('\n', start - 1) + 1;
        if (value.substring(lineStart, lineStart + 2) === '  ') {
          const next = value.substring(0, lineStart) + value.substring(lineStart + 2);
          onChange?.(next);
          setTimeout(() => {
            textarea.selectionStart = textarea.selectionEnd = Math.max(lineStart, start - 2);
          }, 0);
        }
      } else {
        // Tab: Insert 2 spaces
        const next = value.substring(0, start) + '  ' + value.substring(end);
        onChange?.(next);
        setTimeout(() => {
          textarea.selectionStart = textarea.selectionEnd = start + 2;
        }, 0);
      }
    }
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
  };

  return (
    <div
      className={`flex flex-col h-full bg-[#050914] border border-slate-800 rounded-xl overflow-hidden shadow-inner ${className}`}
    >
      {/* Editor Control Toolbar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[#0a0f1d] border-b border-slate-800 text-xs text-slate-400 select-none">
        <div className="flex items-center gap-2">
          {/* Language Selector Pill */}
          <div className="relative inline-flex items-center">
            <select
              value={selectedLang}
              onChange={(e) => setSelectedLang(e.target.value)}
              className="bg-slate-900 hover:bg-slate-800/90 border border-slate-700/80 rounded-lg pl-2 pr-6 py-1 text-[11px] font-semibold text-indigo-300 focus:outline-none focus:border-indigo-500 cursor-pointer appearance-none transition"
              title="Change syntax highlighting language"
            >
              {SUPPORTED_LANGUAGES.map((l) => (
                <option key={l} value={l} className="bg-slate-900 text-slate-200">
                  {getLanguageLabel(l)}
                </option>
              ))}
            </select>
            <ChevronDown className="w-3 h-3 text-slate-400 pointer-events-none absolute right-1.5" />
          </div>

          <span className="text-[11px] text-slate-500 hidden sm:inline">
            {linesCount} {linesCount === 1 ? 'line' : 'lines'}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          {/* Word Wrap Toggle */}
          <button
            type="button"
            onClick={() => setWordWrap(!wordWrap)}
            className={`p-1.5 rounded-lg border text-xs transition cursor-pointer ${
              wordWrap
                ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/30'
                : 'bg-surface-850 hover:bg-surface-800 text-slate-400 border-slate-700/80 hover:text-slate-200'
            }`}
            title={wordWrap ? 'Disable Word Wrap' : 'Enable Word Wrap'}
          >
            <WrapText className="w-3.5 h-3.5" />
          </button>

          {/* Copy Button */}
          <button
            type="button"
            onClick={handleCopy}
            className="p-1.5 rounded-lg border bg-surface-850 hover:bg-surface-800 text-slate-400 hover:text-white border-slate-700/80 transition cursor-pointer"
            title="Copy content"
          >
            {copied ? (
              <Check className="w-3.5 h-3.5 text-emerald-400" />
            ) : (
              <Copy className="w-3.5 h-3.5" />
            )}
          </button>
        </div>
      </div>

      {/* Main Code Area */}
      <div
        className={`code-editor-root flex-1 ${
          wordWrap ? 'code-editor-wrap' : ''
        }`}
      >
        {/* Line Numbers Gutter */}
        <div ref={gutterRef} className="code-editor-gutter">
          {lineNumbers.map((num) => (
            <div key={num}>{num}</div>
          ))}
        </div>

        {/* Editing & Highlighting Pane */}
        <div
          ref={paneRef}
          onScroll={handleScroll}
          className="code-editor-pane custom-scrollbar"
        >
          {/* Syntax Highlighted Underlay */}
          <pre
            className="code-editor-pre"
            aria-hidden="true"
          >
            <code
              className={`language-${selectedLang}`}
              dangerouslySetInnerHTML={{ __html: highlighted + '\n' }}
            />
          </pre>

          {/* Transparent Input Textarea Overlay */}
          <textarea
            ref={textareaRef}
            value={value}
            onChange={(e) => {
              onChange?.(e.target.value);
              updateCursorPosition();
            }}
            onKeyUp={updateCursorPosition}
            onClick={updateCursorPosition}
            onKeyDown={handleKeyDown}
            readOnly={readOnly}
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            autoCorrect="off"
            className="code-editor-textarea"
          />
        </div>
      </div>

      {/* Editor Status Bar */}
      <div className="flex items-center justify-between px-3 py-1 bg-[#070b16] border-t border-slate-800/80 text-[11px] text-slate-500 font-mono select-none">
        <div className="flex items-center gap-3">
          <span>
            Ln {cursorPos.line}, Col {cursorPos.col}
          </span>
          <span className="hidden sm:inline">
            {(new Blob([value]).size / 1024).toFixed(1)} KB
          </span>
        </div>
        <div className="flex items-center gap-2">
          <span>UTF-8</span>
          <span className="text-indigo-400 font-semibold uppercase">
            {selectedLang}
          </span>
        </div>
      </div>
    </div>
  );
};
