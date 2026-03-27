# ADR-0020: Repair Normalizes Managed Directory Permissions Conservatively

## Status

Accepted

## Context

Phase 8 已经能诊断 LLStack 控制平面目录的 owner/group、mode 和 SELinux context，但如果 `repair` 只能补缺失目录，实际恢复能力仍然不足。与此同时，直接递归修复站点源码树或业务日志目录风险过高。

## Decision

- `repair` 新增对 LLStack 控制平面目录的基础权限修复
- 当前自动修复范围仅限：
  - 已存在受管路径的 `chmod`
  - root 场景下的受管路径 `chown`
- mode 修复策略采用最小增量：
  - 目录至少补齐 owner `rwx`
  - 文件至少补齐 owner `rw`
- apply 顺序调整为：
  - 先修已有路径的 mode/owner
  - 再创建缺失目录
  - 再执行 service/site 级 repair
- 不递归修改站点源码树，不强制修复业务目录权限

## Consequences

- `repair` 现在可以处理更常见的控制平面权限漂移
- 父目录权限异常时，缺失子目录也能恢复成功
- owner/group 自动修复仍依赖 root 权限，非 root 环境只会给出 warning
