---
name: Backlog Priorities
description: Current development progress and remaining work items as of 2026-03-26
type: project
---

## Completed in current session

Steps 1-5 of per-site isolation + PHP 7.4-8.5 + Valkey:
- system/user.go (site username ≤12 chars) + system/hardware.go (CPU/RAM detection)
- tuning/tuning.go (hardware-aware parameter calculation)
- site:create/delete integrated with per-site Linux user management
- Apache renderer: per-site FPM pool generation function ready
- OLS compiler: phpIniOverride with open_basedir
- LSWS renderer: IfModule LiteSpeed suEXEC block
- PHP versions expanded to 7.4-8.5 with EOL warnings
- Valkey cache provider added (Redis-compatible fork)

## Remaining steps (in order)

- Step 7: per-site PHP config override CLI (`llstack site:php-config`)
- Step 9: SSL certificate lifecycle (ssl:status/renew/auto-renew + systemd timer)
- Step 10: Cron task management (cron:add/list/remove + wp-cron preset)
- Step 11: Security hardening (fail2ban / IP blacklist / rate limiting / firewalld)
- Step 12: OLS .htaccess compatibility (htaccess-check/compile/watch)
- VPS end-to-end testing (all features together)

## Key design decisions (confirmed by user)

- Per-site group model (方案 B): each site gets own group, web server added per-site
- Username max 12 chars: `{prefix≤7}_{hash4}`
- open_basedir default enabled
- Rate limiting: unified req/s semantics (Apache mod_evasive / OLS perClientConnLimit)
- Valkey/Redis mutually exclusive
- PHP EOL versions: warning only, no block
- SSL auto-renew: systemd timer, 14-day threshold
- Cron: per-site user execution
- Security features: opt-in by default

**Why:** These are foundational architecture decisions for the next major feature set.

**How to apply:** Follow DESIGN_PER_SITE_ISOLATION.md and DESIGN_OLS_HTACCESS_COMPAT.md for implementation details.
