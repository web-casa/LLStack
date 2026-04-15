<div align="center">
  <h1>LLStack</h1>
  <p><strong>Open-source server management panel powered by LiteHttpd</strong></p>
  <p>Apache-level .htaccess compatibility · LiteSpeed-level performance</p>
  <p>
    <a href="https://llstack.com">Documentation</a> ·
    <a href="https://llstack.com/guide/getting-started/">Quick Start</a> ·
    <a href="https://llstack.com/reference/changelog/">Changelog</a>
  </p>
</div>

---

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/web-casa/LLStack/main/scripts/install.sh | sudo bash
```

After installation, open `https://your-server-ip:30333` to create your admin account.

**Requirements**: AlmaLinux / Rocky Linux / CentOS Stream 9 or 10 · 1GB RAM · 5GB disk

## Features

### Core

| Feature | Description |
|---------|-------------|
| **LiteHttpd** | OpenLiteSpeed + 80 .htaccess directives, 2.5x Apache performance |
| **Multi-PHP** | PHP 7.4 ~ 8.4 via REMI, php-litespeed SAPI (not php-fpm) |
| **Databases** | MariaDB / PostgreSQL, import/export/clone/maintenance, Adminer SSO |
| **Redis** | Per-user instances, Object Cache, ACL management (6.0+) |
| **SSL** | Let's Encrypt auto-issue & renewal, manual upload, force HTTPS |

### WordPress Toolkit (24 API endpoints)

- One-click install, plugin/theme management, SSO login
- **Wordfence CVE scanning** — 33,000+ vulnerabilities, CVSS scoring
- Smart Update (clone → test → apply) + auto-update scheduling
- Redis Object Cache integration
- Auto-rollback on failure

### Operations

| Feature | Description |
|---------|-------------|
| **Staging** | One-click clone, Push/Pull (files/database/all), domain auto-replace |
| **Incremental Backup** | restic dedup + AES-256 encryption, 1h-24h scheduling, selective restore |
| **CDN** | Cloudflare integration, one-click cache purge |
| **Monitoring** | CPU/memory/disk, Redis trends, cgroup pressure |

### Security

- **RBAC** — 4 roles: Owner / Admin / Developer / Viewer
- **ALTCHA** — Proof-of-Work anti-brute-force (no CAPTCHA)
- **2FA** — TOTP with Google Authenticator / Authy
- JWT authentication, audit logging, per-site `disable_functions`
- Plan quotas (max sites, max databases, disk quota)

### More

- File manager + WebSocket terminal (xterm.js)
- Cron jobs, firewall (firewalld), log rotation
- Apache migration (litehttpd-confconv)
- App store (WordPress / Laravel / Typecho)
- i18n: 简体中文 / 繁體中文 / English

## Architecture

```
Browser → LiteHttpd :30333 (HTTPS)
            ├── /api/*  → gunicorn :8001 (Flask + SQLite)
            ├── /ws/*   → gunicorn :8001 (WebSocket terminal)
            └── /*      → dist/ (React 19 + Radix UI)
```

## Documentation

Full documentation available at **[llstack.com](https://llstack.com)**

- [IT Admin Guide](https://llstack.com/guide/for-admins/) — Installation, configuration, operations
- [Site User Guide](https://llstack.com/guide/for-users/) — Managing sites, WordPress, databases
- [Developer Guide](https://llstack.com/guide/for-developers/) — Full-stack workflow, API reference
- [LiteHttpd Engine](https://llstack.com/guide/litehttpd/) — .htaccess compatibility, benchmarks
- [FAQ](https://llstack.com/reference/faq/)

## Performance

| Metric | Apache 2.4 | Stock OLS | LiteHttpd |
|--------|:----------:|:---------:|:---------:|
| Static RPS | 23,909 | 63,140 | **58,891** |
| PHP RPS (wp-login) | 274 | 258 | **292** |
| Memory | 818 MB | 654 MB | **689 MB** |
| .htaccess compat | 10/10 | 6/10 | **10/10** |

*Tested on Linode 4C/8G, EL9, PHP 8.3, MariaDB 10.11*

## Related Projects

- [LiteHttpd](https://litehttpd.com) — Apache-compatible lightweight web server
- [WebCasa](https://web.casa) — AI-native server control panel

## License

GPLv3 — See [LICENSE](LICENSE) for details.
