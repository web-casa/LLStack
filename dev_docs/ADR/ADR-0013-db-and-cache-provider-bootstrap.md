# ADR-0013: Database And Cache Provider Bootstrap

## Status

accepted

## Context

Phase 6 需要把数据库与缓存从 install 计划占位推进到可执行子系统，同时保持 LLStack 的统一约束：

- 仍然必须走 plan/apply
- 仍然必须写 manifest
- 必须支持 dry-run / plan-only
- 必须为 TLS、connection info、capability matrix 预留统一接口
- 早期不能把复杂逻辑塞进 bash 脚本

数据库 provider 之间存在明显差异：

- MariaDB / MySQL / Percona 共享较多 MySQL family 语义
- PostgreSQL 的 service、角色、TLS 与客户端调用模型不同
- Memcached / Redis 不应硬套数据库抽象，但应纳入统一 provider lifecycle

## Decision

采用两个并行但风格一致的子系统：

- `internal/db`
- `internal/cache`

共同约束：

- provider spec + capability model
- manager 负责 plan/apply/manifest
- 所有 provider 状态落地到 `/etc/llstack/.../*.json`
- TLS profile 与 connection info 以独立 JSON 管理

数据库子系统采用：

- `ProviderSpec`
- `DatabaseCapability`
- `DatabaseTLSProfile`
- `ProviderManifest`
- `ConnectionInfo`

缓存子系统采用：

- `ProviderSpec`
- `ProviderCapability`
- `ProviderManifest`

Phase 6 的实际 apply 边界：

- 安装：`dnf install` + `systemctl enable --now`
- 初始化：provider-aware SQL / setup command + manifest 更新
- 数据库与用户创建：provider-aware SQL + connection info 落盘
- TLS：先写 provider-native config snippet 和 managed profile，不宣称已做真实服务级验证

## Consequences

优点：

- Phase 6 可以形成真正可执行的 provider 子系统
- CLI 与后续 TUI 可以复用同一 manager
- provider capability / TLS status 已有稳定落点

代价：

- PostgreSQL 的 `pg_hba.conf` 强制 TLS 仍未完整管理
- MySQL / Percona vendor repo 仍要求用户或后续阶段补齐 repo setup
- 真实服务级功能测试要留到 Docker / 裸机验证阶段
