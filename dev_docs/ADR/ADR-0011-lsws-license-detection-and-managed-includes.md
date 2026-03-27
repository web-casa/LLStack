# ADR-0011: LSWS 采用 Apache 风格 managed include，并在渲染阶段做 license detection

## Status

Accepted

## Context

LSWS 与 OLS 不同，它可以沿 Apache 风格配置主导路线前进，但项目仍然需要：

- 显式识别 trial / licensed / unknown
- 输出 capability flags
- 支持 LiteSpeed 专用指令注入

## Decision

Phase 4 对 LSWS backend 采用：

- Apache 风格 managed include：`/usr/local/lsws/conf/llstack/includes/<site>.conf`
- 独立 parity report：`/var/lib/llstack/state/parity/<site>.lsws.json`
- 渲染阶段先做 license detection，再输出 capability snapshot

## Consequences

优点：

- 保持“Apache 配置主导”方向
- CLI dry-run 就能显示 license/capability 语义
- 后续 PHP adapter 和 enterprise feature flags 有稳定挂点

代价：

- 若 license 为 unknown，只能保守判定能力
- 真正的 LSWS 运行时行为仍需后续真实环境验证
