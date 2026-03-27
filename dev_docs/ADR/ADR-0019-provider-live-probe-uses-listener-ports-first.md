# ADR-0019: Provider Live Probe Uses Listener Ports First

## Status

Accepted

## Context

Phase 8 需要把数据库和缓存 provider 的健康检查从静态 manifest/TLS 资产存在性推进到更接近运行态的 probe，但当前仓库测试矩阵并不保证每一种 provider 都能在本地或 Docker 中稳定完成真实协议登录。

## Decision

- `doctor` 新增 `managed_provider_ports` 检查
- 对 DB provider：
  - 优先从 managed manifest 的 admin connection 端口读取
  - 否则回退到 provider spec 默认端口
- 对 cache provider：
  - 优先读取 manifest 中的配置端口
  - 否则回退到 provider 默认端口
- probe 使用 `ss -ltn` 对比预期端口与实际监听端口
- 本阶段不把 SQL 登录或 Redis/Memcached 协议握手作为默认 live probe

## Consequences

- 现在可以发现“provider 已受管但端口未监听”的问题
- 该检查对 Docker/单元测试更稳定，也不会把登录凭证管理提前耦合进 doctor
- 更深的协议级 probe 仍可在后续阶段叠加，但不会替代当前 listener probe
