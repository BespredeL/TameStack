/**
 * Utility helper to dispatch global vault unlock requests from any view or component
 * when an operation encounters locked vault credentials.
 */
export const triggerVaultUnlock = (reason?: string) => {
  window.dispatchEvent(
    new CustomEvent('tamestack:vault-locked', {
      detail: {
        reason: reason || 'Vault unlock required',
      },
    })
  );
};
