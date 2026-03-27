# ADR-0006: 默认站点根目录采用 /data/www/<site>

## Status

Accepted

## Context

LLStack 需要一个稳定、简单、面向 VPS/轻量运维友好的默认站点路径约定，同时避免与系统默认目录混杂，便于多 backend、回滚、备份和模板生成。

## Decision

默认站点根目录采用：

- `/data/www/<site>`

相关派生路径后续默认放置在同一命名体系下，例如日志、runtime、site metadata。

## Consequences

优点：

- 与常见系统包默认目录分离
- 对多站点、迁移、备份更清晰
- 与参考项目中面向 VPS 的统一路径思路兼容

代价：

- 某些用户预期可能更接近 `/var/www`
- 需要在安装与诊断中检查 `/data` 所在文件系统权限与容量
