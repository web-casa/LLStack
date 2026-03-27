# ADR-0001: 选择 Go 作为主实现语言

## Status

Accepted

## Context

LLStack 需要：

- 单二进制分发
- 强 CLI/TUI 体验
- 易于与 systemd、dnf、文件系统、服务探测集成
- 适合 planner / renderer / snapshot testing

## Decision

选择 Go 作为主实现语言，CLI 使用 Cobra，TUI 使用 Bubble Tea + Bubbles + Lip Gloss。

## Consequences

优点：

- 单二进制部署友好
- 性能稳定，适合长期运行的 TUI
- 类型系统有利于 canonical model、plan、capability 建模
- 测试、golden snapshot、交叉编译都更直接

代价：

- Python/Textual 在快速原型上更快
- 某些系统脚本编排写起来没有 Python 灵活

结论：

考虑产品长期维护、分发、测试和架构边界，Go 更适合 LLStack。
