import { useEffect, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { GetVaultStatus, GetBootState, LockVault } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { VaultRequireUnlockPayload } from '../types';

interface VaultGateOptions {
  fetchData: () => void;
  t: (path: string, params?: Record<string, string | number>) => string;
}

export const useVaultGate = ({ fetchData, t }: VaultGateOptions) => {
  const [vaultStatus, setVaultStatus] = useState<main.VaultStatusInfo | null>(null);
  const [isVaultChecking, setIsVaultChecking] = useState(true);
  const [isVaultUnlockOpen, setIsVaultUnlockOpen] = useState(false);
  const [vaultUnlockReason, setVaultUnlockReason] = useState<string | null>(null);

  // Encrypted database container boot gate
  const [isDbGate, setIsDbGate] = useState(false);
  const [dbSealError, setDbSealError] = useState<string | null>(null);

  const checkVaultStatus = async (): Promise<main.VaultStatusInfo | null> => {
    try {
      const vStatus = await GetVaultStatus();
      setVaultStatus(vStatus);
      return vStatus;
    } catch (e) {
      console.warn('Failed to get vault status:', e);
      return null;
    } finally {
      setIsVaultChecking(false);
    }
  };

  const handleLockVault = async () => {
    try {
      await LockVault();
      setVaultUnlockReason(null);
      await checkVaultStatus();
    } catch (e) {
      console.error('Failed to lock vault:', e);
    }
  };

  useEffect(() => {
    const initApp = async () => {
      // Encrypted database container: the backend has no data until the
      // master password decrypts it, so gate before any data RPC.
      // Fail closed — without a confirmed "plain" boot state the backend would
      // only answer data RPCs with a locked-database error.
      let bootLocked = true;
      try {
        const boot = await GetBootState();
        bootLocked = !boot || boot.db_locked || boot.mode !== 'plain';
      } catch (e) {
        console.warn('Failed to get boot state:', e);
      }
      if (bootLocked) {
        setIsDbGate(true);
        setIsVaultChecking(false);
        return;
      }
      const vStatus = await checkVaultStatus();
      // If master password is set and vault is locked, do NOT load data yet!
      // The application will show the blocking Lock Screen until master password is typed.
      if (vStatus && vStatus.has_master_password && vStatus.is_locked) {
        return;
      }
      fetchData();
    };

    initApp();

    // Vault Real-time event listeners
    const unVaultRequire = EventsOn('vault:require_unlock', (data: VaultRequireUnlockPayload) => {
      setVaultUnlockReason(data?.reason || t('vault.unlockReason'));
      setIsVaultUnlockOpen(true);
    });

    const unVaultLocked = EventsOn('vault:locked', async () => {
      await checkVaultStatus();
    });

    const unVaultUnlocked = EventsOn('vault:unlocked', async () => {
      await checkVaultStatus();
      fetchData();
    });

    const unSealFailed = EventsOn('db:seal_failed', (data: { error?: string }) => {
      setDbSealError(data?.error || 'unknown error');
    });

    // Custom DOM event listener for vault unlock requests from frontend components
    const handleCustomVaultLocked = (e: Event) => {
      const detail = (e as CustomEvent<{ reason?: string }>).detail;
      setVaultUnlockReason(detail?.reason || t('vault.unlockReason'));
      setIsVaultUnlockOpen(true);
    };
    window.addEventListener('tamestack:vault-locked', handleCustomVaultLocked);

    return () => {
      if (typeof unVaultRequire === 'function') unVaultRequire();
      else EventsOff('vault:require_unlock');

      if (typeof unVaultLocked === 'function') unVaultLocked();
      else EventsOff('vault:locked');

      if (typeof unVaultUnlocked === 'function') unVaultUnlocked();
      else EventsOff('vault:unlocked');

      if (typeof unSealFailed === 'function') unSealFailed();
      else EventsOff('db:seal_failed');

      window.removeEventListener('tamestack:vault-locked', handleCustomVaultLocked);
    };
  }, []);

  return {
    vaultStatus,
    isVaultChecking,
    isVaultUnlockOpen,
    setIsVaultUnlockOpen,
    vaultUnlockReason,
    setVaultUnlockReason,
    isDbGate,
    setIsDbGate,
    dbSealError,
    checkVaultStatus,
    handleLockVault,
  };
};