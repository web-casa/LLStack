# LLStack Compatibility Matrix

## Backend Scope

| Capability | Apache | OpenLiteSpeed | LiteSpeed Enterprise |
| --- | --- | --- | --- |
| ServerName / ServerAlias | implemented | compiled | apache-style + extension |
| docroot | implemented | compiled | native-like |
| index | implemented | compiled | native-like |
| access/error log | implemented | compiled | native-like |
| TLS basic | implemented | compiled | native-like |
| rewrite basic | implemented | compiled with parity report | native-like |
| reverse proxy basic | implemented | compiled basic context mapping | feature-gated |
| per-site PHP binding | implemented via `php-fpm` | via extprocessor | via lsphp-compatible adapter |
| Apache direct config consumption | yes | no | partial / extended |
| parity report required | low | mandatory | mandatory |

OpenLiteSpeed 当前降级项：

- `HeaderRule`: degraded
- `AccessControlRule`: degraded
- parity report 已输出 JSON，但粒度仍是 feature 级，不是逐指令 diff

LiteSpeed Enterprise 当前能力：

- trial / licensed / unknown 检测：implemented
- directive injection：implemented
- enterprise feature flags：implemented
- 真实服务级验证：pending

## PHP Scope

目标版本：

- 8.2
- 8.3
- 8.4

保留兼容考虑：

- 8.1
- 未来 8.5

约束：

- 使用 Remi `php-litespeed`
- 不使用 LiteSpeed 官方 `lsphp` 安装链路
- Apache backend 使用 `php-fpm`
- OLS / LSWS 的 PHP adapter 以 Remi `php-litespeed` 为主

当前状态：

- 多版本 runtime model：implemented
- per-site PHP switch：implemented
- extension install plan：implemented
- php.ini profiles：implemented

## Site Lifecycle Scope

| Capability | Status | Notes |
| --- | --- | --- |
| deploy profiles | implemented | `generic` / `wordpress` / `laravel` / `static` / `reverse-proxy` |
| scaffold starter files | implemented | profile-specific docroot starter assets |
| drift preview | implemented | `site:diff` compares managed assets with canonical render |
| site start/stop | implemented | toggles managed backend asset paths between active and `.disabled` targets |
| site reload | implemented | backend configtest + reload |
| site logs | implemented | access/error tail read |
| custom TLS update | implemented | rewrites site config and manifest |
| Let's Encrypt flow | partial | `certbot --webroot` orchestration + binary detection + test skeleton exist, real ACME validation pending |

## Database Scope

| Provider | Install | Init | TLS State | Create DB/User | Notes |
| --- | --- | --- | --- | --- | --- |
| MariaDB | implemented | implemented | implemented | implemented | 与 MySQL/Percona 共享较多抽象 |
| MySQL | implemented | implemented | implemented | implemented | 默认假定 vendor repo 已配置 |
| Percona Server | implemented | implemented | implemented | implemented | 默认假定 vendor repo 已配置 |
| PostgreSQL | implemented | implemented | implemented | implemented | `required` TLS 仍需 `pg_hba.conf` 配合 |

## Cache Scope

| Provider | Install | Health Check | Notes |
| --- | --- | --- | --- |
| Memcached | implemented | pending | 已纳入 service component model |
| Redis | implemented | pending | Phase 6 初版已纳入统一 provider model |

## Degradation Rules

- OLS / LSWS 不支持的能力必须写入 parity report
- CLI / TUI 必须提示 degraded / unsupported
- 不允许用户以为“完全兼容 Apache”而实际静默丢失能力
