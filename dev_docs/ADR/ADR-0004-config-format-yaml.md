# ADR-0004: 安装与站点配置文件选择 YAML

## Status

Accepted

## Context

LLStack 需要配置文件驱动安装，同时兼顾 CLI/TUI 用户可读性和手写体验。

## Decision

首选 YAML 作为用户配置文件格式；内部统一解码到 `InstallProfile` / `SiteSpec` 等 canonical 输入模型。JSON 作为机器输出格式保留，TOML 暂不作为主格式。

## Consequences

优点：

- 手写友好
- 注释支持好
- 与运维生态兼容高

代价：

- 需要严格 schema 校验，避免隐式类型坑

补充：

- Phase 1 起引入 schema validation
- 所有计划和报告仍输出 JSON，方便自动化
