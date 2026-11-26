import React, { useEffect, useState } from 'react';
import {
  Users,
  Shield,
  ShieldOff,
  Plus,
  Key,
  Lock,
  FolderCog,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
} from 'lucide-react';
import { ServerRecord, TaskRecord } from '../types';
import { useServer } from '../context/ServerContext';
import { useTranslation } from '../i18n/LanguageContext';
import { useConfirm } from '../context/ConfirmContext';
import { getErrorMessage } from '../utils/errors';
import {
  ListSystemUsers,
  CreateSiteUser,
  SetDirectoryOwner,
  GetUserAuthorizedKeys,
  AddUserAuthorizedKey,
  EnableSFTPChroot,
  DisableSFTPChroot,
  SetUserPassword,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Badge, Button, Input, PageToolbar, EmptyState } from '../components/ui';
import { ModalShell } from '../components/common/ModalShell';

interface SystemUserItem {
  username: string;
  uid: number;
  gid: number;
  home_dir: string;
  shell: string;
  is_system: boolean;
  sftp_chroot: boolean;
  chroot_dir?: string;
}

interface AuthorizedKeyItem {
  key_type: string;
  key_data: string;
  comment: string;
  raw: string;
}

interface UsersViewProps {
  servers: ServerRecord[];
  onOpenTask: (task: TaskRecord) => void;
  onOpenAddServer: () => void;
}

