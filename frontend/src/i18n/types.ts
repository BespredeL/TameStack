export type Language = 'ru' | 'en';

export interface Translations {
  app?: Record<string, any>;
  nav: Record<string, any>;
  header: Record<string, any>;
  titles: Record<string, any>;
  common: Record<string, any>;
  empty: Record<string, any>;
  dashboard?: Record<string, any>;
  servers?: Record<string, any>;
  sites?: Record<string, any>;
  manageSite?: Record<string, any>;
  siteApp?: Record<string, any>;
  createSite?: Record<string, any>;
  security?: Record<string, any>;
  fileScanner?: Record<string, any>;
  php?: Record<string, any>;
  webservers?: Record<string, any>;
  packages?: Record<string, any>;
  services?: Record<string, any>;
  databases?: Record<string, any>;
  docker?: Record<string, any>;
  ssl?: Record<string, any>;
  filemanager?: Record<string, any>;
  cron?: Record<string, any>;
  backups?: Record<string, any>;
  monitoring?: Record<string, any>;
  terminal?: Record<string, any>;
  ai?: Record<string, any>;
  tasks?: Record<string, any>;
  settings?: Record<string, any>;
  vault?: Record<string, any>;
  commandPalette?: Record<string, any>;
  modals?: Record<string, any>;
  [key: string]: any;
}
