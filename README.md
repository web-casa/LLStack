# LLStack Panel

Open-source server control panel built on [LiteHttpd](https://rpms.litehttpd.com) (OpenLiteSpeed with Apache .htaccess compatibility).

## Features

- **Web Server**: LiteHttpd — OLS performance + 80 .htaccess directives
- **PHP**: Multi-version via REMI (7.4–8.5), php-litespeed SAPI (not php-fpm)
- **Databases**: MariaDB / MySQL / Percona / PostgreSQL (official repos)
- **Redis**: Per-user isolated instances (Unix socket, systemd template)
- **Sites**: Virtual host management, SSL (Let's Encrypt), .htaccess editor
- **Security**: ALTCHA PoW login, 2FA/TOTP, LSBruteForce visual config
- **File Manager**: Web file browser + CodeMirror editor (9 languages)
- **Web Terminal**: xterm.js + WebSocket PTY
- **Config Optimizer**: Auto-tune PHP/DB/Redis based on server resources
- **Setup Wizard**: First-login guided installation with streaming logs
- **Multi-user**: RBAC + plans (PHP/Redis/site quotas)
- **Traffic Analytics**: Access log parsing, hourly charts, top pages/IPs
- **Site Clone**: Full clone with WordPress domain replacement (staging mode)
- **Remote Backup**: S3 / SFTP push support
- **Notifications**: Email (SMTP) + Webhook (Slack/DingTalk/Feishu)
- **PageSpeed**: Google PageSpeed Insights integration
- **API Docs**: Auto-generated endpoint documentation
- **i18n**: 简体中文 / 繁體中文 / English, dark/light theme, accent color customization

## Requirements

- **OS**: AlmaLinux / Rocky Linux / CentOS Stream / RHEL — **EL9 or EL10**
- **RAM**: 1GB minimum, 2GB+ recommended
- **Disk**: 5GB minimum

## Install

```bash
curl -sSL https://install.llstack.com | bash
```

Or from source:

```bash
git clone https://github.com/llstack/llstack-panel.git /opt/llstack-panel
bash /opt/llstack-panel/scripts/install.sh
```

After install, open `https://<your-ip>:30333` to create your admin account and run the setup wizard.

## Architecture

```
Browser → LiteHttpd :30333 (HTTPS)
            ├── /api/*  → gunicorn :8001 (Flask)
            ├── /ws/*   → gunicorn :8001 (WebSocket)
            └── /*      → dist/ (static React)
```

- **Backend**: Python 3.12 + Flask, SQLite (WAL mode)
- **Frontend**: React 19 + Radix UI + Vite (pre-built, no Node.js on server)
- **Scripts**: 38 shell scripts for system operations via sudoers

## Development

```bash
# Backend
cd backend
python3.12 -m venv .venv && .venv/bin/pip install -r requirements.txt
LLSTACK_DB_PATH=/tmp/dev.db .venv/bin/python wsgi.py

# Frontend
cd web
npm install && npm run dev  # dev server with API proxy

# Tests (52 tests)
cd backend && .venv/bin/pytest tests/ -v

# Build for production
cd web && npm run build  # output in dist/
```

## Project Structure

```
backend/           Flask API (21 modules)
  app/api/         Auth, Sites, PHP, DB, Redis, Firewall, Cron, Backup,
                   Monitoring, Users, AppStore, Files, Terminal,
                   Htaccess, Migration, Optimizer, Setup Wizard, etc.
  app/utils/       Shell runner, ALTCHA, validators, audit, task runner
  tests/           pytest (52 tests)
web/               React frontend
  src/pages/       23 pages
  src/components/  Shared components (StreamingLog, etc.)
  dist/            Pre-built production bundle
scripts/           38 shell scripts
  site/            Site create/delete
  php/             PHP install/uninstall/config
  db/              DB create/delete/export + engine installers
  redis/           Per-user instance management
  ssl/             Let's Encrypt via acme.sh
  install.sh       Server installer
  upgrade.sh       Panel updater
config/            LiteHttpd vhost templates
docs4ai/           Developer documentation
```

## License

GPLv3
