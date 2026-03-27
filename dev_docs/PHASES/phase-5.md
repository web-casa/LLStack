# Phase 5: PHP 子系统（Remi php-litespeed）

## 1. Goal

- 集成 Remi 仓库 PHP runtime 模型
- 支持多版本 PHP 安装与并存
- 支持 per-site PHP 版本切换
- 支持扩展安装
- 支持 php.ini profile 模板
- 为 Apache / OLS / LSWS 提供统一 PHP adapter

## 2. Scope

做了什么：

- 新增 `internal/php` 子系统
- 实现 Remi package resolver
- 实现 PHP runtime manager
- 实现 php.ini profile 模板
- 实现 Apache / OLS / LSWS 统一 PHP adapter
- 实现 `php:list` / `php:install` / `php:extensions` / `php:ini`
- 实现 `site:php`
- 将 install plan 接到 PHP package resolver
- 增加 PHP unit/integration tests

没有做什么：

- 未做真实 EL9/EL10 上的 dnf 安装验证
- 未实现 PHP 卸载
- 未实现 PHP-FPM pool 粒度调优
- 未实现 OLS / LSWS 的运行时热切换验证

## 3. Decisions / ADR

新增 ADR：

- `ADR-0012-remi-php-runtime-and-backend-adapter.md`

## 4. Architecture

当前 PHP 链路：

`CLI -> php.Manager -> Remi Resolver -> plan/apply -> runtime manifest + managed profile snippet`

site PHP 切换链路：

`site:php -> site.Manager.UpdatePHPVersion -> php.BindSiteRuntime -> backend renderer -> manifest rewrite`

当前管理对象：

- runtime manifest: `/etc/llstack/php/runtimes/<version>.json`
- managed profile snippet: `/etc/opt/remi/phpXX/php.d/90-llstack-profile.ini`

## 5. Data Models

Phase 5 新增/扩展：

- `php.RuntimeManifest`
- `php.InstallOptions`
- `php.ExtensionsOptions`
- `php.ProfileOptions`
- `php.ProfileName`
- `model.PHPRuntimeBinding.Command`

## 6. File Tree Changes

新增核心文件：

- `internal/php/types.go`
- `internal/php/profiles.go`
- `internal/php/remi.go`
- `internal/php/adapter.go`
- `internal/php/service.go`
- `internal/cli/php.go`
- `tests/unit/php/service_test.go`
- `tests/integration/php/php_test.go`

更新核心文件：

- `internal/site/service.go`
- `internal/config/config.go`
- `internal/core/model/site.go`
- `internal/core/validate/site.go`
- `internal/backend/ols/compiler.go`
- `internal/backend/lsws/renderer.go`
- `internal/cli/root.go`
- `internal/cli/site.go`
- `internal/cli/install.go`

## 7. Commands / UX Flows

当前新增命令：

- `llstack php:list`
- `llstack php:install 8.4 --profile wp --extensions gd,intl`
- `llstack php:extensions 8.4 --install redis`
- `llstack php:ini 8.4 --profile=laravel`
- `llstack site:php example.com --version 8.4`

行为说明：

- `php:install` 默认同时规划 `php-fpm` 与 `php-litespeed`
- Apache site 绑定到 `php-fpm`
- OLS / LSWS site 绑定到 Remi `lsphp` 命令
- `site:php` 会重渲染目标 backend 配置并更新 manifest

## 8. Code

当前 PHP resolver 输出：

- 包前缀：`php83-php-*` / `php84-php-*`
- Apache adapter：`php-fpm`
- OLS / LSWS adapter：`lsphp`
- FPM socket：`/var/opt/remi/phpXX/run/php-fpm/www.sock`
- lsphp command：`/opt/remi/phpXX/root/usr/bin/lsphp`

当前 profile：

- `generic`
- `wp`
- `laravel`
- `api`
- `custom`

## 9. Tests

新增测试：

- Remi resolver unit test
- PHP adapter unit test
- PHP runtime install integration test
- `site:php` integration test
- CLI `php:install` JSON test

已验证：

- `go test ./...`
- `go build ./...`
- CLI dry-run 冒烟：`php:install 8.4 --profile wp --extensions gd,intl --dry-run`

## 10. Acceptance Criteria

本阶段验收结论：已满足 Phase 5 的最小目标。

- Remi PHP runtime model 已落地
- 多版本并存模型已落地
- per-site PHP 切换已落地
- 扩展安装计划已落地
- php.ini profile 模型已落地
- Apache / OLS / LSWS 统一 adapter 已落地

## 11. Risks / Tradeoffs

- `php:install` 当前依赖 `dnf install`，但未在真实 EL9/EL10 做功能验证
- profile 仅覆盖通用场景，不包含 pool 级别调优
- PHP runtime manifest 当前按版本粒度管理，不含 service health probe
- install 总命令已接入 resolver，但主 apply 仍未统一调用 PHP manager

## 12. Next Phase

Phase 6 将实现：

- 数据库 provider 初版
- TLS profile 初版
- MariaDB / MySQL / PostgreSQL / Percona / Memcached / Redis 抽象
- DB init / secure / create user / create database
