import React, { Component, ErrorInfo, ReactNode } from 'react';
import { ru } from '../../i18n/ru';
import { en } from '../../i18n/en';
import { AlertTriangle, RefreshCw, Copy, Check, Home } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
  onReset?: () => void;
  variant?: 'inline' | 'fullscreen';
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
  copied: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
    errorInfo: null,
    copied: false,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error, errorInfo: null, copied: false };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    const label =
      this.props.variant === 'fullscreen'
        ? 'Uncaught error caught by ErrorBoundary:'
        : 'TameStack Uncaught UI Error:';
    console.error(label, error, errorInfo);
    this.setState({ errorInfo });
  }

  private handleReset = () => {
    this.setState({ hasError: false, error: null, errorInfo: null, copied: false });
    if (this.props.onReset) {
      this.props.onReset();
    }
  };

  private handleCopy = () => {
    const errorDetails = `TameStack UI Error Report:
Error: ${this.state.error?.message || 'Unknown error'}
Stack: ${this.state.error?.stack || 'No stack trace'}
Component Stack: ${this.state.errorInfo?.componentStack || 'No component stack'}
Time: ${new Date().toISOString()}`;

    navigator.clipboard.writeText(errorDetails).then(() => {
      this.setState({ copied: true });
      setTimeout(() => this.setState({ copied: false }), 2500);
    });
  };

  getDict() {
    const lang = localStorage.getItem('tamestack_lang');
    return lang === 'en' ? en : ru;
  }

  public render() {
    const d = this.getDict();
    if (!this.state.hasError) {
      return this.props.children;
    }

    if (this.props.variant === 'fullscreen') {
      return (
        <div className="flex h-screen w-screen items-center justify-center bg-slate-950 p-6 text-slate-100 select-text">
          <div className="max-w-xl w-full p-8 rounded-2xl bg-slate-900 border border-rose-500/30 shadow-2xl space-y-6">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-center text-rose-400 shrink-0">
                <AlertTriangle className="w-6 h-6" />
              </div>
              <div>
                <h1 className="text-lg font-bold text-white">{d.common.errorBoundary.appErrorTitle}</h1>
                <p className="text-xs text-slate-400 mt-0.5">
                  {d.common.errorBoundary.boundaryAppDesc}
                </p>
              </div>
            </div>

            {this.state.error && (
              <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 font-mono text-xs text-rose-300 overflow-x-auto max-h-48 leading-relaxed">
                <div className="font-bold mb-1 text-rose-400">{this.state.error.name}: {this.state.error.message}</div>
                {this.state.error.stack && (
                  <pre className="text-[11px] text-slate-500 whitespace-pre-wrap">{this.state.error.stack}</pre>
                )}
              </div>
            )}

            <div className="flex items-center justify-end gap-3 pt-2 border-t border-slate-800">
              <button
                onClick={this.handleReset}
                className="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-indigo-600/20 transition-all"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>{d.common.errorBoundary.reloadUi}</span>
              </button>
            </div>
          </div>
        </div>
      );
    }

    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] h-full p-8 text-center bg-slate-900/60 rounded-xl border border-rose-500/30 m-4">
        <div className="w-14 h-14 rounded-2xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 mb-4 shadow-lg shadow-rose-950/20">
          <AlertTriangle className="w-7 h-7" />
        </div>

        <h2 className="text-xl font-bold text-slate-100 mb-2">
          {this.props.fallbackTitle || d.common.errorBoundary.defaultTitle}
        </h2>

        <p className="text-sm text-slate-400 max-w-md mb-6">
          {d.common.errorBoundary.boundaryDesc}
        </p>

        {this.state.error && (
          <div className="w-full max-w-xl bg-slate-950 border border-slate-800 rounded-lg p-3.5 mb-6 text-left font-mono text-xs text-rose-300 overflow-x-auto shadow-inner">
            <span className="text-rose-400 font-semibold">{this.state.error.name}: </span>
            {this.state.error.message}
          </div>
        )}

        <div className="flex items-center gap-3">
          <button
            onClick={this.handleReset}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs rounded-lg flex items-center gap-2 shadow-lg shadow-indigo-900/30 transition-all cursor-pointer"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            {d.common.errorBoundary.tryAgain}
          </button>

          <button
            onClick={this.handleCopy}
            className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs rounded-lg flex items-center gap-2 border border-slate-700 transition-all cursor-pointer"
          >
            {this.state.copied ? (
              <>
                <Check className="w-3.5 h-3.5 text-emerald-400" />
                <span className="text-emerald-400">{d.common.errorBoundary.copied}</span>
              </>
            ) : (
              <>
                <Copy className="w-3.5 h-3.5" />
                {d.common.errorBoundary.copyReport}
              </>
            )}
          </button>

          <button
            onClick={() => window.location.reload()}
            className="px-3.5 py-2 bg-slate-800/80 hover:bg-slate-700/80 text-slate-400 hover:text-slate-200 font-medium text-xs rounded-lg flex items-center gap-2 border border-slate-700/50 transition-all cursor-pointer"
          >
            <Home className="w-3.5 h-3.5" />
            {d.common.errorBoundary.reloadApp}
          </button>
        </div>
      </div>
    );
  }
}