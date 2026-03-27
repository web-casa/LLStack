# ADR-0015: Certbot Detection And Scaffold Scope

## Status

Accepted

## Context

Phase 7 需要补齐 Let’s Encrypt 体验，但当前仓库无法自洽完成真实公网 ACME 验证。同时，`wordpress` / `laravel` profile 需要比最初 placeholder 更完整的 starter files，但仍不能把应用下载安装逻辑硬塞进 Phase 7。

## Decision

- Let’s Encrypt 继续使用 `certbot --webroot` 路线，不引入 web server plugin 依赖。
- 在运行 `site:ssl --letsencrypt` 前，先做 `certbot` binary detection。
- detection 优先使用发行版常见路径候选，其次回退 PATH 查找。
- dry-run / plan-only 允许在未检测到 `certbot` 时继续生成 plan，但必须显式 warning。
- apply 时若未检测到 `certbot`，必须失败，不能静默跳过。
- WordPress / Laravel profile 只提供更完整的 starter scaffold，不自动下载 WordPress 核心，也不执行 `composer create-project`。

## Consequences

- Phase 7 可以完成 ACME 命令链路、发行版探测和 Docker/集成测试骨架。
- Phase 7 仍不宣称“真实 ACME 功能测试已完成”。
- profile scaffold 对新手更友好，但不会把应用安装生命周期和站点管理生命周期耦合在一起。
