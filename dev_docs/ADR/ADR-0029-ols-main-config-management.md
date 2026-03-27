# ADR-0029: OLS Main Config Management

## Status

Accepted

## Context

Phase 3 建立了 OLS compiler，为每个站点生成 vhconf.conf 和 listener map 文件，但不管理 OLS 主配置 `httpd_config.conf`。运维人员必须手动将 virtualhost block 和 listener map entry 注册到主配置。

这是 Phase 3 以来的核心遗留缺口：site:create 生成配置但 OLS 不知道加载它们。

## Decision

新增 `internal/backend/ols/configmanager.go`，在 site create/delete 时自动管理 `httpd_config.conf`：

1. **RegisterSiteInMainConfig** — 在主配置末尾追加 virtualhost block，在 listener block 中插入 map entry
2. **UnregisterSiteFromMainConfig** — 从主配置移除对应段落

使用注释标记识别 LLStack 管理的段落：
- `# LLSTACK_VHOST_BEGIN <site>` / `# LLSTACK_VHOST_END <site>`
- `# LLSTACK_MAP_BEGIN <site>` / `# LLSTACK_MAP_END <site>`

操作是幂等的——重复注册会先移除旧段落再追加。

## Consequences

优点：

- site:create --backend ols 后 OLS 可立即加载新站点配置
- site:delete 自动清理主配置
- 标记化设计不影响用户手工配置
- 与现有 FileApplier/rollback 机制兼容

限制：

- 直接修改主配置，比 include 机制稍有侵入性
- 依赖主配置中已存在 listener block
- 如果主配置不存在或格式异常，会静默跳过（warn 日志）
