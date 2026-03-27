# Changelog

All notable changes to LLStack will be documented in this file.

## [0.1.0] - 2026-03-28

### 🎉 Initial Release

The first public release of LLStack — a CLI + TUI web stack installer and site lifecycle manager for EL9/EL10.

### Web Backends
- Apache httpd with automatic vhost management and `IncludeOptional` configuration
- OpenLiteSpeed with config compiler, parity report, main config auto-registration, and `.htaccess` compatibility tools
- LiteSpeed Enterprise with ProcessGroup suEXEC and capability detection

### PHP
- Versions 7.4–8.5 via Remi repository (with EOL warnings for 7.4/8.0/8.1)
- Per-site PHP version switching (`site:php-switch`)
- Per-site PHP config overrides (`site:php-config`)
- PHP-FPM pool tuning (`php:tune`)
- Install and uninstall (`php:install`, `php:remove`)

### Databases
- MariaDB, MySQL, PostgreSQL, Percona Server
- Install, init, create database/user, TLS configuration
- Hardware-aware parameter tuning (`db:tune`)
- Backup with mysqldump/pg_dumpall (`db:backup`)
- Scheduled backups via systemd timer

### Cache
- Redis, Memcached, Valkey (Redis-compatible fork)
- Install, configure, live probes, memory saturation monitoring

### Site Lifecycle
- Create, delete, update, show, list, diff
- Start, stop, restart, reload
- SSL/TLS management with Let's Encrypt integration
- Access logs with follow mode and grep filtering
- Per-site Linux user isolation
- Batch creation from YAML/JSON config
- Access log analytics (`site:stats`)
- Site backup and restore (`site:backup`, `site:restore`)

### Application Deployment
- One-click deploy: WordPress, Drupal, Joomla, Nextcloud, Matomo, MediaWiki, Typecho, Laravel
- Auto-generates database credentials and configures application

### SSL Certificates
- Certificate status monitoring (`ssl:status`)
- Manual and batch renewal (`ssl:renew`)
- Auto-renewal via systemd timer (`ssl:auto-renew`)
- Doctor check for certificate expiry (14-day warning)

### Security
- fail2ban with LLStack default jails (monitor-only mode)
- IP blocking/unblocking via firewalld rich rules (IPv4 + IPv6)
- Rate limiting: Apache mod_evasive, OLS/LSWS perClientConnLimit
- Firewall port management (`firewall:open`, `firewall:close`)

### Cron Management
- Per-site cron task management
- WordPress wp-cron preset (auto-disables WP internal cron)
- Laravel scheduler preset

### SFTP
- Per-site SFTP account creation with chroot isolation
- Password and SSH key authentication

### Diagnostics
- 26 doctor checks covering OS, SELinux, firewalld, PHP-FPM, databases, cache, SSL, OLS compatibility
- Auto-repair for directory permissions, SELinux contexts, firewalld ports, PHP config drift, OLS .htaccess conversion
- Diagnostics bundle export (30+ snapshots)

### TUI Interface
- 12-page interactive terminal interface
- Dashboard with hardware detection and tuning recommendations
- Install wizard with scenario selector (WordPress/Laravel/API/Static/Reverse-Proxy)
- Site management with create/edit wizards
- SSL certificate status, Cron tasks, Security overview

### Hardware Tuning
- Auto-detect CPU cores and RAM
- Calculate optimal parameters for PHP-FPM, Apache, OLS, database, and cache
- `llstack tune` command for recommendations

### Installation
- Interactive installer (`llstack install` with step-by-step wizard)
- One-line install script: `curl -sSL https://get.llstack.com | bash`
- Config-driven install from YAML/JSON profiles
- Post-install welcome page with PHP probe (x-prober) and Adminer

### Release Infrastructure
- Cross-platform build (linux/amd64, linux/arm64)
- SHA256 checksums, SPDX SBOM, provenance manifest with build context
- OpenSSL detached signatures (default-enforced when pubkey provided)
- Provider-neutral publish (GitHub, directory)
- GitHub Actions CI + tag-driven release workflow
- Docker functional smoke matrix: EL9/EL10 × Apache/OLS/LSWS (6 services)
