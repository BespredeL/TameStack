import React, { createContext, useContext, useState, useEffect, useCallback, useMemo } from 'react';
import { ServerRecord } from '../types';

interface ServerContextProps {
  selectedServerId: string;
  setSelectedServerId: (id: string) => void;
  selectedServer: ServerRecord | undefined;
  servers: ServerRecord[];
  isPageLoading: boolean;
  pageLoadingMessage: string | null;
  setPageLoading: (loading: boolean, message?: string | null) => void;
}

const ServerContext = createContext<ServerContextProps>({
  selectedServerId: '',
  setSelectedServerId: () => { },
  selectedServer: undefined,
  servers: [],
  isPageLoading: false,
  pageLoadingMessage: null,
  setPageLoading: () => { },
});

export const useServer = () => useContext(ServerContext);

interface ServerProviderProps {
  servers: ServerRecord[];
  children: React.ReactNode;
}

export const ServerProvider: React.FC<ServerProviderProps> = ({ servers, children }) => {
  const [selectedServerId, setSelectedServerIdState] = useState<string>(() => {
    return localStorage.getItem('tamestack_selected_server_id') || '';
  });
  const [isPageLoading, setIsPageLoading] = useState<boolean>(false);
  const [pageLoadingMessage, setPageLoadingMessage] = useState<string | null>(null);

  const setPageLoading = useCallback((loading: boolean, message?: string | null) => {
    setIsPageLoading(loading);
    setPageLoadingMessage(loading ? (message ?? null) : null);
  }, []);

  const setSelectedServerId = useCallback((id: string) => {
    setSelectedServerIdState(id);
    if (id) {
      localStorage.setItem('tamestack_selected_server_id', id);
    }
  }, []);

  useEffect(() => {
    if (servers.length > 0) {
      const exists = servers.some((s) => s.id === selectedServerId);
      if (!exists) {
        setSelectedServerId(servers[0].id);
      }
    }
  }, [servers, selectedServerId, setSelectedServerId]);

  const selectedServer = useMemo(() => {
    return servers.find((s) => s.id === selectedServerId) || servers[0];
  }, [servers, selectedServerId]);

  const contextValue = useMemo(() => ({
    selectedServerId,
    setSelectedServerId,
    selectedServer,
    servers,
    isPageLoading,
    pageLoadingMessage,
    setPageLoading,
  }), [
    selectedServerId,
    setSelectedServerId,
    selectedServer,
    servers,
    isPageLoading,
    pageLoadingMessage,
    setPageLoading,
  ]);

  return (
    <ServerContext.Provider value={contextValue}>
      {children}
    </ServerContext.Provider>
  );
};
