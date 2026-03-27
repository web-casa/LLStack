# ADR-0010: OLS 采用受管配置树与 parity report 并行输出

## Status

Accepted

## Context

OLS 不直接消费 Apache vhost 文件。若只渲染单个文本文件，无法覆盖 listener map、extprocessor、script handler 和编译降级信息，也无法支撑后续 parity 审计。

## Decision

Phase 3 对 OLS backend 采用：

- 受管配置树输出
- 独立 parity report JSON
- manifest 记录所有 managed assets

默认输出布局：

- `/usr/local/lsws/conf/vhosts/<site>/vhconf.conf`
- `/usr/local/lsws/conf/llstack/listeners/<site>.map`
- `/var/lib/llstack/state/parity/<site>.ols.json`

## Consequences

优点：

- 用户不需要手工维护 OLS 原生配置
- delete / rollback 可基于 asset list 工作
- parity report 成为一等输出，而不是临时日志

代价：

- manifest 模型需要从单文件升级为多资产清单
- OLS create/delete 的 apply 逻辑比 Apache 更复杂
