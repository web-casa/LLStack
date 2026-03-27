# ADR-0002: 以 Apache VirtualHost 语义作为 canonical source

## Status

Accepted

## Context

LLStack 必须同时支持 Apache、OLS、LSWS，并且要求：

- 用户不手工维护 OLS 原生配置
- 站点定义保持 backend-agnostic
- 配置能进行能力映射、降级和 parity report

## Decision

以 Apache VirtualHost 语义作为 single source of truth，内部建立 canonical AST / IR，再由 backend renderer / compiler 生成目标配置。

## Consequences

优点：

- 用户站点模型稳定
- Apache 后端实现最直接
- OLS / LSWS 能共享更高层语义

代价：

- OLS 需要 compiler，而不是简单模板渲染
- 必须维护 capability / parity report

约束：

- backend 不能扩散为新的“用户主配置入口”
- 所有 backend-specific 特性必须通过扩展层注入并带有 capability 标记
