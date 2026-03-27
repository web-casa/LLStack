# ADR-0017: Diagnostics Bundle Format

## Status

Accepted

## Context

Phase 8 需要提供可导出的 diagnostics bundle，用于在不暴露整机文件系统的前提下收集 LLStack 当前诊断结果、受管清单和最近历史。

## Decision

- diagnostics bundle 使用 `.tar.gz` 格式
- 入口复用 `llstack doctor --bundle`
- bundle 当前包含：
  - `report.json`
  - `summary.json`
  - `probes/checks.json`
  - `host/os-release`
  - `sites/`
  - `db/providers/`
  - `db/connections/`
  - `cache/providers/`
  - `php/runtimes/`
  - `history/` 最近若干条记录
  - `logs/sites/<site>/{access,error}.log` 的 tail 摘要

## Consequences

- 诊断包和 `doctor` 报告保持同一来源
- 当前 bundle 只采集 LLStack 受管元数据、probe 快照和日志摘要，不打包完整日志和站点代码目录
- 后续可在不破坏格式的前提下继续往 bundle 中增加内容
