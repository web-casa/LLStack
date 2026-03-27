# ADR-0016: Doctor Repair And Site Reconcile

## Status

Accepted

## Context

Phase 8 需要把 `doctor / repair / rollback` 从占位命令推进到可执行能力。仅输出静态诊断文案无法支撑实际运维，因此至少需要：

- 可读的机器报告
- 可执行的 repair plan
- 对受管站点缺失 asset 的最小恢复路径

## Decision

- 新增 `internal/doctor` service，统一生成 doctor report 与 repair plan。
- `doctor` 当前覆盖：
  - OS support
  - managed directory layout
  - managed sites / missing assets
  - runtime binary detection
  - rollback history
- `repair` 当前覆盖：
  - 缺失目录创建
  - 受管站点基于 manifest 的 `site.reconcile`
- 新增 `site.Reconcile`，从已存 canonical manifest 重新渲染 backend assets 与 scaffold assets，并复用原有 apply/reload/rollback 链路。

## Consequences

- Phase 8 初版已经不是只读诊断，而是可执行修复。
- repair 仍然是保守实现，不会自动修正 rewrite/header/access rule 级语义问题。
- 后续更深的 doctor / repair 规则继续叠加在同一 service 内，而不是散落到 CLI 命令里。