export const UsersView: React.FC<UsersViewProps> = ({
  servers,
  onOpenTask,
  onOpenAddServer,
}) => {
  const { selectedServerId, setSelectedServerId } = useServer();
  const { language } = useTranslation();
  const confirm = useConfirm();
  const isEN = language === 'en';

  const [users, setUsers] = useState<SystemUserItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSystem, setShowSystem] = useState(false);

  // Create user modal
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newHomeDir, setNewHomeDir] = useState('');

  // Chroot modal
  const [chrootUser, setChrootUser] = useState<SystemUserItem | null>(null);
  const [chrootDir, setChrootDir] = useState('');
  const [writableSubdir, setWritableSubdir] = useState('htdocs');

  // Password modal
  const [passUser, setPassUser] = useState<string | null>(null);
  const [newPassword, setNewPassword] = useState('');

  // SSH Keys modal
  const [keysUser, setKeysUser] = useState<string | null>(null);
  const [userKeys, setUserKeys] = useState<AuthorizedKeyItem[]>([]);
  const [newPubKey, setNewPubKey] = useState('');

  // Chown modal
  const [chownUser, setChownUser] = useState<string | null>(null);
  const [chownPath, setChownPath] = useState('/var/www/');

  const loadUsers = async () => {
    if (!selectedServerId) return;
    setLoading(true);
    setError(null);
    try {
      const list = await ListSystemUsers(selectedServerId);
      setUsers((list || []) as SystemUserItem[]);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUsers();
  }, [selectedServerId]);

  useEffect(() => {
    const unsub = EventsOn('task:finished', (task: TaskRecord) => {
      if (task && task.server_id === selectedServerId && task.operation_type?.startsWith('user_')) {
        loadUsers();
      }
    });
    return () => {
      if (typeof unsub === 'function') unsub();
    };
  }, [selectedServerId]);

  if (servers.length === 0) {
    return (
      <EmptyState
        icon={<Users className="w-8 h-8 text-indigo-400" />}
        title={isEN ? 'Users & SFTP Isolation' : 'Пользователи и SFTP-изоляция'}
        description={isEN ? 'Add a server to manage isolated site users and SFTP chroot.' : 'Добавьте сервер для управления изолированными пользователями и SFTP chroot.'}
        action={<Button variant="primary" onClick={onOpenAddServer}>{isEN ? 'Add Server' : 'Добавить сервер'}</Button>}
      />
    );
  }

  const filteredUsers = users.filter((u) => showSystem || (!u.is_system && u.uid >= 1000) || u.sftp_chroot || u.username === 'www-data');

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !newUsername.trim()) return;
    try {
      const task = await CreateSiteUser(selectedServerId, newUsername.trim(), newHomeDir.trim());
      setIsCreateOpen(false);
      setNewUsername('');
      setNewHomeDir('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleEnableChroot = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !chrootUser) return;
    const ok = await confirm(
      isEN
        ? `Enable SFTP chroot for "${chrootUser.username}" in "${chrootDir}"? This sets directory owner to root:root (755) and disables interactive shell login.`
        : `Включить SFTP-изоляцию (chroot) для «${chrootUser.username}» в «${chrootDir}»? Владелец каталога станет root:root (755), а интерактивный вход по SSH будет отключён.`,
      { danger: true }
    );
    if (!ok) return;
    try {
      const task = await EnableSFTPChroot(
        selectedServerId,
        chrootUser.username,
        chrootDir.trim(),
        writableSubdir.trim()
      );
      setChrootUser(null);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleDisableChroot = async (username: string) => {
    if (!selectedServerId) return;
    const ok = await confirm(
      isEN
        ? `Disable SFTP chroot isolation for "${username}"?`
        : `Отключить SFTP-изоляцию (chroot) для пользователя «${username}»?`,
      { danger: true }
    );
    if (!ok) return;
    try {
      const task = await DisableSFTPChroot(selectedServerId, username);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !passUser) return;
    const ok = await confirm(
      isEN ? `Update password for user "${passUser}"?` : `Обновить пароль для пользователя «${passUser}»?`,
      { danger: true }
    );
    if (!ok) return;
    try {
      const task = await SetUserPassword(selectedServerId, passUser, newPassword);
      setPassUser(null);
      setNewPassword('');
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const openKeysModal = async (username: string) => {
    if (!selectedServerId) return;
    setKeysUser(username);
    setUserKeys([]);
    try {
      const list = await GetUserAuthorizedKeys(selectedServerId, username);
      setUserKeys((list || []) as AuthorizedKeyItem[]);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleAddKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !keysUser || !newPubKey.trim()) return;
    try {
      const task = await AddUserAuthorizedKey(selectedServerId, keysUser, newPubKey.trim());
      setNewPubKey('');
      setKeysUser(null);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  const handleChown = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedServerId || !chownUser || !chownPath.trim()) return;
    const ok = await confirm(
      isEN
        ? `Recursively change owner of "${chownPath.trim()}" to ${chownUser}:${chownUser}?`
        : `Рекурсивно изменить владельца «${chownPath.trim()}» на ${chownUser}:${chownUser}?`,
      { danger: true }
    );
    if (!ok) return;
    try {
      const task = await SetDirectoryOwner(selectedServerId, chownPath.trim(), chownUser, chownUser);
      setChownUser(null);
      onOpenTask(task as TaskRecord);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    }
  };

  return (
    <div className="page space-y-6">
      <PageToolbar
        servers={servers}
        selectedServerId={selectedServerId}
        onSelectServer={setSelectedServerId}
        actions={
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 text-xs text-slate-400 px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={showSystem}
                onChange={(e) => setShowSystem(e.target.checked)}
                className="rounded border-slate-700 bg-slate-950 text-indigo-500"
              />
              {isEN ? 'Show system' : 'Системные'}
            </label>
            <Button
              variant="secondary"
              size="sm"
              onClick={loadUsers}
              disabled={loading}
              title={isEN ? 'Refresh user list' : 'Обновить список пользователей'}
              aria-label={isEN ? 'Refresh user list' : 'Обновить список пользователей'}
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setIsCreateOpen(true)}
              title={isEN ? 'Create a new isolated site user' : 'Создать нового изолированного пользователя сайта'}
            >
              <Plus className="w-3.5 h-3.5 mr-1.5" />
              {isEN ? 'Create Site User' : 'Создать пользователя'}
            </Button>
          </div>
        }
      />

      {error && (
        <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <div className="bg-slate-900/70 border border-slate-800 rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-950/60 text-slate-400 border-b border-slate-800">
              <tr>
                <th className="px-4 py-3 font-medium">{isEN ? 'User' : 'Пользователь'}</th>
                <th className="px-4 py-3 font-medium">UID / GID</th>
                <th className="px-4 py-3 font-medium">{isEN ? 'Home Directory' : 'Домашний каталог'}</th>
                <th className="px-4 py-3 font-medium">Shell</th>
                <th className="px-4 py-3 font-medium">{isEN ? 'Isolation' : 'Изоляция'}</th>
                <th className="px-4 py-3 font-medium text-right">{isEN ? 'Actions' : 'Действия'}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/70">
              {filteredUsers.map((u) => (
                <tr key={u.username} className="hover:bg-slate-800/30">
                  <td className="px-4 py-3 font-medium text-slate-100">{u.username}</td>
                  <td className="px-4 py-3 font-mono text-slate-400">{u.uid}:{u.gid}</td>
                  <td className="px-4 py-3 font-mono text-slate-300">{u.home_dir}</td>
                  <td className="px-4 py-3 font-mono text-slate-400">{u.shell}</td>
                  <td className="px-4 py-3">
                    {u.sftp_chroot ? (
                      <Badge variant="success" size="sm" className="inline-flex items-center gap-1">
                        <CheckCircle2 className="w-3 h-3" />
                        SFTP Chroot ({u.chroot_dir})
                      </Badge>
                    ) : (
                      <Badge variant="neutral" size="sm">
                        {u.is_system ? (isEN ? 'System' : 'Системный') : (isEN ? 'Standard' : 'Обычный')}
                      </Badge>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {u.username !== 'root' && (
                      <div className="inline-flex items-center gap-1.5">
                        {u.sftp_chroot ? (
                          <Button
                            variant="ghost"
                            size="sm"
                            title={isEN ? 'Disable SFTP Chroot' : 'Отключить SFTP Chroot'}
                            onClick={() => handleDisableChroot(u.username)}
                          >
                            <ShieldOff className="w-3.5 h-3.5 text-amber-400" />
                          </Button>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            title={isEN ? 'Enable SFTP Chroot' : 'Включить SFTP Chroot'}
                            onClick={() => {
                              setChrootUser(u);
                              setChrootDir(u.home_dir || `/var/www/${u.username}`);
                            }}
                          >
                            <Shield className="w-3.5 h-3.5 text-emerald-400" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          title={isEN ? 'SSH Keys' : 'SSH-ключи'}
                          onClick={() => openKeysModal(u.username)}
                        >
                          <Key className="w-3.5 h-3.5 text-indigo-400" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          title={isEN ? 'Set Password' : 'Задать пароль'}
                          onClick={() => {
                            setPassUser(u.username);
                            setNewPassword('');
                          }}
                        >
                          <Lock className="w-3.5 h-3.5 text-cyan-400" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          title={isEN ? 'Set Folder Owner' : 'Назначить владельца каталога'}
                          onClick={() => {
                            setChownUser(u.username);
                            setChownPath(u.home_dir || `/var/www/${u.username}`);
                          }}
                        >
                          <FolderCog className="w-3.5 h-3.5 text-slate-300" />
                        </Button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
              {filteredUsers.length === 0 && !loading && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-slate-500">
                    {isEN ? 'No users found' : 'Пользователи не найдены'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {isCreateOpen && (
        <ModalShell
          onClose={() => setIsCreateOpen(false)}
          title={isEN ? 'Create Isolated Site User' : 'Создать пользователя сайта'}
          icon={<Users className="w-5 h-5" />}
        >
          <form onSubmit={handleCreateUser}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Username' : 'Имя пользователя'}</label>
                <Input value={newUsername} onChange={(e) => setNewUsername(e.target.value)} placeholder="site_example" required />
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Home directory (optional)' : 'Домашний каталог (необязательно)'}</label>
                <Input value={newHomeDir} onChange={(e) => setNewHomeDir(e.target.value)} placeholder="/var/www/site_example" />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setIsCreateOpen(false)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Create' : 'Создать'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {chrootUser && (
        <ModalShell
          onClose={() => setChrootUser(null)}
          title={isEN ? `Enable SFTP Chroot: ${chrootUser.username}` : `SFTP-изоляция: ${chrootUser.username}`}
          icon={<Shield className="w-5 h-5 text-emerald-400" />}
        >
          <form onSubmit={handleEnableChroot}>
            <div className="p-6 space-y-4">
              <p className="text-xs text-slate-400">
                {isEN
                  ? 'OpenSSH requires the chroot directory to be owned by root:root (755). A writable subdirectory will be created inside for the user.'
                  : 'OpenSSH требует, чтобы chroot-каталог принадлежал root:root (755). Внутри будет создан доступный для записи подкаталог пользователя.'}
              </p>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Chroot directory' : 'Chroot-каталог'}</label>
                <Input value={chrootDir} onChange={(e) => setChrootDir(e.target.value)} required />
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Writable subdirectory' : 'Подкаталог с правами записи'}</label>
                <Input value={writableSubdir} onChange={(e) => setWritableSubdir(e.target.value)} placeholder="htdocs" />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setChrootUser(null)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Enable Chroot' : 'Включить Chroot'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {passUser && (
        <ModalShell
          onClose={() => setPassUser(null)}
          title={isEN ? `Set Password: ${passUser}` : `Задать пароль: ${passUser}`}
          icon={<Lock className="w-5 h-5 text-cyan-400" />}
        >
          <form onSubmit={handleSetPassword}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'New Password (min 8 chars)' : 'Новый пароль (мин. 8 символов)'}</label>
                <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} minLength={8} required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setPassUser(null)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Save' : 'Сохранить'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {keysUser && (
        <ModalShell
          onClose={() => setKeysUser(null)}
          title={isEN ? `SSH Keys: ${keysUser}` : `SSH-ключи: ${keysUser}`}
          icon={<Key className="w-5 h-5 text-indigo-400" />}
        >
          <form onSubmit={handleAddKey}>
            <div className="p-6 space-y-4">
              <div className="max-h-48 overflow-y-auto space-y-2">
                {userKeys.length === 0 ? (
                  <p className="text-xs text-slate-500">{isEN ? 'No authorized keys found.' : 'Ключи не найдены.'}</p>
                ) : (
                  userKeys.map((k, idx) => (
                    <div key={idx} className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 break-all">
                      <span className="text-indigo-400">{k.key_type}</span> {k.key_data.slice(0, 32)}... {k.comment}
                    </div>
                  ))
                )}
              </div>
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Add Public Key' : 'Добавить публичный ключ'}</label>
                <Input value={newPubKey} onChange={(e) => setNewPubKey(e.target.value)} placeholder="ssh-ed25519 AAAA..." required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setKeysUser(null)}>{isEN ? 'Close' : 'Закрыть'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Add Key' : 'Добавить ключ'}</Button>
            </div>
          </form>
        </ModalShell>
      )}

      {chownUser && (
        <ModalShell
          onClose={() => setChownUser(null)}
          title={isEN ? `Set Directory Owner: ${chownUser}` : `Назначить владельца: ${chownUser}`}
          icon={<FolderCog className="w-5 h-5" />}
        >
          <form onSubmit={handleChown}>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-xs text-slate-400 mb-1.5">{isEN ? 'Directory path' : 'Путь к каталогу'}</label>
                <Input value={chownPath} onChange={(e) => setChownPath(e.target.value)} required />
              </div>
            </div>
            <div className="px-6 py-3.5 border-t border-slate-800 bg-slate-950/80 flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setChownUser(null)}>{isEN ? 'Cancel' : 'Отмена'}</Button>
              <Button type="submit" variant="primary">{isEN ? 'Apply' : 'Применить'}</Button>
            </div>
          </form>
        </ModalShell>
      )}
    </div>
  );
};
