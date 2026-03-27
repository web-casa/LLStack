# ADR-0007: Apache backend 使用 php-fpm 而不是 php-litespeed

## Status

Accepted

## Context

项目要求基于 Remi `php-litespeed` 生态支持 OLS / LSWS 相关 PHP runtime，但 Apache backend 不需要为这一点引入不必要的复杂性。Apache 的稳定主路径应优先选择生态成熟、运维预期明确的方案。

## Decision

- Apache backend 的 PHP 运行时统一采用 `php-fpm`
- OLS / LSWS 的 PHP adapter 继续围绕 Remi `php-litespeed` 设计
- 不在 Apache 模式下引入 LiteSpeed 官方 `lsphp` 安装链路

## Consequences

优点：

- Apache Phase 2/5 实现边界更清晰
- 运维行为与 EL 生态习惯一致
- 减少 backend-specific 运行时耦合

代价：

- 多 backend 的 PHP adapter 实现路径会分化
- 文档中需要明确 Apache 与 OLS / LSWS 的 runtime 差异
