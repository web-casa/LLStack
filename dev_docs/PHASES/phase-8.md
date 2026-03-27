# Phase 8: Doctor / Repair / Rollback

## 1. Goal

- 把 `doctor` 从占位输出升级为真实检查
- 提供最小可执行的 `repair` 闭环
- 在现有 rollback 之上补齐 preflight / diagnostics 基线

## 2. Scope

本轮完成：

- 新增 `internal/doctor` service
- `llstack doctor` 输出真实 report
- `llstack doctor --bundle` 生成 diagnostics archive
- `llstack repair` 命令接入
- `status` 不再只是 Phase 1 占位，开始读取真实 managed state
- 新增 `site.reconcile`，可从 manifest 恢复缺失 managed assets
- doctor 初版覆盖：
  - `os_support`
  - `managed_directories`
  - `managed_sites`
  - `runtime_binaries`
  - `selinux_state`
  - `firewalld_state`
  - `listening_ports`
  - `php_fpm_sockets`
  - `managed_path_ownership`
  - `managed_path_permissions`
  - `managed_selinux_contexts`
  - `db_tls_state`
  - `managed_services`
  - `managed_provider_ports`
  - `managed_db_live_probe`
  - `rollback_history`
- repair 初版覆盖：
  - 缺失目录创建
  - 受管目录基础 mode 修复
  - root 场景下的受管目录 owner/group 修复
  - 缺失受管站点 asset 的 reconcile
  - 明确 inactive/failed service 的 `service.start`

本轮不做：

- firewalld rich rules / zone / service 级诊断
- unix socket / php-fpm socket 级诊断
- 更深的 DB/service sanity checks
- 完整日志目录与站点代码目录打包

## 3. Decisions / ADR

- `ADR-0016-doctor-repair-and-site-reconcile.md`
- `ADR-0017-diagnostics-bundle-format.md`
- `ADR-0018-managed-service-probe-and-repair.md`
- `ADR-0019-provider-live-probe-uses-listener-ports-first.md`
- `ADR-0020-repair-normalizes-managed-dir-permissions.md`

## 4. Architecture

新增链路：

`doctor CLI -> doctor.Service.Run -> report`

`doctor CLI --bundle -> doctor.Service.Bundle -> .tar.gz diagnostics archive`

`repair CLI -> doctor.Service.Repair -> chmod/chown/mkdir/service.start/site.reconcile plan -> apply`

`site.reconcile -> manifest -> canonical render + scaffold -> apply -> optional reload`

## 5. Data Models

新增：

- `doctor.Check`
- `doctor.Report`
- `doctor.RepairOptions`
- `doctor.BundleResult`
- `site.ReconcileOptions`

## 6. File Tree Changes

新增核心文件：

- `internal/doctor/service.go`
- `internal/doctor/bundle.go`
- `internal/cli/repair.go`
- `tests/unit/doctor/service_test.go`
- `tests/integration/doctor/doctor_test.go`

更新核心文件：

- `internal/cli/doctor.go`
- `internal/cli/status.go`
- `internal/cli/root.go`
- `internal/site/service.go`
- `tests/unit/cli/root_test.go`

## 7. Commands / UX Flows

新增或增强命令：

- `llstack doctor`
- `llstack doctor --json`
- `llstack doctor --bundle`
- `llstack doctor --bundle --bundle-path /tmp/llstack-diagnostics.tar.gz`
- `llstack repair --dry-run`
- `llstack repair --plan-only`
- `llstack repair --skip-reload`
- `llstack status`

## 8. Code

当前已落地：

- doctor report JSON/text output
- runtime binary detection including certbot
- SELinux mode detection
- firewalld state + required web port detection
- listening TCP port checks for managed web entrypoints
- php-fpm socket presence checks for managed sites
- managed path owner/group sampling
- managed path permission sampling
- managed SELinux context sampling for LLStack-controlled paths
- database TLS asset presence checks
- managed service active probe via `systemctl is-active`
- managed DB/cache provider listener probe via `ss -ltn`
- managed DB live probe via `mysqladmin ping` / `pg_isready`
- managed sites health summary
- rollback history summary
- diagnostics bundle export
- diagnostics bundle now includes managed cache provider metadata
- diagnostics bundle now includes probe snapshots and managed log tails
- diagnostics bundle now includes host runtime snapshot, managed status snapshot, and managed service snapshot
- diagnostics bundle now includes raw command snapshots for SELinux, firewalld, listening ports, and failed systemd units
- diagnostics bundle now includes additional host probe snapshots for `uname -a`, `hostnamectl`, `df -h`, and `free -m`
- diagnostics bundle now includes per-service `journalctl` summaries for managed services
- diagnostics bundle now includes provider-specific config snapshots for DB TLS snippets and cache config files when present
- doctor now includes `db_managed_artifacts` and `managed_db_auth_probe`
- DB init/user creation now persists managed credential files for later diagnostics
- repair now includes `db.reconcile` for recoverable provider metadata/TLS config gaps
- repair can now run `restorecon -Rv` on suspicious managed SELinux paths when root
- repair can now add missing required web ports to firewalld and reload rules when firewalld is running
- TUI History page now supports filter cycling for `all / pending / rolled-back`
- TUI Doctor page now supports filter cycling for `all / warn / pass`
- repair plan now summarizes actionable doctor findings in warnings
- CLI plan text now prints operation details instead of dropping them
- TUI plan preview now prints operation details and redacts secret-bearing fields
- TUI now has a dedicated Doctor page with report/detail/repair preview/apply
- TUI Doctor page now shows repair coverage grouping for `auto-repair` vs `manual-only` findings
- TUI now has a dedicated History page with rollback history list, selected-entry detail, rollback preview and apply confirmation
- CLI now exposes `rollback:list` and `rollback:show` for rollback history inspection
- site rollback no longer records shared LLStack control-plane directories as site-scoped rollback changes
- repair plan generation
- managed directory chmod/chown repair apply
- directory repair apply
- service start repair apply
- site reconcile apply
- status command managed state summary

