import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { useServer } from './ServerContext';
import { ListSites } from '../../wailsjs/go/main/App';

interface PrivacyContextValue {
  isPrivacyMode: boolean;
  togglePrivacyMode: () => void;
  setPrivacyMode: (enabled: boolean) => void;
  maskServerName: (name: string) => string;
  maskHost: (host: string) => string;
  maskDomain: (domain: string) => string;
  maskUser: (username: string) => string;
  maskPath: (path: string) => string;
  maskDatabase: (name: string) => string;
  maskText: (text: string) => string;
  registerDomains: (domains: string[]) => void;
}

const STORAGE_KEY = 'tamestack_privacy_mode';

const PrivacyContext = createContext<PrivacyContextValue>({
  isPrivacyMode: false,
  togglePrivacyMode: () => {},
  setPrivacyMode: () => {},
  maskServerName: (v) => v,
  maskHost: (v) => v,
  maskDomain: (v) => v,
  maskUser: (v) => v,
  maskPath: (v) => v,
  maskDatabase: (v) => v,
  maskText: (v) => v,
  registerDomains: () => {},
});

const standardUsers = new Set([
  'root',
  'www-data',
  'nobody',
  'daemon',
  'ubuntu',
  'admin',
  'postgres',
  'debian-sys-maint',
  'mysql.sys',
  'mysql.session',
  'mysql.infoschema',
  'syslog',
  'nginx',
  'apache',
  'caddy',
]);

const systemDatabases = new Set([
  'information_schema',
  'mysql',
  'performance_schema',
  'sys',
  'postgres',
  'template0',
  'template1',
]);

const standardPathSegments = new Set([
  'var',
  'www',
  'home',
  'srv',
  'opt',
  'etc',
  'usr',
  'local',
  'root',
  'tmp',
  'public',
  'public_html',
  'private',
  'html',
  'htdocs',
  'httpdocs',
  'web',
  'app',
  'current',
  'releases',
  'shared',
  'dist',
  'build',
  'data',
  'vhosts',
  'sites',
  'sites-available',
  'sites-enabled',
  'conf',
  'conf.d',
  'nginx',
  'apache2',
  'httpd',
  'caddy',
  'php',
  'fpm',
  'pool.d',
  'log',
  'logs',
  'ssl',
  'certs',
  'letsencrypt',
  'live',
  'archive',
  'bin',
  'sbin',
  'lib',
  'share',
  'storage',
  'bootstrap',
  'cache',
  'vendor',
  'node_modules',
  'cron.d',
  'crontab',
  'sftp',
  'chroot',
  'uploads',
  'files',
  'backup',
  'backups',
]);

const standardFileNames = new Set([
  'nginx.conf',
  'apache2.conf',
  'httpd.conf',
  '.htaccess',
  'access.log',
  'error.log',
  'php.ini',
  'artisan',
  'index.php',
  'index.html',
  'composer.json',
  'package.json',
  'docker-compose.yml',
  'docker-compose.yaml',
  '.env',
]);

