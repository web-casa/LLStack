# ADR-0012: PHP 子系统采用 Remi SCL 风格 runtime model，并通过 backend adapter 绑定到站点

## Status

Accepted

## Context

LLStack 需要：

- 多 PHP 版本并存
- Apache 使用 `php-fpm`
- OLS / LSWS 使用 Remi `php-litespeed`
- per-site PHP 切换

## Decision

Phase 5 采用：

- Remi SCL 风格包命名：`php83-php-*`
- runtime manifest：`/etc/llstack/php/runtimes/<version>.json`
- Apache adapter -> `php-fpm`
- OLS / LSWS adapter -> `lsphp`

## Consequences

优点：

- 并行版本安装模型清晰
- backend 差异集中在 adapter，不污染 site manager 和 renderer
- 后续 PHP extension / ini profile / health probe 有稳定挂点

代价：

- 当前仍需后续真实 EL9/EL10 功能验证
- install 总流程与 php.Manager 的 apply 还未完全统一
