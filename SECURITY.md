# Security Policy for TameStack

[![Author](https://img.shields.io/badge/author-bespredel.name-8a2be2.svg)](https://bespredel.name)
[![Security EN](https://img.shields.io/badge/Security-EN-blue.svg)](SECURITY.md)
[![Security RU](https://img.shields.io/badge/Security-RU-blue.svg)](SECURITY_RU.md)
[![GitHub license](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

Security is the foundational priority of TameStack, as the platform operates with superuser (`root`) privileges across critical production Linux infrastructure.

---

## 🎯 1. Threat Model

### Attack Vectors & Risks
1. **Man-In-The-Middle (MITM) at SSH Layer**: Spoofing server public keys during the handshake, eavesdropping on credentials or executing unauthorized commands.
2. **Local Storage Compromise**: Unauthorized physical or malware access to the SQLite database file on the operator's machine.
3. **Command Injection**: Malicious shell metacharacter injection via user-supplied input fields (domains, database names, file paths).
4. **Credential Leakage in Logs**: Passwords, tokens, or private keys escaping into application log files or Wails event streams.
5. **Accidental Destructive Operations**: Unintended deletion of production assets, corrupting configuration files, or losing firewall/SSH connectivity.

---

## 🔑 2. SSH Connection Security

### 2.1. Strict SSH Host Key Verification
- **Zero tolerance** for `ssh.InsecureIgnoreHostKey()`.
- Use of a dedicated OpenSSH-formatted `known_hosts` file residing in the application's protected data directory.
- On the first connection to a new server:
  - The SHA-256 fingerprint of the remote public key is calculated.
  - The connection is paused, requiring explicit operator verification and confirmation in the UI.
  - Only after explicit approval is the host key committed to `known_hosts`.
- On any host key mismatch:
  - The connection is **immediately aborted**.
  - A critical MITM warning banner is presented in the UI (Host Key Verification Failed).

### 2.2. Private Key & Passphrase Protection
- Passphrase-protected private keys are decrypted exclusively in transient volatile memory when establishing an SSH session.
- Private keys are never forwarded to third-party endpoints or stored unencrypted on disk.

---

## 🔐 3. Local Secret Storage (Vault)

### 3.1. Cryptographic Standard
All passwords, sudo tokens, API keys, and private SSH keys stored in the local SQLite database are protected with:
- **AES-256-GCM** (Galois/Counter Mode) — providing authenticated encryption with ciphertext integrity guarantees.
- A unique 12-byte cryptographic `Nonce` generated for every individual record via `crypto/rand`.

### 3.2. Master Key Derivation
- A random 32-byte machine key is generated at first launch and stored protected by the OS secret store where available (Windows DPAPI, bound to the user account with application-specific entropy); on other platforms it is kept in a `0600` key file. The vault key is then derived from this machine key and the local salt file via **HKDF-SHA256**.
- Operators can optionally define a custom Master Password; then the vault key is derived via **PBKDF2** (HMAC-SHA256, 200,000 iterations) from the password and the local salt, and secrets are only readable after unlock.

### 3.3. Whole-Database Encrypted Container (optional)
- The operator can enable encryption of the entire database: `TameStack.db.enc` is a chunked container where every 64 KiB chunk is sealed with **AES-256-GCM** under a key derived from the existing vault master password (section 3.2) via **PBKDF2** (HMAC-SHA256, 200,000 iterations) and a random per-container 16-byte salt stored in the container header, so the container key is domain-separated from the vault key even though both come from the same password. Encryption can only be enabled when a master password is already configured; no separate database password exists. Chunk nonces are counter-based from a random per-generation base nonce; the header is authenticated as additional data.
- The master password is then required before the database opens at all: the startup gate decrypts the container into a working copy, and services are wired only after successful unlock. A wrong password is detected by GCM authentication failure of the first chunk; after the database opens, the vault is unlocked transparently with the same password.
- While the application runs, a plaintext working copy exists in the `run\` subdirectory of the protected data directory; it is re-sealed into the container every minute (when changes occurred) and on clean shutdown, after which the working copy is deleted. An unclean shutdown leaves the working copy until the next successful password unlock. The container therefore protects against offline theft of the data directory, not against malware running inside the active user session.

---

## 🛡 4. Defense Against Command Injection

### 4.1. Input Whitelisting & Validation
All values that can reach a remote shell undergo strict whitelist validation first (`internal/ssh/safe.go`), per category rather than per screen:
- **Domain Names**: Validated against RFC 1123 (`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`).
- **Linux & Database Usernames / Database Names**: `^[a-zA-Z_][a-zA-Z0-9_-]{0,31}$`; host tokens (`localhost`, `%`) have their own validator.
- **Service Unit Names, Log Selectors, Firewall Verbs/Protocols/Ports, Chmod Modes, Docker Image References, PHP Versions, Charset Names**: each has a dedicated validator; anything outside the accepted grammar is rejected, never "escaped and run".
- **Fail2ban Jails, Filters & Timing**: Jail identifiers, monitored ports, filter rule names, retry thresholds, time durations (`10m`, `24h`, `7d`), and IP/CIDR addresses in `ignoreip` are validated against strict whitelists before modifying `jail.local` or issuing `fail2ban-client` commands.
- **Cron Schedules & User Contexts**: Crontab intervals and commands undergo syntax verification; cron entries are strictly segregated between system (`root`) and unprivileged user accounts to prevent unauthorized privilege crossing.
- **Filesystem Paths**: Normalized using `filepath.Clean`, rejecting path traversal sequences (`../`), newlines, and non-absolute paths.

AI-supplied tool arguments go through the same validators as GUI input, so a model-crafted value such as `log_type=$(id)` is refused before any command is built.

### 4.2. Safe Shell Argument Escaping
When constructing remote commands:
- Every interpolated value passes through the single escaping function `ssh.ShellEscape` (`internal/ssh/builder.go`), which applies strict POSIX single-quote semantics — `$(...)`, backticks and `$VAR` are inert inside the result. Go's `%q` is not used for shell quoting.
- Callers outside a module cannot hand it arbitrary command strings. Read-only inspection goes through the typed API in `internal/modules/security/readonly.go`: command templates, log-file lists and line limits (max 200 lines per call) are owned by code, and files holding credentials (private keys, `shadow`, `.env`, `master.key`) are excluded from AI reads.
- Administration SQL for MySQL/PostgreSQL is piped to the client over stdin (`ExecStdin`) instead of being embedded in a shell word; identifiers are validated and quoted as SQL identifiers.

### 4.3. Symlink Resolution Before Managed Writes
Before a configuration file is written, the path is resolved on the server with `readlink -f` through `SSHClient.ResolveRemotePath` (`internal/ssh/resolve.go`), and the security policy runs again on the resolved target. A symlink inside an allowed directory (e.g. `/var/www`) therefore cannot redirect a write into a protected location such as `/etc/shadow`.

---

## 🧠 5. AI Tool Execution Gate

- Each registered tool declares a risk level and a mutating flag. `SecurityGate.RequiresHumanApproval` (`internal/ai/policies/security_gate.go`) treats a tool as needing confirmation if it is mutating **or** if its risk level is anything other than `safe` — a mislabelled tool that touches the server still reaches the user.
- The chat loop splits the model's tool calls into auto-run and approval-required batches; only safe, non-mutating read-only inspection tools execute without confirmation.
- Enforcement is not left to the caller's classification: `Registry.Execute` refuses any tool whose definition requires approval, so a tool call crafted to bypass the loop is rejected before its handler runs. The release path is `Registry.ExecuteApproved`.
- Every executed tool writes an audit row (`ai_tool_audit`): tool name, risk level, sanitized arguments, duration, success flag and whether a human approved the run. Refused calls are not recorded as executed.
- Mutating tools (`prepare_config_diff`, `propose_service_restart`) never run inside the chat loop: it holds them back as approval-required. Changes are actually applied as Tasks Engine jobs (`SubmitRequiresApproval`) that the operator releases explicitly. The chat-side pending approval currently has no release path in the UI, so such changes stay blocked instead of running silently.
- **Cloud AI Aliasing & UI Privacy Mode**: When interacting with cloud AI providers, sensitive infrastructure identifiers (server names, IP addresses, domains) are replaced with deterministic aliases before outbound requests and restored locally. Additionally, a global UI Privacy Mode masks sensitive identifiers across the desktop interface for safe screen-sharing.

---

## 🧹 6. Log Scrubbing & Sanitization

An internal log filter (`LogScrubber`) intercepts all Task Engine events before writing to disk or transmitting to the client UI:
- Private SSH keys (`-----BEGIN.*PRIVATE KEY-----`) are automatically masked.
- Key-value pairs containing keywords such as `password`, `secret`, `token`, `passphrase`, or `api_key` are redacted.
- Redacted fields appear in logs as `[REDACTED_SECRET]`.

---

## ⚠️ 7. Destructive Action Safeguards

1. **Audit First / Dry-Run Inspection**:
   When connecting to a server, TameStack initially performs non-mutating inspections only. No packages or configurations are altered without direct user initiation.
2. **Mandatory Configuration Backups**:
   Before modifying any system configuration (`nginx.conf`, `php.ini`, virtual host definitions), a timestamped backup is automatically generated (e.g. `/etc/nginx/sites-available/domain.conf.TameStack.bak.<timestamp>`).
3. **SSH & Firewall Lockout Prevention**:
   When applying firewall (UFW/Firewalld) rules, the active SSH port rule is validated first. When migrating SSH ports, the legacy port remains open until a successful handshake is confirmed on the target port.
4. **Controlled System Upgrades**:
   Unattended or hidden `apt-get dist-upgrade` executions are blocked; operators must review and confirm package diffs before deployment.
5. **Two-Step Destructive Approvals**:
   High-risk actions (deleting websites, dropping databases, removing Docker containers, or rebooting servers) require explicit confirmation via a confirmation modal with manual resource name re-entry.

---

## 🔄 8. Application Self-Update Trust Model

1. **Fixed HTTPS Source Only**:
   Update metadata is fetched exclusively over HTTPS from the GitHub Releases API for this repository (`api.github.com`). No mirrors, no custom endpoints, no user-configurable update URLs.
2. **Mandatory SHA-256 Verification**:
   Before a downloaded archive is executed or installed, its SHA-256 digest is fetched from the GitHub asset API and compared to the archive. If GitHub publishes no digest, or if it does not match, the update is aborted — an unverified binary is never swapped in (fail-closed).
3. **Archive Extraction Hardening**:
   Only the single root-level executable entry is extracted from the release archive; entry names containing paths are ignored, so a malicious archive cannot write outside the update staging directory.
4. **Atomic Binary Swap**:
   The running executable is renamed aside (Windows permits renaming loaded images), the verified binary is moved into place, and the old copy is restored automatically if the swap fails. Leftover old copies are deleted on the next application start.
5. **Explicit UAC Elevation Only When Required**:
   The updater attempts to write inside the application's own directory first without elevation. On Windows, if the installation directory is protected (such as `Program Files`) and not writable by the current user, it triggers an explicit OS UAC prompt to apply the already SHA-256-verified binary — never silent elevation.
6. **Restart Discipline**:
   Relaunch of the updated binary happens only after the shutdown sequence has sealed the encrypted database container and closed all handles, so two instances never contend for the database.
