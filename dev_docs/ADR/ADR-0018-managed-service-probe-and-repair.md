# ADR-0018: Managed Service Probe And Repair Scope

## Status

Accepted

## Context

Phase 8 需要把 `doctor` 从静态环境检查推进到运行态检查，但 LLStack 当前同时支持 Apache、OLS、LSWS、PHP、DB 和 cache。并不是每一类组件都天然有稳定统一的 service unit 名称。

## Decision

- `doctor` 新增 `managed_services` 检查
- service probe 优先基于以下来源推导 service name：
  - 站点 backend 的 `systemctl` 型 reload/restart 命令
  - 站点 `php-fpm` binding 的 `fpm_service`
  - DB provider manifest 的 `service_name`
  - cache provider manifest 的 `service_name`
- probe 统一使用 `systemctl is-active <service>`
- `repair` 对明确判定为 `inactive` / `failed` 的 service 增加 `service.start` 自动修复规则
- 无法稳定推导 service name 的 backend 进入 `unprobed`，在 `doctor` 中显式 warning，而不是静默跳过

## Consequences

- Apache、php-fpm、DB、cache 现在有真实 active probe
- OLS / LSWS 只有在运行命令显式映射到 `systemctl` service 时才会进入 active probe
- `repair` 仍保持保守，只自动处理“明确 inactive 且可 start”的服务，不做更激进的 restart/reconfigure
