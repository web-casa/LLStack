<p align="center">
  <h1 align="center">LLStack</h1>
  <p align="center">
    CLI + TUI Web Stack Manager for EL9 / EL10
    <br />
    <em>Apache · OpenLiteSpeed · LiteSpeed Enterprise</em>
  </p>
  <p align="center">
    <a href="https://github.com/web-casa/llstack/releases"><img src="https://img.shields.io/github/v/release/web-casa/llstack?style=flat-square" alt="Release"></a>
    <a href="https://github.com/web-casa/llstack/actions"><img src="https://img.shields.io/github/actions/workflow/status/web-casa/llstack/ci.yml?style=flat-square&label=CI" alt="CI"></a>
    <a href="https://github.com/web-casa/llstack/blob/main/LICENSE"><img src="https://img.shields.io/github/license/web-casa/llstack?style=flat-square" alt="License"></a>
    <a href="https://github.com/web-casa/llstack"><img src="https://img.shields.io/github/stars/web-casa/llstack?style=flat-square" alt="Stars"></a>
  </p>
</p>

---

## Install

```bash
curl -sSL https://get.llstack.com | bash
```

> Requires Rocky Linux / AlmaLinux / RHEL 9 or 10. Runs an interactive wizard to set up your web stack.

<!-- TODO: Add terminal recording GIF here -->
<!-- ![LLStack Install Demo](docs/assets/install-demo.gif) -->

## Why LLStack?

|  | cPanel | Plesk | CyberPanel | **LLStack** |
|---|--------|-------|------------|-------------|
| **Price** | $15-45/mo | $10-30/mo | Free | **Free & Open Source** |
| **Interface** | Web GUI | Web GUI | Web GUI | **CLI + TUI (no web exposure)** |
| **Attack Surface** | Large (web panel) | Large | Large | **Minimal (no web daemon)** |
| **Automation** | API required | API required | Limited | **Native CLI, scriptable** |
| **OLS/LSWS** | Plugin | Plugin | OLS only | **Native (Apache + OLS + LSWS)** |
| **Target** | Hosting providers | Hosting providers | Personal | **Developers & Sysadmins** |

## Features

**Web Backends** — Apache, OpenLiteSpeed, LiteSpeed Enterprise with per-site user isolation

**PHP 7.4–8.5** — Multi-version via Remi, per-site config overrides, pool tuning, version switching

**Databases** — MariaDB, MySQL, PostgreSQL, Percona — install, TLS, backup, hardware-aware tuning

**Cache** — Redis, Memcached, Valkey — with live probes and saturation monitoring

**Site Lifecycle** — Create, deploy apps (WordPress/Drupal/Nextcloud/Laravel/+5 more), backup/restore, SSL auto-renewal

**Security** — fail2ban, IP blocking, rate limiting, firewall management — all opt-in

**Diagnostics** — 26 doctor checks, auto-repair, diagnostics bundle

**TUI** — 12-page interactive terminal interface

**Hardware Tuning** — Auto-detect CPU/RAM and calculate optimal parameters

## Quick Start

```bash
# Install LLStack
curl -sSL https://get.llstack.com | bash

# The interactive wizard handles everything:
#   ✓ Web backend (Apache/OLS/LSWS)
#   ✓ PHP version
#   ✓ Database (MariaDB/MySQL/PostgreSQL/Percona)
#   ✓ Cache (Redis/Memcached/Valkey)
#   ✓ Security (fail2ban)
#   ✓ Hardware auto-tuning

# After install, add your first site:
llstack site:create example.com --backend apache --profile wordpress

# Deploy WordPress automatically:
llstack app:install wordpress --site example.com

# Or open the TUI:
llstack tui
```

## CLI Commands (76)

```
Setup:            install, tune, version, status, welcome:remove
Site Management:  site:create, site:delete, site:show, site:list, site:diff,
                  site:update, site:start, site:stop, site:reload, site:restart,
                  site:ssl, site:logs, site:stats, site:php-config, site:php-switch,
                  site:backup, site:restore, site:batch-create,
                  site:htaccess-check, site:htaccess-compile
App Deploy:       app:install, app:list
PHP:              php:install, php:remove, php:tune, php:list, php:extensions, php:ini
Database:         db:install, db:init, db:create, db:create-user,
                  db:remove, db:backup, db:tune, db:list
Cache:            cache:install, cache:configure, cache:status
SSL:              ssl:status, ssl:renew, ssl:auto-renew
Cron:             cron:add, cron:list, cron:remove
SFTP:             sftp:create, sftp:list, sftp:remove
Security:         security:fail2ban, security:fail2ban-status,
                  security:block, security:unblock, security:blocklist,
                  security:ratelimit
Firewall:         firewall:status, firewall:open, firewall:close
Diagnostics:      doctor, repair, rollback:list, rollback:show
```

## Supported Applications

| App | Database | Min PHP |
|-----|----------|---------|
| WordPress | MySQL | 7.4 |
| Drupal | Any | 8.1 |
| Joomla | MySQL | 8.1 |
| Nextcloud | Any | 8.0 |
| Matomo | MySQL | 7.4 |
| MediaWiki | Any | 8.1 |
| Typecho | MySQL | 7.4 |
| Laravel | Any | 8.2 |

```bash
llstack app:install wordpress --site wp.example.com
llstack app:install nextcloud --site cloud.example.com
```

## Requirements

- **OS**: Rocky Linux 9/10, AlmaLinux 9/10, or RHEL 9/10
- **Arch**: x86_64 or aarch64
- **RAM**: 1 GB minimum, 2 GB+ recommended
- **Root**: Required for service management

## Development

```bash
# Build
make build

# Test
make test

# Smoke test
make smoke

# Docker functional smoke (6 services: EL9/EL10 × Apache/OLS/LSWS)
make docker-smoke

# Full release pipeline
make release-pipeline VERSION=v0.1.0 MODE=validate
```

## Documentation

- [Architecture](dev_docs/ARCHITECTURE.md)
- [Compatibility Matrix](dev_docs/COMPATIBILITY.md)
- [TUI UX Design](dev_docs/TUI_UX.md)
- [Per-Site Isolation Design](dev_docs/DESIGN_PER_SITE_ISOLATION.md)
- [OLS .htaccess Compatibility](dev_docs/DESIGN_OLS_HTACCESS_COMPAT.md)
- [VPS Test Plan](dev_docs/VPS_TEST_PLAN.md)
- [Release Operations](dev_docs/RELEASE_OPERATIONS.md)
- [Known Limitations](dev_docs/KNOWN_LIMITATIONS.md)
- [Changelog](CHANGELOG.md)

## License

[Apache License 2.0](LICENSE)

## Contributing

Contributions are welcome! Please read the [Architecture](dev_docs/ARCHITECTURE.md) document first to understand the canonical model and non-negotiable constraints.
