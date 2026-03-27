# Phase 8 / Phase 9 Closeout Review

## Purpose

本文件用于固化 Phase 8 与 Phase 9 的正式结项条件，避免阶段状态判断继续分散在 `ROADMAP.md`、阶段文档和口头说明中。

结论优先：

- 当前代码与文档已经满足“功能面基本完成”的判断。
- 当前项目已可将 Phase 8 视为正式完成。
- 当前项目暂不建议把 Phase 9 直接标记为 `completed`。
- Phase 9 的剩余阻塞项已经收敛为单一 release gate，而不是功能缺口。

## Decision Record

日期：`2026-03-26`

本次结项判定结果：

- 最终采用 `Path A: Full Closeout`
- 过程说明：
  - 初始直接访问 `docker.sock` 失败
  - 后续确认 `sudo -n docker info` 可访问 daemon
  - 在具备 Docker 权限的路径下完成了真实全矩阵 smoke
- 决策：
  - Phase 8 标记为 `completed`
  - Phase 9 标记为 `completed`
  - 后续工作转入 release maintenance / backlog

## Current Snapshot

当前事实基线：

- Phase 8 的 `doctor / repair / rollback` 主链路已落地。
- Phase 9 的发布、安装、升级、release metadata、Docker smoke matrix 已落地。
- `go test ./...` 与 `go build ./...` 在最近开发轮次已持续通过。
- 文档已明确 Phase 8 / Phase 9 均已完成，当前进入 post-Phase-9 maintenance / backlog 阶段。

## Phase 8 Closeout Decision

### Already Met

- `doctor` 已提供真实 report，而不是占位输出。
- `repair` 已提供 dry-run / plan-only / apply。
- `rollback` 已在 CLI 与 TUI 中具备可见入口。
- diagnostics bundle 已包含：
  - probe snapshots
  - host runtime/status snapshots
  - managed service snapshots
  - command snapshots
  - provider config snapshots
  - journal/log tail 摘要
- 关键 preflight checks 已覆盖：
  - SELinux
  - firewalld
  - listening ports
  - php-fpm sockets
  - managed path ownership/permissions
  - managed SELinux contexts
  - DB TLS assets
  - managed services
  - managed provider ports
  - DB live probe
  - DB credentialed auth probe
- 关键 repair rules 已覆盖：
  - managed directory create/permission normalize
  - root 场景 owner/group 修复
  - site reconcile
  - inactive service start
  - suspicious SELinux path `restorecon`
  - firewalld required web ports
  - DB provider metadata/TLS config reconcile

### Remaining Before Formal Completion

- 无新的功能性阻塞项。
- 更深的 DB/service sanity checks 已下放为 post-Phase-8 backlog。

### Recommended Decision

- Phase 8 可正式标记为 `completed`。

## Phase 9 Closeout Decision

### Already Met

- release build / package / install / upgrade 主链路已落地。
- 本地文件、远程 URL、release index 安装链路已落地。
- `checksums.txt`、`index.json`、`sbom.spdx.json`、`provenance.json` 已生成。
- `verify.sh` 已校验：
  - package checksums
  - index consistency
  - SBOM presence/consistency
  - provenance consistency
- config-driven install 已支持：
  - YAML / JSON
  - nested schema
  - legacy flat compatibility
  - CLI override merge
- Docker functional matrix 已覆盖：
  - EL9/EL10
  - Apache/OLS/LSWS
- Docker smoke 已具备：
  - per-service artifacts
  - success marker assertions
  - Apache real service assertions
  - OLS / LSWS managed artifact assertions
- 仓库级 README、operator release guide、quickstart 文档入口已建立。

### Remaining Before Formal Completion

- 无新的正式结项阻塞项。
- artifact signing / trusted provenance chain 继续作为 post-Phase-9 backlog。

### Recommended Decision

- Phase 9 可正式标记为 `completed`。

## Closeout Paths

### Path A: Full Closeout

适用条件：

- 能执行真实 Docker daemon
- 能完成全矩阵 smoke

动作：

1. 执行真实 Docker smoke。
2. 将结果写回文档。
3. 把 Phase 8 / Phase 9 状态改为 `completed`。

### Path B: Conditional Closeout

已作为本次结项前的中间状态，不再是当前结论。

## Recommended Immediate Next Step

本次判定最终已完成 `Path A`。

真实执行记录：

- 日期：`2026-03-26`
- 方式：`sudo -n env PATH=\"$PATH\" LLSTACK_DOCKER_ARTIFACTS_DIR=... bash scripts/docker/functional-smoke.sh`
- 汇总：`/tmp/llstack-docker-smoke-final/summary.json`
- 结果：6/6 服务 `passed`

执行细则见：

- [phase-9-final-closeout-runbook.md](/home/web-casa/llstack/dev_docs/PHASES/phase-9-final-closeout-runbook.md)
