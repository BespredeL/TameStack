import React, { useState } from 'react';
import { Plus, Server, Filter } from 'lucide-react';
import { ServerRecord, DiagnosticsRecord } from '../types';
import { ServerCard } from '../components/ServerCard';
import { NoServersEmptyState } from '../components/NoServersEmptyState';
import { SearchInput, Button, Tabs, PageToolbar, EmptyState } from '../components/ui';
import { useTranslation } from '../i18n/LanguageContext';
import { useServer } from '../context/ServerContext';

interface ServersViewProps {
  servers: ServerRecord[];
  diagnosticsMap: Record<string, DiagnosticsRecord>;
  onRefresh: () => void;
  onOpenAddServer: () => void;
  onSelectServer?: (server: ServerRecord) => void;
  onEditServer?: (server: ServerRecord) => void;
}

export const ServersView: React.FC<ServersViewProps> = ({
  servers,
  diagnosticsMap,
  onRefresh,
  onOpenAddServer,
  onSelectServer,
  onEditServer,
}) => {
  const { t } = useTranslation();
  const { setSelectedServerId } = useServer();
  const [search, setSearch] = useState('');
  const [selectedGroup, setSelectedGroup] = useState<string>('all');

  const handleSelectServer = (server: ServerRecord) => {
    setSelectedServerId(server.id);
    if (onSelectServer) {
      onSelectServer(server);
    }
  };

  if (servers.length === 0) {
    return (
      <NoServersEmptyState
        icon={Server}
        description={t('empty.serversDesc')}
        onOpenAddServer={onOpenAddServer}
      />
    );
  }

  // Collect unique group names
  const groups = Array.from(new Set(servers.map((s) => s.group_name).filter(Boolean)));

  // Filter servers
  const filtered = servers.filter((s) => {
    const matchesSearch =
      s.name.toLowerCase().includes(search.toLowerCase()) ||
      s.host.toLowerCase().includes(search.toLowerCase()) ||
      s.tags?.some((t) => t.toLowerCase().includes(search.toLowerCase()));

    const matchesGroup = selectedGroup === 'all' || s.group_name === selectedGroup;

    return matchesSearch && matchesGroup;
  });

  return (
    <div className="page space-y-6">
      {/* Top Filter & Action Bar */}
      <PageToolbar
        icon={<Server className="w-5 h-5 text-indigo-400" />}
        title={t('nav.servers')}
        subtitle={`${servers.length} ${t('dashboard.nodes') || 'серверов'}`}
        filters={
          groups.length > 0 ? (
            <Tabs
              tabs={[
                { id: 'all', label: t('common.all') },
                ...groups.map((grp) => ({ id: grp, label: grp })),
              ]}
              activeTab={selectedGroup}
              onChange={setSelectedGroup}
              variant="pill"
            />
          ) : undefined
        }
        search={
          <SearchInput
            placeholder={t('common.search')}
            value={search}
            onChangeValue={setSearch}
            sizeVariant="md"
          />
        }
        actions={
          <Button
            variant="primary"
            size="md"
            onClick={onOpenAddServer}
            icon={<Plus className="w-4 h-4" />}
          >
            {t('common.addServer')}
          </Button>
        }
      />

      {/* Servers Grid */}
      {filtered.length === 0 ? (
        <EmptyState
          icon={<Server className="w-8 h-8 text-slate-500" />}
          title={t('servers.notFoundTitle')}
          description={t('servers.notFoundDesc', { query: search })}
        />
      ) : (
        <div className="card-grid">
          {filtered.map((s) => (
            <ServerCard
              key={s.id}
              server={s}
              diagnostics={diagnosticsMap[s.id]}
              onRefresh={onRefresh}
              onSelectServer={handleSelectServer}
              onEditServer={onEditServer}
            />
          ))}
        </div>
      )}
    </div>
  );
};