## 9. Tests

新增测试：

- doctor service unit test
- repair dry-run unit test
- repair CLI dry-run JSON test
- repair integration test for missing managed asset recovery
- managed service inactive warning unit test
- managed service repair integration test
- managed provider port warning unit test
- managed DB live probe unit test
- managed SELinux context warning unit test
- repair permission normalization unit/integration tests
- repair CLI text output warning/detail test
- TUI plan preview detail/redaction test
- TUI doctor page preview/apply tests
- TUI history page preview/apply tests
- CLI rollback:list/show tests
- diagnostics bundle unit test
- diagnostics bundle probe/log tail coverage
- diagnostics bundle host/status/service snapshot coverage
- diagnostics bundle raw command snapshot coverage
- diagnostics bundle extended host probe snapshot coverage
- diagnostics bundle managed service journal snapshot coverage
- diagnostics bundle provider config snapshot coverage
- DB credential file lifecycle coverage
- doctor DB auth probe coverage
- doctor DB reconcile plan coverage
- TUI history filter coverage
- TUI doctor filter coverage
- firewalld repair plan/apply coverage
- CLI doctor bundle JSON test

已验证：

- `go test ./...`
- `go build ./...`

## 10. Acceptance Criteria

当前状态：Phase 8 completed。

本轮已经满足：

- `doctor` 不再是静态占位
- `repair` 已进入 CLI
- diagnostics bundle 已可导出
- diagnostics bundle 已带出 probe snapshot 与受管日志摘要
- diagnostics bundle 已带出 host runtime / status / managed service 快照
- diagnostics bundle 已带出原始 command snapshot，便于脱离 CLI 复盘现场
- diagnostics bundle 已带出 managed service journal 摘要与 provider config snapshot
- 缺失 managed asset 已有恢复路径
- inactive managed service 已有探测与修复路径
- managed directory permission drift 已有修复路径
- rollback 已在 TUI 中具备历史查看、preview 和 apply 入口
- rollback 已在 CLI 中具备 history list/show 入口
- `status` 已能反映真实 managed state
- SELinux / firewalld / listening ports 已进入 preflight report
- php-fpm socket / managed path permission / DB TLS 也已进入 preflight report
- managed path ownership / SELinux context 也已进入 preflight report
- managed service active probe 已进入 preflight report
- managed DB/cache provider listener probe 已进入 preflight report
- managed DB protocol-level reachability probe 已进入 preflight report
- managed DB credentialed auth probe 已进入 preflight report
- managed DB provider metadata/TLS config 缺口已有 repair 路径
- TUI Doctor 已能区分哪些 warning 可由 repair 处理、哪些仍需手动处理
- suspicious managed SELinux label 在 root 场景下已有 `restorecon` repair 路径
- TUI History 已支持按状态过滤

已作为 post-Phase-8 backlog 下放：

- 更深的 DB/TLS/service sanity checks
- 更多 diagnostics bundle 深化项

## 11. Risks / Tradeoffs

- 当前 repair 仍偏保守，只修控制平面目录、recoverable DB provider metadata/TLS config、service 和 canonical site asset
- 当前 service repair 只自动处理明确 inactive/failed 的 systemd service
- 当前 owner/group 自动修复仅在 root 场景下启用
- `site.reconcile` 会重写受管配置，不处理手工文件的语义合并
- `status` 仍是 host-state summary，不是 full health dashboard
- diagnostics bundle 当前只采集受管元数据、基础 host info、probe 快照和日志摘要，不打包完整日志和站点代码目录
- 当前 rollback UI 仍只支持 latest pending entry 的执行，不支持从历史列表中任意挑选一条回滚
- 当前 DB reconcile 不会重建丢失的 credential secret 内容；secret 缺失仍需要人工恢复

## 12. Next Phase

Phase 8 已正式结项。

后续如继续推进，仅作为 post-Phase-8 backlog：

- 更深的 diagnostics/repair 能力
- 更细的 DB/service sanity checks

当前正式结项条件见：

- [closeout-review.md](/home/web-casa/llstack/dev_docs/PHASES/closeout-review.md)
