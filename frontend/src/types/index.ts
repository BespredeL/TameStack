export type NavigationTab =
  | 'dashboard'
  | 'servers'
  | 'provisioning'
  | 'sites'
  | 'php'
  | 'webservers'
  | 'packages'
  | 'services'
  | 'databases'
  | 'docker'
  | 'ssl'
  | 'mail'
  | 'dns'
  | 'filemanager'
  | 'cron'
  | 'backups'
  | 'monitoring'
  | 'security'
  | 'users'
  | 'terminal'
  | 'ai'
  | 'tasks'
  | 'settings';

export interface ServerRecord {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  auth_type: string;
  ssh_host_key: string;
  group_name: string;
  tags: string[];
  is_favorite: boolean;
  status: 'online' | 'offline' | 'unknown' | 'degraded';
  mode?: 'audit' | 'management';
  last_connected_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SystemUnit {
  unit: string;
  load: string;
  active: string;
  sub: string;
  description: string;
}

export interface ListeningSocket {
  proto: string;
  local: string;
  port: number;
  process: string;
  pid: number;
}

export interface OSUser {
  username: string;
  uid: number;
  gid: number;
  home: string;
  shell: string;
  is_system: boolean;
}

export interface MountPoint {
  filesystem: string;
  mount: string;
  type: string;
  total_bytes: number;
  free_bytes: number;
}

export interface CronEntry {
  schedule: string;
  command: string;
  user: string;
  source: string;
}

export interface ContainerInfo {
  id: string;
  names: string;
  image: string;
  status: string;
  ports: string;
}

export interface VerifiedLink {
  site_domain: string;
  vhost_path: string;
  vhost_hash?: string;
  has_drift?: boolean;
  fpm_socket: string;
  pool_name: string;
  cert_path: string;
  status: 'proven' | 'unproven';
  note: string;
}

export interface DatabaseInfo {
  engine: string;
  version: string;
  databases: string[];
}

export interface ServerInventorySnapshot {
  id: string;
  server_id: string;
  units: SystemUnit[];
  sockets: ListeningSocket[];
  users: OSUser[];
  mounts: MountPoint[];
  cron_jobs: CronEntry[];
  containers: ContainerInfo[];
  databases?: DatabaseInfo[];
  links: VerifiedLink[];
  created_at: string;
}

export interface DiagnosticsRecord {
  server_id: string;
  os_name: string;
  os_version: string;
  os_family: string;
  kernel_version: string;
  arch: string;
  cpu_cores: number;
  cpu_model: string;
  ram_total_bytes: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  uptime_seconds: number;
  ssh_version: string;
  installed_webservers: string[];
  installed_php_versions: string[];
  installed_db_versions: string[];
  docker_version: string;
  diagnosed_at: string;
}

export interface TaskRecord {
  id: string;
  server_id: string;
  operation_type: string;
  title: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  current_step: string;
  progress: number;
  payload: string;
  result: string;
  error_message: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface TaskLogRecord {
  id: number;
  task_id: string;
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  created_at: string;
}

export interface HostKeyInspection {
  status: 'known' | 'new' | 'mismatch';
  fingerprint: string;
  key_type: string;
  raw_entry: string;
  message: string;
}

export interface DashboardStats {
  total_servers: number;
  online_servers: number;
  offline_servers: number;
  unknown_servers: number;
  total_cores: number;
  total_ram_bytes: number;
  total_disk_bytes: number;
  total_disk_used: number;
  total_sites?: number;
  active_sites?: number;
  total_ssl_certs?: number;
  expiring_ssl_certs?: number;
  total_backups?: number;
  total_tasks: number;
  running_tasks: number;
  failed_tasks?: number;
  recent_tasks: TaskRecord[];
  server_diagnostics: DiagnosticsRecord[];
}

export interface AIProviderConfigDTO {
  id: string;
  provider_type: 'ollama' | 'openai_compatible' | 'llamacpp' | 'openai' | 'gemini' | 'anthropic' | string;
  name: string;
  base_url: string;
  has_api_key: boolean;
  is_enabled: boolean;
  is_default: boolean;
  default_model?: string;
  system_prompt?: string;
  created_at: string;
  updated_at: string;
}

export interface AIConversationRecord {
  id: string;
  title: string;
  server_id?: string;
  system_prompt: string;
  selected_provider: string;
  selected_model: string;
  created_at: string;
  updated_at: string;
}

export interface AIMessageRecord {
  id: string;
  conversation_id: string;
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
  tool_calls_json?: string;
  tool_call_id?: string;
  prompt_tokens: number;
  completion_tokens: number;
  created_at: string;
}

export interface AIToolDefinition {
  name: string;
  description: string;
  risk_level: string;
  is_mutating: boolean;
  parameters?: Record<string, any>;
}

export interface AIMemoryFactRecord {
  id: string;
  server_id?: string;
  category: string;
  fact_key: string;
  fact_value: string;
  source: string;
  created_at: string;
  updated_at: string;
}

export interface RouteDecision {
  provider_type: string;
  model: string;
  reason: string;
  estimated_cost: number;
  is_fallback: boolean;
}

export interface TaskProgressPayload {
  task_id: string;
  step: string;
  progress: number;
}

export interface VaultRequireUnlockPayload {
  reason: string;
}

export interface AIStreamChunkPayload {
  conversation_id: string;
  content_delta: string;
  tool_call_delta?: unknown;
  finish_reason: string;
  usage?: unknown;
}

export interface AIStreamCompletePayload {
  conversation_id: string;
}

export interface AIStreamErrorPayload {
  conversation_id: string;
  error: string;
}