const ipv4Regex = /\b(?:(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\b/g;

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function shortTag(input: string): string {
  let h = 2166136261;
  for (let i = 0; i < input.length; i++) {
    h ^= input.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return ((h >>> 0) % 900 + 100).toString();
}

function isAlreadyMaskedSegment(seg: string): boolean {
  return (
    seg.startsWith('site-') ||
    seg.startsWith('user-') ||
    seg.startsWith('server-') ||
    seg.startsWith('host-') ||
    seg.startsWith('db-') ||
    seg.startsWith('dir-') ||
    seg.startsWith('198.51.100.')
  );
}

export const PrivacyProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { servers } = useServer();
  const [isPrivacyMode, setIsPrivacyMode] = useState<boolean>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === 'true';
    } catch {
      return false;
    }
  });
  const [knownDomains, setKnownDomains] = useState<string[]>([]);

  useEffect(() => {
    if (!isPrivacyMode) return;
    let cancelled = false;
    ListSites('')
      .then((sites) => {
        if (cancelled || !Array.isArray(sites)) return;
        const doms = new Set<string>();
        sites.forEach((s) => {
          if (s.domain) doms.add(s.domain.trim());
          if (Array.isArray(s.aliases)) {
            s.aliases.forEach((a) => {
              if (a && a.trim()) doms.add(a.trim());
            });
          }
        });
        setKnownDomains((prev) => {
          const merged = new Set([...prev, ...doms]);
          return Array.from(merged);
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [isPrivacyMode, servers]);

  const registerDomains = useCallback((domains: string[]) => {
    const cleaned = domains.map((d) => d.trim()).filter(Boolean);
    if (cleaned.length === 0) return;
    setKnownDomains((prev) => {
      const set = new Set(prev);
      let changed = false;
      for (const d of cleaned) {
        if (!set.has(d)) {
          set.add(d);
          changed = true;
        }
      }
      return changed ? Array.from(set) : prev;
    });
  }, []);

  const setPrivacyMode = useCallback((enabled: boolean) => {
    setIsPrivacyMode(enabled);
    try {
      localStorage.setItem(STORAGE_KEY, String(enabled));
    } catch {}
  }, []);

  const togglePrivacyMode = useCallback(() => {
    setPrivacyMode(!isPrivacyMode);
  }, [isPrivacyMode, setPrivacyMode]);

  const maps = useMemo(() => {
    const serverNames = new Map<string, string>();
    const hosts = new Map<string, string>();
    const users = new Map<string, string>();
    const domains = new Map<string, string>();

    servers.forEach((s, idx) => {
      const n = idx + 1;
      if (s.name && s.name.trim()) {
        serverNames.set(s.name.trim(), `server-${n}`);
      }
      if (s.host && s.host.trim()) {
        const h = s.host.trim();
        if (/^\d+\.\d+\.\d+\.\d+$/.test(h)) {
          hosts.set(h, `198.51.100.${n}`);
        } else {
          hosts.set(h, `host-${n}.example.test`);
        }
      }
      if (s.username && s.username.trim() && !standardUsers.has(s.username.trim().toLowerCase())) {
        const u = s.username.trim();
        if (!users.has(u)) {
          users.set(u, `user-${users.size + 1}`);
        }
      }
    });

    knownDomains.forEach((d) => {
      if (!domains.has(d)) {
        domains.set(d, `site-${domains.size + 1}.example.test`);
      }
    });

    // Build sorted replacement pairs (longest first to avoid partial match collisions).
    const pairs: Array<[string, string]> = [];
    serverNames.forEach((alias, orig) => pairs.push([orig, alias]));
    hosts.forEach((alias, orig) => pairs.push([orig, alias]));
    domains.forEach((alias, orig) => pairs.push([orig, alias]));
    users.forEach((alias, orig) => pairs.push([orig, alias]));
    pairs.sort((a, b) => b[0].length - a[0].length);

    return { serverNames, hosts, users, domains, pairs };
  }, [servers, knownDomains]);

  const maskServerName = useCallback(
    (name: string) => {
      if (!isPrivacyMode || !name) return name;
      return maps.serverNames.get(name.trim()) || 'server-masked';
    },
    [isPrivacyMode, maps]
  );

  const maskHost = useCallback(
    (host: string) => {
      if (!isPrivacyMode || !host) return host;
      const trimmed = host.trim();
      if (maps.hosts.has(trimmed)) return maps.hosts.get(trimmed)!;
      if (/^\d+\.\d+\.\d+\.\d+$/.test(trimmed)) return '198.51.100.99';
      return 'host-masked.example.test';
    },
    [isPrivacyMode, maps]
  );

  const maskDomain = useCallback(
    (domain: string) => {
      if (!isPrivacyMode || !domain) return domain;
      const trimmed = domain.trim();
      if (maps.domains.has(trimmed)) return maps.domains.get(trimmed)!;
      return `site-${shortTag(trimmed)}.example.test`;
    },
    [isPrivacyMode, maps]
  );

  const maskUser = useCallback(
    (username: string) => {
      if (!isPrivacyMode || !username) return username;
      const trimmed = username.trim();
      if (standardUsers.has(trimmed.toLowerCase())) return trimmed;
      return maps.users.get(trimmed) || `user-${shortTag(trimmed)}`;
    },
    [isPrivacyMode, maps]
  );

  const maskDatabase = useCallback(
    (name: string) => {
      if (!isPrivacyMode || !name) return name;
      const trimmed = name.trim();
      if (systemDatabases.has(trimmed.toLowerCase())) return trimmed;
      return `db-${shortTag(trimmed)}`;
    },
    [isPrivacyMode]
  );

  const maskText = useCallback(
    (text: string) => {
      if (!isPrivacyMode || !text) return text;
      let out = text;

      for (const [orig, alias] of maps.pairs) {
        if (orig.length < 3) continue;
        const re = new RegExp(escapeRegex(orig), 'gi');
        out = out.replace(re, alias);
      }

      // Mask /home/<user> and /var/www/<project> segments inside arbitrary text/commands.
      out = out.replace(/(\/(?:home|var\/www)\/)([^/\s'"]+)/gi, (_m, prefix: string, seg: string) => {
        if (standardPathSegments.has(seg.toLowerCase()) || isAlreadyMaskedSegment(seg)) {
          return `${prefix}${seg}`;
        }
        return `${prefix}dir-${shortTag(seg)}`;
      });

      // Mask any remaining public IPv4 addresses (keep loopback/unspecified/already-masked TEST-NET-2).
      let extraIpIdx = maps.hosts.size + 1;
      const extraIps = new Map<string, string>();
      out = out.replace(ipv4Regex, (ip) => {
        if (ip === '127.0.0.1' || ip === '0.0.0.0' || ip.startsWith('198.51.100.')) {
          return ip;
        }
        if (!extraIps.has(ip)) {
          extraIps.set(ip, `198.51.100.${Math.min(254, extraIpIdx++)}`);
        }
        return extraIps.get(ip)!;
      });

      return out;
    },
    [isPrivacyMode, maps]
  );

  const maskPath = useCallback(
    (path: string) => {
      if (!isPrivacyMode || !path) return path;
      const preMasked = maskText(path);
      return preMasked
        .split('/')
        .map((seg) => {
          if (!seg || seg === '.' || seg === '..') return seg;
          const lower = seg.toLowerCase();
          if (standardPathSegments.has(lower) || standardFileNames.has(lower) || isAlreadyMaskedSegment(seg)) {
            return seg;
          }
          const extMatch = seg.match(/^(.+?)(\.(?:conf|vhost|log|crt|pem|key|sock|pid))$/i);
          if (extMatch) {
            const stem = extMatch[1];
            const ext = extMatch[2];
            if (isAlreadyMaskedSegment(stem)) return seg;
            return `dir-${shortTag(stem)}${ext}`;
          }
          return `dir-${shortTag(seg)}`;
        })
        .join('/');
    },
    [isPrivacyMode, maskText]
  );

  const value = useMemo(
    () => ({
      isPrivacyMode,
      togglePrivacyMode,
      setPrivacyMode,
      maskServerName,
      maskHost,
      maskDomain,
      maskUser,
      maskPath,
      maskDatabase,
      maskText,
      registerDomains,
    }),
    [
      isPrivacyMode,
      togglePrivacyMode,
      setPrivacyMode,
      maskServerName,
      maskHost,
      maskDomain,
      maskUser,
      maskPath,
      maskDatabase,
      maskText,
      registerDomains,
    ]
  );

  return <PrivacyContext.Provider value={value}>{children}</PrivacyContext.Provider>;
};

export const usePrivacy = () => useContext(PrivacyContext);
