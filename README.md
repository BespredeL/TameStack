# TameStack - Complex infrastructure. Simple control. SSH Control Panel for Linux Servers

<p align="center">
  <img src="frontend/src/assets/images/logo.png" alt="TameStack Logo" width="128" height="128">
</p>

[![Author](https://img.shields.io/badge/author-bespredel.name-8a2be2.svg)](https://bespredel.name)
[![Readme EN](https://img.shields.io/badge/README-EN-blue.svg)](README.md)
[![Readme RU](https://img.shields.io/badge/README-RU-blue.svg)](README_RU.md)
[![GitHub license](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg?logo=go)](https://go.dev)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000.svg)](https://wails.io)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg?logo=react)](https://react.dev)

🖥 A ready-to-use desktop platform for centralized, secure, and visual Linux server administration via SSH in real time.

**TameStack** eliminates the tedious routine of juggling dozens of open terminal windows and manually executing hundreds of bash commands. It unites the versatility of CLI automation with the clarity, speed, and safety of a modern desktop application built on Go and Wails.

Unlike traditional server control panels, TameStack operates as a **local Desktop Application**. **No background agents, daemons, or extra web ports** are required on your target servers — all communication is performed strictly and securely over native **SSH**.

---

## ✨ Key Features

- 🖥️ **Centralized multi-server fleet:** manage dozens of VPS and dedicated servers (Debian/Ubuntu and RHEL/AlmaLinux/Rocky families) with real-time global resource telemetry (CPU, RAM, storage, network, Load Average, Uptime), connectivity filters (All, Online, Offline), and an Active Server Spotlight with stack details and quick actions.
- 🔐 **Secure credentials vault:** local encryption of secrets (SSH keys, passwords, sudo tokens, AI API keys) with **AES-256-GCM** and **PBKDF2** key derivation, plus an optional Master Password lock.
- 🛡️ **Strict SSH hardening:** Host Key verification via `known_hosts` protecting against MITM attacks, and sensitive data masking in logs (`[REDACTED_SECRET]`).
- 🕵️ **File security scanner:** discovery of recently modified or suspicious files, heuristic detection of web shells, obfuscated PHP, and backdoors, baseline snapshots with instant delta diff, and low-overhead background scanning.
- 🔥 **Firewall & Fail2ban:** UFW / Firewalld management with SSH port protection, granular Fail2ban jail configuration (custom ports, filters, log files, findtime, bantime, ignoreip), dynamic active jail monitoring, and IP unbanning.
- 🤖 **AI DevOps assistant & autonomous agents:** local models (**Ollama**, **llama.cpp**, **vLLM**, **Unsloth**) and cloud providers (**OpenAI**, **Anthropic Claude**, **Google Gemini**), adaptive Model Router, streaming with structured Tool Calling, long-term infrastructure memory on SQLite FTS5, and a Human-in-the-Loop gate requiring explicit approval for mutating operations.
- 🌐 **Sites, web servers & PHP ecosystem:** Nginx and Apache with isolated log viewers, parallel PHP 7.4–8.4+ with availability & OS-compatibility filtering (Installed, Available, Unsupported), per-site PHP-FPM socket switching without restarting global services, FPM pool and extension management, visual `php.ini` editor with `.bak` backups, and a step-by-step site wizard with DNS verification.
- 🗄️ **Databases, Docker & services:** MySQL, MariaDB, PostgreSQL, and Redis (creation, privileges, compressed dumps, restores), full container/network/volume/Compose lifecycle, systemd service controller, process explorer, and socket inspector.
- 📜 **SSL, Cron & backups:** automated Let's Encrypt / ACME issuance and renewal via Certbot, visual Cron scheduler with root & user crontab separation and expression builder, local site and database backup engine with retention rotation.
- 📁 **File manager:** SFTP explorer with breadcrumbs, inline code editor, and secure `run_sudo` execution fallback.
- 💻 **Interactive SSH terminal (PTY):** multi-tabbed terminal with resize handling and full compatibility with CLI utilities (`htop`, `mc`, `nano`).
- 🌍 **Full localization:** 100% bilingual user interface (English and Russian) with instant on-the-fly language switching.

---

## 📸 Screenshots

<p align="center">
  <img src="docs/screenshotes/en/1_en.png" alt="TameStack Dashboard" width="100%">
</p>

<details>
<summary><b>Show all other section screenshots and windows</b></summary>
<br>

### 🖥️ Server Management
| Server Fleet Overview | Add Server over SSH |
| :---: | :---: |
| ![Servers](docs/screenshotes/en/2_en.png) | ![Add Server](docs/screenshotes/en/21_en.png) |

| Real-time Telemetry & Monitoring | Background Task Engine |
| :---: | :---: |
| ![Monitoring](docs/screenshotes/en/15_en.png) | ![Tasks](docs/screenshotes/en/19_en.png) |

### 🛠️ Server Setup Wizard (Provisioning)
| Step 1: Automated Environment Discovery | Step 2: Predefined Stack Profiles |
| :---: | :---: |
| ![Server Discovery](docs/screenshotes/en/3_en.png) | ![Select Profile](docs/screenshotes/en/3a_en.png) |

### 🌐 Web Stack, Sites & PHP
| Virtual Hosts & Sites | Web Servers (Nginx / Apache) |
| :---: | :---: |
| ![Sites](docs/screenshotes/en/4_en.png) | ![Web Servers](docs/screenshotes/en/5_en.png) |

| Multi-version PHP Manager | SSL Certificates (Certbot / Let's Encrypt) |
| :---: | :---: |
| ![PHP Manager](docs/screenshotes/en/6_en.png) | ![SSL Certificates](docs/screenshotes/en/11_en.png) |

### 🗄️ Databases, Docker & Packages
| Database Engines (MySQL / PostgreSQL / Redis) | Docker Containers & Compose |
| :---: | :---: |
| ![Databases](docs/screenshotes/en/9_en.png) | ![Docker](docs/screenshotes/en/10_en.png) |

| APT Package Manager | Systemd Services & Sockets |
| :---: | :---: |
| ![APT Packages](docs/screenshotes/en/7_en.png) | ![Services & Processes](docs/screenshotes/en/8_en.png) |

### 🛡️ Security, File Management & Terminal
| Security Audit, UFW & Fail2ban | Interactive SSH Terminal |
| :---: | :---: |
| ![Security](docs/screenshotes/en/16_en.png) | ![SSH Terminal](docs/screenshotes/en/17_en.png) |

| SFTP File Manager & Code Editor | Automated Backups & Archives |
| :---: | :---: |
| ![File Manager](docs/screenshotes/en/12_en.png) | ![Backups](docs/screenshotes/en/14_en.png) |

### 🤖 DevOps AI Assistant, Cron & Settings
| AI Assistant & Autonomous Agents | Visual Cron Scheduler |
| :---: | :---: |
| ![AI Assistant](docs/screenshotes/en/18_en.png) | ![Cron Jobs](docs/screenshotes/en/13_en.png) |

| Application Settings & Vault | |
| :---: | :---: |
| ![Settings](docs/screenshotes/en/20_en.png) | |

</details>

> [!TIP]
> **No secrets leave your machine:**  
> All credentials live only in the local SQLite database on the operator's machine, encrypted with AES-256-GCM.  
> No agents are installed on target servers — everything runs over plain SSH.  
> Details: [`SECURITY.md`](SECURITY.md).

---

## 🚀 Use Cases

- 🏢 **Web studios and freelancers:** run a fleet of client VPS with Nginx/Apache and PHP sites from a single window.
- 🛠️ **System administrators:** daily server routine (packages, services, cron, firewall, backups) with auditing and safe destructive operations.
- 🤖 **AI-assisted DevOps:** incident diagnosis and change preparation through the AI assistant, with per-mutation approval.
- 🏭 **Infrastructure projects:** server inventory, file integrity auditing, site uptime and SSL certificate monitoring.

---

## 🧠 How It Works

```mermaid
flowchart LR
    A[Operator<br>Desktop UI React/Wails] --> B[TameStack<br>Go core]
    B -- SSH + known_hosts --> C[Linux server<br>root / sudo]
    B --> D[(Local SQLite<br>AES-256-GCM vault)]
    E[AI provider<br>Ollama / OpenAI / Claude / Gemini] -. Tool Calling .-> B
```

1. **Connection:** a server is added over SSH; on first contact the remote key fingerprint must be explicitly verified and pinned to `known_hosts`.
2. **Inspection:** the app reads OS configuration, services, and telemetry using non-mutating commands only.
3. **Task Engine:** long-running operations execute asynchronously with stages, cancellation, and streaming logs in the UI.
4. **Mutations:** configuration changes always create timestamped backups, and destructive actions require two-step confirmation.
5. **AI assistant:** the assistant answers through strict read-only tools and requests approval for every mutating action.

---

## 🛠 Technology Stack

### Backend
- **Go 1.23+**: High performance, strict typing, and minimal memory footprint.
- **Wails v2 (v2.11+)**: Lightweight native desktop runtime without Electron bloat.
- **SQLite3 (modernc.org/sqlite)**: Pure-Go embedded database with WAL mode (CGO-free compilation).
- **golang.org/x/crypto/ssh**: Low-level SSH/SFTP protocol implementation and known_hosts management.
- **AES-256-GCM + PBKDF2**: Local secret encryption at rest.

### Frontend
- **React 18 / TypeScript**: Modular, strictly typed frontend architecture.
- **Vite**: Ultra-fast build tool and developer HMR.
- **Tailwind CSS**: Ergonomic dark-themed design system.
- **Lucide Icons & Recharts**: Clean iconography and vector telemetry charts.

---

## 🏛 Architecture Overview

For threat modeling and the declared cryptography standards, see [SECURITY.md](./SECURITY.md).

```
TameStack/
├── app.go                 # Wails RPC controller bridging Go and JS
├── main.go                # Desktop application entry point
├── wails.json             # Wails configuration manifest
├── internal/              # Go backend core logic
│   ├── config/            # Application paths and settings
│   ├── crypto/            # AES-256-GCM encryption engine
│   ├── db/                # SQLite migrations and repositories
│   ├── ssh/               # Connection pooling, known_hosts, run_sudo
│   ├── osadapter/         # Distro adapters (Debian, RHEL)
│   ├── tasks/             # Asynchronous Task Engine & worker pool
│   ├── ai/                # AI Assistant (Router, Agents, Memory, Tools)
│   └── modules/           # Functional feature modules
│       ├── server/        # Server lifecycle and telemetry
│       ├── site/          # Virtual hosts and isolated logs
│       ├── security/      # File scanner, UFW, Fail2ban
│       ├── php/           # PHP & FPM pool management
│       ├── webserver/     # Nginx & Apache
│       ├── database/      # Database engines (MySQL, Postgres, Redis)
│       ├── docker/        # Containers and Compose stacks
│       ├── ssl/           # ACME / Let's Encrypt automation
│       ├── cron/          # Cron scheduling
│       ├── backup/        # Backup archives and retention
│       ├── files/         # SFTP explorer and editor
│       └── terminal/      # Interactive PTY terminal
└── frontend/              # React + TypeScript SPA
    ├── src/
    │   ├── components/    # Modals and reusable UI elements
    │   ├── views/         # Application screens
    │   ├── i18n/          # Localization dictionaries (EN/RU)
    │   └── context/       # Reactive UI state providers
    └── wailsjs/           # Generated typed Go-JS bindings
```

---

## 📦 Installation & Build

### Prerequisites

- **Go 1.23+**
- **Node.js 18+** & **npm 9+**
- **Wails CLI v2.11+**:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Option 1: Development Mode

```bash
# Clone repository
git clone https://github.com/BespredeL/TameStack.git
cd TameStack

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run desktop application with live reload
wails dev
```

### Option 2: Production Builds

```bash
# Windows (x64) - NSIS installer (builds installer and .exe inside build/bin/)
wails build -platform windows/amd64 -nsis

# Windows (x64) - standard executable
wails build -platform windows/amd64

# Linux (x64)
wails build -platform linux/amd64

# macOS (Universal)
wails build -platform darwin/universal
```

---

## 👨‍💻 Author

**Aleksandr Kireev**

- Website: [bespredel.name](https://bespredel.name)
- Email: [hello@bespredel.name](mailto:hello@bespredel.name)
- GitHub: [@BespredeL](https://github.com/BespredeL)

---

## 🔗 Useful Links

- Security policy (EN): [SECURITY.md](SECURITY.md)
- Security policy (RU): [SECURITY_RU.md](SECURITY_RU.md)
- Stack: [Go](https://go.dev) | [Wails](https://wails.io) | [React](https://react.dev) | [Tailwind CSS](https://tailwindcss.com) | [SQLite](https://www.sqlite.org/)

---

## 📄 License

This project is licensed under the **GNU General Public License v3.0 (GPL-3.0)**. See the [LICENSE](LICENSE) file for details.

---

## ⭐ Support

If this project is useful to you, please support it — drop a **star ⭐** on GitHub!
