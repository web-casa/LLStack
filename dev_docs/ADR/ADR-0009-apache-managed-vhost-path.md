# ADR-0009: Apache 托管 vhost 默认目录暂定为 /etc/httpd/conf.d/llstack/sites

## Status

Proposed

## Context

LLStack 需要为 Apache backend 提供稳定的托管配置目录，既避免与系统默认 `conf.d` 下其他文件混杂，又保持运维可读性和回滚边界清晰。

## Decision

Phase 2 当前实现采用：

- `/etc/httpd/conf.d/llstack/sites`

作为 LLStack 托管的 Apache vhost 默认目录。

## Consequences

优点：

- 明确区分 LLStack 管理文件与用户其他 Apache 配置
- 便于 future cleanup、备份和回滚
- 目录职责清晰，适合后续加入 `site:disable` / `site:enable`

风险：

- 该路径仍待用户最终确认
- 若后续改为 `conf.d/llstack/*.conf` 或其他布局，需要迁移逻辑
