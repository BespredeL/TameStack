import React from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';
import App from './App';
import { LanguageProvider } from './i18n/LanguageContext';
import { ConfirmProvider } from './context/ConfirmContext';
import { AlertProvider } from './context/AlertContext';
import { ErrorBoundary } from './components/common/ErrorBoundary';

const container = document.getElementById('root');

const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <ErrorBoundary variant="fullscreen">
      <LanguageProvider>
        <ConfirmProvider>
          <AlertProvider>
            <App />
          </AlertProvider>
        </ConfirmProvider>
      </LanguageProvider>
    </ErrorBoundary>
  </React.StrictMode>
);
