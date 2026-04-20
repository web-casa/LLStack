<div align="center">
  <img src="logo/logo.svg" alt="LLStack" width="96" />
  <h1>LLStack Panel</h1>
  <p><strong>Open-source server control panel built on LiteHttpd</strong></p>
  <p>Apache-level <code>.htaccess</code> compatibility · LiteSpeed-level performance</p>
  <p>
    <a href="https://llstack.com">Website &amp; Docs</a> ·
    <a href="https://demo.llstack.com">Live Demo</a> ·
    <a href="https://llstack.com/reference/changelog/">Changelog</a> ·
    <a href="README_CN.md">简体中文</a> ·
    <a href="README_TW.md">繁體中文</a>
  </p>
</div>

---

## Preview

<div align="center">
  <img src="snapshots/001.png" alt="Dashboard" width="90%" />
  <img src="snapshots/002.png" alt="Site management" width="90%" />
  <img src="snapshots/003.png" alt="WordPress toolkit" width="90%" />
</div>

## One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/web-casa/LLStack/main/scripts/install.sh | sudo bash
```

After the installer finishes, open `https://<your-server-ip>:30333` to create the admin account.

**Requirements**: AlmaLinux / Rocky Linux / CentOS Stream 9 or 10 · 1 GB RAM · 5 GB disk

## Features

### Core services

| Area | Highlights |
|------|------------|
| **LiteHttpd engine** | OpenLiteSpeed + 80 `.htaccess` directives, ~2.5× Apache throughput |
| **Multi-version PHP** | PHP 7.4 – 8.5 via REMI, `php-litespeed` SAPI (not php-fpm) |
| **Databases** | MariaDB / MySQL / Percona / PostgreSQL — import, export, clone, maintenance, Adminer SSO |
| **Redis** | Per-user isolated instances, object cache, ACL (6.0+) |
| **SSL certificates** | Let's Encrypt issue/renew, manual upload, force HTTPS |

### WordPress toolkit (24 API endpoints)

- One-click install, plugin/theme management, passwordless SSO
- **Wordfence CVE scan** — 33,000+ vulnerability records with CVSS scoring
- Smart Update (clone → test → apply) + scheduled auto-updates
- Redis object cache integration
- Automatic rollback on failed updates

### Operations

| Area | Highlights |
|------|------------|
| **Staging** | One-click clone, Push / Pull (files / DB / both), automatic domain rewrite |
| **Incremental backup** | restic dedup + AES-256, 1h–24h schedule, selective restore |
| **CDN** | Cloudflare one-click setup, cache purge |
| **Monitoring** | CPU/memory/disk, Redis trends, cgroup pressure |

### Security

- **RBAC** — 4 roles: Owner / Admin / Developer / Viewer
- **ALTCHA** — proof-of-work brute-force protection (no CAPTCHA)
- **2FA/TOTP** — Google Authenticator / Authy compatible
- JWT auth, audit log, per-site `disable_functions`
- Plan-based quotas (site count, DB count, disk)

### Extras

- File manager + xterm.js web terminal (WebSocket PTY)
- Cron jobs, firewalld rules, log rotation
- One-click Apache migration (`litehttpd-confconv`)
- App store (WordPress / Laravel / Typecho)
- Full i18n: 简体中文 / 繁體中文 / English

## Architecture

```
Browser → LiteHttpd :30333 (HTTPS)
            ├── /api/*  → gunicorn :8001 (Flask + SQLite)
            ├── /ws/*   → gunicorn :8001 (WebSocket terminal)
            └── /*      → dist/ (React 19 + Radix UI)
```

- **Backend**: Python 3.12 + Flask, SQLite (WAL mode), 245+ pytest tests
- **Frontend**: React 19 + Radix UI + Vite (pre-built, no Node.js on server)
- **Scripts**: 43 shell scripts for system operations, invoked via sudoers

## Documentation

Full documentation lives at **[llstack.com](https://llstack.com)**:

- [IT admin guide](https://llstack.com/guide/for-admins/) — install, configure, operate
- [Site user guide](https://llstack.com/guide/for-users/) — sites, WordPress, databases
- [Developer guide](https://llstack.com/guide/for-developers/) — full-stack workflow, API reference
- [LiteHttpd engine](https://llstack.com/guide/litehttpd/) — `.htaccess` compatibility, benchmarks
- [FAQ](https://llstack.com/reference/faq/)

## Performance

| Metric | Apache 2.4 | Stock OLS | LiteHttpd |
|--------|:----------:|:---------:|:---------:|
| Static RPS | 23,909 | 63,140 | **58,891** |
| PHP RPS (wp-login) | 274 | 258 | **292** |
| Memory | 818 MB | 654 MB | **689 MB** |
| `.htaccess` compat | 10/10 | 6/10 | **10/10** |

*Environment: Linode 4C/8G, EL9, PHP 8.3, MariaDB 10.11*

## Related projects

- [LiteHttpd](https://litehttpd.com) — a lightweight web server with first-class Apache compatibility
- [WebCasa](https://web.casa) — an AI-native open-source server control panel

## License

GPLv3 — see [LICENSE](LICENSE)
