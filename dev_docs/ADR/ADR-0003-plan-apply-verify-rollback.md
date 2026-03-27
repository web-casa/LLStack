# ADR-0003: 所有写操作采用 plan -> apply -> verify -> rollback metadata 流程

## Status

Accepted

## Context

LLStack 需要 dry-run、plan-only、自动备份、断点恢复、doctor / repair / rollback。

## Decision

所有写操作必须：

1. 先生成 operation plan
2. 在 apply 前写入 snapshot / backup metadata
3. apply 完成后执行 verify
4. 将结果写入 history，以支持 rollback

## Consequences

- CLI / TUI / 配置文件驱动共享同一底层逻辑
- dry-run 与 JSON plan 输出天然成立
- 需要单独设计 operation、resource、snapshot 元数据模型
