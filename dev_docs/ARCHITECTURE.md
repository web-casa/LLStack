# LLStack Architecture

## 1. Product Boundary

LLStack 负责：

- EL9 / EL10 Web 环境初始化
- Apache / OpenLiteSpeed / LiteSpeed Enterprise 三后端管理
- 站点定义、渲染、应用、回滚
- PHP 多版本管理与 per-site 绑定
- 数据库 / 缓存 provider 生命周期管理
- TLS、诊断、修复、状态与日志查询
- CLI、TUI、配置文件驱动的统一 plan/apply 流程

LLStack 当前不负责：

- 多节点集群编排
- Web GUI 面板
- 应用代码部署流水线
- WAF / CDN / DNS 托管
- 全量备份系统实现（先保留 hooks）

## 2. Architecture Overview

文字版架构图：

```text
CLI args / TUI forms / YAML config
            |
            v
      input normalizer
            |
            v
   canonical install/site model
            |
            +----------------------+
            | validator            |
            | capability matcher   |
            | default resolver     |
            +----------------------+
            |
            v
           planner
            |
            v
   operation plan (JSON-able)
            |
      +-----+------+------------------------+
      |            |                        |
      v            v                        v
 renderer      package/service         snapshot/backup
 compiler      orchestration           rollback metadata
      |            |                        |
      +------------+-----------+------------+
                               |
                               v
                            applier
                               |
                               v
                            verifier
                               |
                               v
                     status/report/history store
```

## 3. Core Principles

1. Apache VirtualHost 语义是 single source of truth。
2. Canonical model 必须 backend-agnostic。
3. 所有写操作必须先生成 plan。
4. 所有配置写入前必须自动备份并可回滚。
5. backend 不允许静默降级；必须输出 warning / parity report。
6. 系统命令统一走 executor，禁止分散 shell 调用。

## 4. Module Boundaries

- `internal/app`: 应用组装、依赖注入、运行入口。
- `internal/cli`: Cobra 命令树、参数解析、JSON/表格输出。
- `internal/tui`: Bubble Tea 程序、页面状态、动作派发。
- `internal/core/model`: canonical data model / AST / IR。
- `internal/core/validate`: 语义校验、默认值补全前置检查。
- `internal/core/capability`: backend / provider capability 检测与匹配。
- `internal/core/plan`: plan builder、operation graph、dry-run 输出。
- `internal/core/render`: 抽象 renderer/compiler 接口与共享 helper。
- `internal/core/apply`: applier、事务边界、文件写入、服务动作。
- `internal/core/verify`: 配置测试、服务状态验证、post-check。
- `internal/backend/apache`: Apache renderer + verifier。
- `internal/backend/ols`: OLS compiler + parity report。
- `internal/backend/lsws`: LSWS renderer + enterprise flags。
- `internal/php`: PHP runtime、version、extension、ini profile 管理与 backend adapter。
- `internal/db`: 数据库 provider 抽象与初始化逻辑。
- `internal/cache`: Memcached / Redis provider。
- `internal/install`: 安装编排，复用已有 subsystem manager。
- `internal/ssl`: 证书、TLS profile、Let's Encrypt 集成。
- `internal/doctor`: 诊断、修复建议、探测规则。
- `internal/system`: executor、fs、systemd、package manager、os facts。
- `internal/rollback`: snapshot metadata、rollback plan builder。
- `internal/logging`: 结构化日志、operation history。
- `internal/config`: 配置文件加载、合并、默认值。
- `internal/site`: site lifecycle orchestration 与 manifest store。
- `pkg/...`: 稳定且通用的可复用组件。

## 5. Canonical Flow

### Install Flow

`CLI/TUI/config -> InstallProfile -> Validate -> Resolve defaults -> Plan -> Preview -> Apply -> Verify -> Report`

### Site Flow

`CLI/TUI/config -> Site model -> Validate -> Capability check -> Render/Compile -> File ops plan -> Apply -> Reload -> Verify -> Snapshot`

### Rollback Flow

`operation history -> select target -> build reverse plan -> preview -> apply rollback -> verify`

### Doctor / Repair Flow

`doctor.Service.Run -> checks/report`

