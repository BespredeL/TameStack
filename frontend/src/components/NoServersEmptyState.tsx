import React from 'react';
import { Server as ServerIcon } from 'lucide-react';
import { useTranslation } from '../i18n/LanguageContext';
import { Button, EmptyState } from './ui';

interface NoServersEmptyStateProps {
  icon?: React.ComponentType<{ className?: string }>;
  title?: string;
  description: string;
  onOpenAddServer: () => void;
}

export const NoServersEmptyState: React.FC<NoServersEmptyStateProps> = ({
  icon: Icon = ServerIcon,
  title,
  description,
  onOpenAddServer,
}) => {
  const { t } = useTranslation();

  return (
    <EmptyState
      icon={<Icon className="w-8 h-8" />}
      title={title || t('empty.noServersTitle')}
      description={description}
      action={
        <Button
          variant="success"
          size="md"
          onClick={onOpenAddServer}
          icon={<ServerIcon className="w-4 h-4" />}
        >
          {t('common.addServer')}
        </Button>
      }
    />
  );
};
