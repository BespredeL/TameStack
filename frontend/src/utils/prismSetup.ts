import Prism from 'prismjs';

// Base languages
import 'prismjs/components/prism-clike';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-typescript';
import 'prismjs/components/prism-jsx';
import 'prismjs/components/prism-tsx';
import 'prismjs/components/prism-css';
import 'prismjs/components/prism-markup'; // HTML / XML

// Templating dependency for PHP
import 'prismjs/components/prism-markup-templating';
import 'prismjs/components/prism-php';

// Scripting & Backend
import 'prismjs/components/prism-python';
import 'prismjs/components/prism-bash';
import 'prismjs/components/prism-sql';
import 'prismjs/components/prism-json';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-toml';
import 'prismjs/components/prism-markdown';

// Server Configurations
import 'prismjs/components/prism-ini';
import 'prismjs/components/prism-nginx';
import 'prismjs/components/prism-apacheconf';
import 'prismjs/components/prism-docker';
import 'prismjs/components/prism-systemd';

// Custom / Enhanced Log Tokenizer for TameStack
Prism.languages.log = {
  error: {
    pattern: /\b(?:ERROR|CRITICAL|FATAL|ALERT|EMERG|FAIL|FAILED|FAILURE|ERR|EXCEPTION)\b/i,
    alias: 'important log-error',
  },
  warn: {
    pattern: /\b(?:WARN|WARNING|NOTICE)\b/i,
    alias: 'constant log-warn',
  },
  info: {
    pattern: /\b(?:INFO|SUCCESS|OK|STARTED|READY|RUNNING|LISTENING)\b/i,
    alias: 'keyword log-info',
  },
  debug: {
    pattern: /\b(?:DEBUG|TRACE)\b/i,
    alias: 'comment log-debug',
  },
  date: {
    pattern: /\b(?:\d{4}[-/.]\d{2}[-/.]\d{2}|\d{2}[-/.]\w{3}[-/.]\d{4}|\w{3}\s+\d{1,2})\b(?:[T\s]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)?|\b\d{2}:\d{2}:\d{2}(?:[.,]\d+)?\b/,
    alias: 'number log-date',
  },
  ip: {
    pattern: /\b(?:\d{1,3}\.){3}\d{1,3}\b|\b(?:[a-f0-9]{1,4}:){2,7}[a-f0-9]{1,4}\b/i,
    alias: 'string log-ip',
  },
  method: {
    pattern: /\b(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT)\b/,
    alias: 'function log-method',
  },
  status: {
    pattern: /\b[1-5]\d{2}\b/,
    alias: 'number log-status',
  },
  path: {
    pattern: /(?:[\s"']|^)(?:\/[a-zA-Z0-9_.~%-]+)+/,
    alias: 'attr-name log-path',
  },
  url: {
    pattern: /https?:\/\/[^\s"'<>]+/i,
    alias: 'url',
  },
  string: {
    pattern: /"(?:\\.|[^"\\\r\n])*"|'(?:\\.|[^'\\\r\n])*'/,
    greedy: true,
  },
  number: /\b\d+\b/,
  punctuation: /[()[\]{},;:]/,
};

/**
 * Detect language ID from filename or path.
 */
export function detectLanguage(filePath: string): string {
  if (!filePath) return 'plaintext';
  const lower = filePath.toLowerCase();
  const base = lower.split(/[\\/]/).pop() || '';

  // Config files by name
  if (base === 'nginx.conf' || base.endsWith('.conf') || lower.includes('/nginx/') || lower.includes('nginx')) {
    return 'nginx';
  }
  if (base === '.htaccess' || base === 'apache2.conf' || base === 'httpd.conf' || lower.includes('/apache2/')) {
    return 'apacheconf';
  }
  if (base.endsWith('.ini') || base === 'php.ini' || base === '.env' || base.startsWith('.env.') || base.endsWith('.local')) {
    return 'ini';
  }
  if (base.endsWith('.service') || base.endsWith('.timer') || base.endsWith('.socket')) {
    return 'systemd';
  }
  if (base === 'dockerfile' || base.startsWith('dockerfile.')) {
    return 'docker';
  }
  if (base === 'docker-compose.yml' || base === 'docker-compose.yaml' || base.endsWith('.yml') || base.endsWith('.yaml')) {
    return 'yaml';
  }
  
  // Extension based
  if (base.endsWith('.php') || base.endsWith('.phtml')) return 'php';
  if (base.endsWith('.py')) return 'python';
  if (base.endsWith('.js') || base.endsWith('.mjs') || base.endsWith('.cjs')) return 'javascript';
  if (base.endsWith('.ts')) return 'typescript';
  if (base.endsWith('.jsx')) return 'jsx';
  if (base.endsWith('.tsx')) return 'tsx';
  if (base.endsWith('.json') || base === 'composer.json' || base === 'package.json') return 'json';
  if (base.endsWith('.sql')) return 'sql';
  if (base.endsWith('.sh') || base.endsWith('.bash') || base === '.bashrc' || base === '.profile') return 'bash';
  if (base.endsWith('.html') || base.endsWith('.htm') || base.endsWith('.xml') || base.endsWith('.svg')) return 'markup';
  if (base.endsWith('.css') || base.endsWith('.scss')) return 'css';
  if (base.endsWith('.md') || base.endsWith('.markdown')) return 'markdown';
  if (base.endsWith('.log') || lower.includes('/log/') || lower.includes('/logs/')) return 'log';
  if (base.endsWith('.toml')) return 'toml';

  return 'plaintext';
}

/**
 * Friendly label for language identifier.
 */
export function getLanguageLabel(lang: string): string {
  switch (lang) {
    case 'php': return 'PHP';
    case 'python': return 'Python';
    case 'javascript': return 'JavaScript';
    case 'typescript': return 'TypeScript';
    case 'nginx': return 'Nginx Config';
    case 'apacheconf': return 'Apache Config';
    case 'ini': return 'INI / Config';
    case 'json': return 'JSON';
    case 'yaml': return 'YAML';
    case 'sql': return 'SQL';
    case 'bash': return 'Shell / Bash';
    case 'docker': return 'Dockerfile';
    case 'systemd': return 'Systemd Unit';
    case 'log': return 'Log File';
    case 'markup': return 'HTML / XML';
    case 'css': return 'CSS';
    case 'markdown': return 'Markdown';
    default: return 'Plain Text';
  }
}

/**
 * Highlight code using Prism.
 */
export function highlightCode(code: string, language: string): string {
  if (!code) return '';
  const lang = Prism.languages[language] ? language : 'plaintext';
  if (lang === 'plaintext') {
    // Escape HTML characters
    return code
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }
  try {
    return Prism.highlight(code, Prism.languages[lang], lang);
  } catch {
    return code
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }
}

export default Prism;
