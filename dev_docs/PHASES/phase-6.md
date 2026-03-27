# Phase 6: 数据库与缓存子系统

## 1. Goal

- 建立数据库 provider 初版
- 建立 TLS profile 初版
- 建立 DB init / create database / create user 链路
- 建立 Memcached / Redis cache provider 初版
- 为后续 TUI 向导和 doctor/repair 铺好 provider manifest 基线

## 2. Scope

做了什么：

- 新增 `internal/db` 子系统
- 新增 `internal/cache` 子系统
- 实现 database provider spec / capability / tls profile / manifest
- 实现 `db:list` / `db:install` / `db:init` / `db:create` / `db:user:create`
- 实现 `cache:install` / `cache:status` / `cache:configure`
- 实现 DB connection info JSON 输出
- 让 `install` 总命令接入 DB/cache resolver 细节
- 增加 DB/cache unit/integration tests

没有做什么：

- 未做真实 EL9/EL10 上的数据库安装功能测试
- 未实现数据库卸载
- 未实现 DB backup hooks 的真实逻辑
- 未实现 PostgreSQL `pg_hba.conf` 重写
- 未实现 Redis/Memcached 真服务健康探测

## 3. Decisions / ADR

新增 ADR：

- `ADR-0013-db-and-cache-provider-bootstrap.md`

## 4. Architecture

当前数据库链路：

`CLI -> db.Manager -> ProviderSpec/TLSProfile -> plan/apply -> provider manifest + connection info`

当前缓存链路：

`CLI -> cache.Manager -> ProviderSpec -> plan/apply -> provider manifest + managed config`

Phase 6 新增托管路径：

- DB provider manifest：`/etc/llstack/db/providers/<provider>.json`
- DB connection info：`/etc/llstack/db/connections/<provider>-<name>.json`
- DB cert base dir：`/etc/llstack/db/certs/<provider>/`
- cache provider manifest：`/etc/llstack/cache/providers/<provider>.json`

## 5. Data Models

Phase 6 新增：

- `db.ProviderSpec`
- `db.DatabaseCapability`
- `db.DatabaseTLSProfile`
- `db.ConnectionInfo`
- `db.ProviderManifest`
- `db.DatabaseRecord`
- `db.UserRecord`
- `cache.ProviderSpec`
- `cache.ProviderCapability`
- `cache.ProviderManifest`

## 6. File Tree Changes

新增核心文件：

- `internal/db/types.go`
- `internal/db/providers.go`
- `internal/db/service.go`
- `internal/cache/types.go`
- `internal/cache/providers.go`
- `internal/cache/service.go`
- `internal/cli/db.go`
- `internal/cli/cache.go`
- `tests/unit/db/service_test.go`
- `tests/unit/cache/service_test.go`
- `tests/integration/db/db_test.go`
- `tests/integration/cache/cache_test.go`

更新核心文件：

- `internal/config/config.go`
- `internal/cli/root.go`
- `internal/cli/install.go`

## 7. Commands / UX Flows

当前新增命令：

- `llstack db:list`
- `llstack db:install mariadb --tls enabled`
- `llstack db:init --provider mariadb --admin-user llstack_admin --admin-password secret`
- `llstack db:create appdb --provider mariadb`
- `llstack db:user:create appuser --provider mariadb --password secret --database appdb`
- `llstack cache:install memcached`
- `llstack cache:status`
- `llstack cache:configure memcached --bind 127.0.0.1 --max-memory 256`

行为说明：

- `db:*` 写操作支持 `--dry-run` / `--plan-only` / `--json`
- provider 已明确时，dry-run 不要求 manifest 已存在
- `db:init` 会生成 managed admin connection info
- `db:user:create` 会在指定 database 时生成连接信息 JSON

## 8. Code

当前数据库 provider 覆盖：

- `mariadb`
- `mysql`
- `postgresql`
- `percona`

当前缓存 provider 覆盖：

- `memcached`
- `redis`

当前实现边界：

- 安装：`dnf install` + `systemctl enable --now`
- 初始化：provider-aware SQL / `postgresql-<ver>-setup initdb`
- TLS：provider-native snippet 渲染 + managed TLS profile
- 缓存：安装 + 配置文件写入 + service restart

## 9. Tests

新增测试：

- DB provider resolver / TLS profile unit test
- DB install unit test
- DB lifecycle integration test（install/init/create db/create user）
- CLI `db:install --dry-run --json` test
- cache install/configure unit test
- cache lifecycle integration test
- CLI `cache:install --dry-run --json` test

已验证：

- `go test ./...`
- `go build ./...`
- CLI dry-run 冒烟：`db:install mariadb --tls enabled --dry-run`
- CLI dry-run 冒烟：`cache:install memcached --dry-run`
- CLI dry-run 冒烟：`db:user:create appuser --provider mariadb --password secret --database appdb --dry-run --json`

## 10. Acceptance Criteria

本阶段验收结论：已满足 Phase 6 的最小目标。

- DB provider model 已落地
- TLS profile 初版已落地
- `db:init` / `db:create` / `db:user:create` 已落地
- Memcached / Redis provider 初版已落地
- CLI 子系统已落地
- tests 已覆盖关键 plan/apply/manifest 路径

## 11. Risks / Tradeoffs

- MySQL / Percona / PostgreSQL 当前默认假定 vendor repo 已由用户或后续阶段配置
- PostgreSQL `required` TLS 仍依赖 `pg_hba.conf` 的 `hostssl` 规则
- 当前 DB SQL 执行路径未处理复杂认证或 socket auth 差异
- cache 配置路径先采用产品默认值，尚未做发行版级路径探测

## 12. Next Phase

Phase 7 将实现：

- install wizard 完整化
- site:create wizard 完整化
- database setup wizard 完整化
- TLS（Let's Encrypt）接入
- WordPress / Laravel / Static / Reverse Proxy 模板
- 一键日志查看 / 配置测试 / reload / restart