`doctor.Service.Repair -> repair plan -> mkdir/site.reconcile -> optional reload`

`doctor.Service.Bundle -> report + managed metadata snapshot -> tar.gz archive`

## 6. OLS Compiler Strategy

OLS 不直接消费 Apache vhost 文件，因此需要 compiler：

`canonical Site -> OLS IR -> OLS file tree renderer -> listener/vhost mapping -> extprocessor/script handler wiring -> parity report`

输出包括：

- OLS 原生 vhost 配置
- listener 绑定
- rewrite 配置块
- PHP extprocessor / script handler 配置
- TLS 配置块
- parity report（mapped / degraded / unsupported）

## 7. Persistence & Runtime State

Phase 1 先采用本地文件状态目录，后续可替换内部 store 实现。

建议目录：

- `/var/lib/llstack/state/`
- `/var/lib/llstack/history/`
- `/var/lib/llstack/backups/`
- `/var/log/llstack/`
- `/etc/llstack/`

站点目录约定：

- 默认站点根目录：`/data/www/<site>`
- 该约定同时影响 site scaffolding、日志路径默认值、未来模板与 Docker fixtures

Apache PHP 运行时约定：

- Apache backend 早期不使用 `php-litespeed`
- Apache 模式下统一采用 `php-fpm`
- `php-litespeed` 生态仅服务于 OLS / LSWS 相关 PHP adapter 设计

LSWS Phase 4 交付边界：

- 早期接受“配置编译 + capability detection + 基本探测”优先
- 真实授权能力验证后置到具备 license 环境的阶段执行

Phase 2 Apache 托管路径：

- canonical site manifest：`/etc/llstack/sites/<site>.json`
- Apache managed vhost：`/etc/httpd/conf.d/llstack/sites/<site>.conf`
- rollback history：`/var/lib/llstack/history/*.json`
- backups：`/var/lib/llstack/backups/`

Phase 3 OLS 托管路径：

- OLS vhost config：`/usr/local/lsws/conf/vhosts/<site>/vhconf.conf`
- OLS listener map：`/usr/local/lsws/conf/llstack/listeners/<site>.map`
- OLS parity report：`/var/lib/llstack/state/parity/<site>.ols.json`
- manifest 记录 managed asset 清单，以支持 delete / rollback

Phase 4 LSWS 托管路径：

- LSWS include config：`/usr/local/lsws/conf/llstack/includes/<site>.conf`
- LSWS parity report：`/var/lib/llstack/state/parity/<site>.lsws.json`
- LSWS license serial 默认路径：`/usr/local/lsws/conf/serial.no`
- manifest 记录 capability snapshot，以支持 `site:list` 与后续 TUI 展示

Phase 5 PHP 托管路径：

- runtime manifest：`/etc/llstack/php/runtimes/<version>.json`
- profile snippet：`/etc/opt/remi/phpXX/php.d/90-llstack-profile.ini`
- Apache adapter：`php-fpm`
- OLS / LSWS adapter：Remi `lsphp`

Phase 6 DB / cache 托管路径：

- DB provider manifest：`/etc/llstack/db/providers/<provider>.json`
- DB connection info：`/etc/llstack/db/connections/<provider>-<name>.json`
- DB credential file：`/etc/llstack/db/credentials/<provider>-<name>.secret`
- DB TLS cert base dir：`/etc/llstack/db/certs/<provider>/`
- MariaDB TLS config：`/etc/my.cnf.d/llstack-mariadb-tls.cnf`
- MySQL TLS config：`/etc/my.cnf.d/llstack-mysql-tls.cnf`
- Percona TLS config：`/etc/my.cnf.d/llstack-percona-tls.cnf`
- PostgreSQL TLS config：`/var/lib/pgsql/16/data/conf.d/llstack-tls.conf`
- cache provider manifest：`/etc/llstack/cache/providers/<provider>.json`
- Memcached config path：`/etc/sysconfig/memcached`
- Redis config path：`/etc/redis.conf`

Phase 7 lifecycle / install 约定：

- install 通过 `internal/install.Service` 顺序编排 PHP / DB / cache / site manager
- deploy profile 只写 canonical defaults，不直接产出 backend-specific config
- Let's Encrypt 默认证书路径：`/etc/letsencrypt/live/<domain>/`

