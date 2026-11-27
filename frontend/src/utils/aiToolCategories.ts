import { AIToolDefinition } from '../types';

export type ToolCategory = 'all' | 'system' | 'web_php' | 'database' | 'security' | 'workflows';

export interface CategoryMeta {
  id: ToolCategory;
  labelKey: string;
  iconName: string;
}

export const TOOL_CATEGORIES: CategoryMeta[] = [
  { id: 'all', labelKey: 'ai.toolsManager.catAll', iconName: 'Layers' },
  { id: 'system', labelKey: 'ai.toolsManager.catSystem', iconName: 'Server' },
  { id: 'web_php', labelKey: 'ai.toolsManager.catWebPhp', iconName: 'Globe' },
  { id: 'database', labelKey: 'ai.toolsManager.catDatabase', iconName: 'Database' },
  { id: 'security', labelKey: 'ai.toolsManager.catSecurity', iconName: 'Shield' },
  { id: 'workflows', labelKey: 'ai.toolsManager.catWorkflows', iconName: 'Sparkles' },
];

export function getToolCategory(toolName: string): ToolCategory {
  const name = toolName.toLowerCase();

  if (
    name.startsWith('run_') ||
    name.startsWith('diagnose_') ||
    name.startsWith('optimize_') ||
    name.includes('workflow')
  ) {
    return 'workflows';
  }

  if (
    name.includes('security') ||
    name.includes('firewall') ||
    name.includes('fail2ban') ||
    name.includes('ssl') ||
    name.includes('cert') ||
    name.includes('dns') ||
    name.includes('ssh_keys') ||
    name.includes('system_users') ||
    name.includes('check_port')
  ) {
    return 'security';
  }

  if (name.includes('database') || name.includes('db')) {
    return 'database';
  }

  if (
    name.includes('site') ||
    name.includes('app_process') ||
    name.includes('vhost') ||
    name.includes('webserver') ||
    name.includes('php') ||
    name.includes('nginx') ||
    name.includes('apache')
  ) {
    return 'web_php';
  }

  return 'system';
}

export function isToolMutating(tool: AIToolDefinition): boolean {
  if (tool.is_mutating) return true;
  const name = tool.name.toLowerCase();
  return (
    name.startsWith('create_') ||
    name.startsWith('update_') ||
    name.startsWith('delete_') ||
    name.startsWith('restart_') ||
    name.includes('_plan')
  );
}

export function getReadOnlyToolNames(tools: AIToolDefinition[]): string[] {
  return tools.filter((t) => !isToolMutating(t)).map((t) => t.name);
}

export function getMutatingToolNames(tools: AIToolDefinition[]): string[] {
  return tools.filter((t) => isToolMutating(t)).map((t) => t.name);
}
