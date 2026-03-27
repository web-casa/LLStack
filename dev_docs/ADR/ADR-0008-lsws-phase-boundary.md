# ADR-0008: LSWS 早期阶段以编译与探测优先，授权能力验证后置

## Status

Accepted

## Context

LiteSpeed Enterprise 受授权环境约束，难以在项目早期完全自动化验证所有功能。如果强行要求 Phase 4 同时完成完整真实服务验证，会阻塞整体路线。

## Decision

Phase 4 的 LSWS 交付边界定义为：

- 配置编译正确性
- capability flags
- trial / licensed 模式识别
- 基本启动与探测能力

完整授权功能验证在具备 license 环境时后置补齐。

## Consequences

- 项目可在不牺牲主架构的前提下持续推进
- 必须在文档、测试和 CLI/TUI 中清晰标注当前验证级别
- 不能把 compile-only 阶段描述成“完整生产等价验证”