Phase 8 doctor / repair 约定：

- `internal/doctor.Service` 统一持有 doctor report 与 repair 行为
- diagnostics bundle 由 `doctor.Service.Bundle` 统一导出
- repair 当前通过 `site.reconcile` 恢复受管站点缺失 asset
- repair 当前也会通过 `db.reconcile` 恢复 recoverable DB provider metadata/TLS config 缺口
- repair 也会对明确 inactive/failed 的 managed service 下发 `systemctl start`
- repair 在 root 场景下会对 suspicious managed SELinux labels 下发 `restorecon -Rv`
- repair 在 firewalld 运行且已明确算出缺失端口时，会下发 `firewall-cmd --permanent --add-port=...` 并 `--reload`
- `site.reconcile` 复用 canonical manifest、renderer、applier、verifier 和 rollback history
- `db.reconcile` 复用 provider manifest、TLS renderer 和 connection metadata，不负责重建丢失 secret 内容
- managed service probe 优先基于 backend systemctl 命令、php-fpm binding、DB manifest、cache manifest 推导 service name
- managed provider live probe 当前优先使用 listener 端口对比，而不是协议登录
- managed DB live probe 当前使用无需密码的 reachability 命令：
  - MySQL family: `mysqladmin ping`
  - PostgreSQL: `pg_isready`
- managed DB auth probe 当前基于受管 admin connection + credential file：
  - MySQL family: `mysql ... -e 'SELECT 1'`
  - PostgreSQL: `psql -tAc 'SELECT 1'` via `PGPASSWORD=...`
- managed ownership / SELinux context 目前只采样 LLStack 控制平面的目录，不对站点业务目录做强制判断
- repair 目前也只自动归一化 LLStack 控制平面的目录权限，不递归修改站点业务目录
- diagnostics bundle 当前会额外导出：
  - host runtime 快照
  - managed status 快照
  - managed service 快照
  - 原始 command snapshot（SELinux / firewalld / listening ports / failed systemd units）
  - 扩展 host probe snapshot（`uname -a` / `hostnamectl` / `df -h` / `free -m`）
  - probe 快照
  - 受管站点 access/error log 的 tail 摘要
- repair plan 会把 `doctor` 中可修复项聚合成 warning，并把 operation details 传到 CLI/TUI plan preview
- TUI plan preview 对 `sql` / `password` / `secret` 类 detail 做默认脱敏
- TUI Doctor 页直接复用 `doctor.Service.Run` / `Repair`，不维护平行诊断逻辑
- TUI History 页直接复用 `rollback.List` / `site.Manager.RollbackLast`，当前只暴露 latest pending rollback 执行能力
- TUI History 页当前支持 `all / pending / rolled-back` 过滤
- CLI `rollback:list` / `rollback:show` 直接复用 `rollback.List` / `rollback.Get`，不维护单独 history store
- `site.create` 的 rollback history 只记录站点自身变更；共享控制平面目录只会 ensure，不会作为站点级 rollback target 记录
- preflight checks 当前覆盖 OS、managed dirs/sites、runtime binaries、SELinux、firewalld、listening ports、php-fpm sockets、managed path ownership、managed path permissions、managed SELinux contexts、DB TLS state、DB managed artifacts、managed services、managed provider ports、managed DB live probe、managed DB auth probe、rollback history

Phase 9 release / packaging 约定：

- `internal/buildinfo.Info` 作为二进制构建元数据载体
- `scripts/release/build.sh` 负责跨平台 `go build` 与 `ldflags` 注入
- `scripts/release/package.sh` 负责生成平台级 tarball
- `scripts/install.sh` / `scripts/upgrade.sh` 负责本地安装与升级
- `tests/e2e/smoke.sh` 与 `docker/fixtures/smoke.sh` 作为 host/container smoke 入口
- 当前仍是“仓库内脚本驱动发布”，不是完整 CI/CD pipeline

## 8. Phase 1 Constraints

- 不实现真实安装逻辑
- 先把 app / cli / tui / plan / logging / status 抽象立住
- 为 Phase 2 的 Apache 后端预留 renderer 接口
