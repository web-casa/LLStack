# ADR-0005: 数据库采用 provider/capability/TLS 三层抽象

## Status

Accepted

## Context

LLStack 必须支持 MariaDB、MySQL、Percona、PostgreSQL，并统一展示 TLS 初始化与状态。

## Decision

数据库子系统采用三层抽象：

1. provider：安装、初始化、用户/数据库操作
2. capability：支持项与限制
3. TLS profile：server-side TLS、客户端连接参数、证书路径、状态报告

## Consequences

- MariaDB / MySQL / Percona 可共享大量 provider 行为
- PostgreSQL 保留独立认证与 TLS 适配
- CLI / TUI 可统一展示 “已安装 / 已初始化 / TLS 状态 / 连接信息”
