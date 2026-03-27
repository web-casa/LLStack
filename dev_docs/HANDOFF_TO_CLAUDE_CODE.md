# LLStack Handoff To Claude Code

## 1. Purpose

本文件用于在切换到 Claude Code 后，快速让新代理接手 LLStack 的当前真实状态，避免重复探索、重复设计或破坏既有架构约束。

当前基线日期：`2026-03-26`

## 2. Project Snapshot

LLStack 是一个面向 EL9 / EL10 的 CLI + TUI Web Stack Installer 与 Site Lifecycle Manager。

当前阶段结论：

- Phase 0 到 Phase 9 已完成
- 当前处于 `post-Phase-9 backlog / release maintenance`
- 发布链路、Docker functional smoke matrix、doctor/repair/rollback、site lifecycle、PHP/DB/cache 主链路都已落地

当前已完成的关键能力：

- 三后端：
  - Apache
  - OpenLiteSpeed
  - LiteSpeed Enterprise
- Apache semantics -> canonical model -> backend render/compile 主架构
- 多版本 PHP：
  - Apache 使用 `php-fpm`
  - OLS / LSWS 使用 Remi `php-litespeed`
- DB / cache provider：
  - MariaDB / MySQL / PostgreSQL / Percona
  - Memcached / Redis
- doctor / repair / rollback / diagnostics bundle
- release build / package / install / upgrade / sign / verify
- config-driven install：
  - nested schema
  - legacy flat compatibility
  - scenario profile defaults
- Docker functional smoke：
  - EL9 / EL10
  - Apache / OLS / LSWS
  - 最近一次真实全矩阵 smoke 已通过

## 3. Must-Read Files

Claude Code 接手前，优先阅读这些文件：

1. [ROADMAP.md](/home/web-casa/llstack/dev_docs/ROADMAP.md)
2. [ARCHITECTURE.md](/home/web-casa/llstack/dev_docs/ARCHITECTURE.md)
3. [TESTING.md](/home/web-casa/llstack/dev_docs/TESTING.md)
4. [KNOWN_LIMITATIONS.md](/home/web-casa/llstack/dev_docs/KNOWN_LIMITATIONS.md)
5. [closeout-review.md](/home/web-casa/llstack/dev_docs/PHASES/closeout-review.md)
6. [phase-9.md](/home/web-casa/llstack/dev_docs/PHASES/phase-9.md)
7. [RELEASE_OPERATIONS.md](/home/web-casa/llstack/dev_docs/RELEASE_OPERATIONS.md)

关键代码入口：

- app / dependency wiring:
  - [app.go](/home/web-casa/llstack/internal/app/app.go)
- CLI:
  - [root.go](/home/web-casa/llstack/internal/cli/root.go)
  - [install.go](/home/web-casa/llstack/internal/cli/install.go)
  - [site.go](/home/web-casa/llstack/internal/cli/site.go)
  - [doctor.go](/home/web-casa/llstack/internal/cli/doctor.go)
- TUI:
  - [model.go](/home/web-casa/llstack/internal/tui/model.go)
- site lifecycle:
  - [service.go](/home/web-casa/llstack/internal/site/service.go)
- install orchestration:
  - [service.go](/home/web-casa/llstack/internal/install/service.go)
  - [profile.go](/home/web-casa/llstack/internal/install/profile.go)
- doctor / repair / bundle:
  - [service.go](/home/web-casa/llstack/internal/doctor/service.go)
  - [bundle.go](/home/web-casa/llstack/internal/doctor/bundle.go)
- release / packaging:
  - [pipeline.sh](/home/web-casa/llstack/scripts/release/pipeline.sh)
  - [package.sh](/home/web-casa/llstack/scripts/release/package.sh)
  - [verify.sh](/home/web-casa/llstack/scripts/release/verify.sh)
  - [verify-remote.sh](/home/web-casa/llstack/scripts/release/verify-remote.sh)
- Docker functional:
  - [functional-smoke.sh](/home/web-casa/llstack/scripts/docker/functional-smoke.sh)
  - [smoke.sh](/home/web-casa/llstack/docker/fixtures/smoke.sh)
  - [docker_test.go](/home/web-casa/llstack/tests/integration/docker/docker_test.go)

## 4. Non-Negotiable Architecture Constraints

这些约束不能擅自改：

1. Apache VirtualHost semantics 是 single source of truth。
2. 用户不直接维护 OLS 原生配置；OLS 必须由 canonical model 编译生成。
3. 所有写操作都应走统一的 `plan -> apply -> verify -> rollback metadata` 主链路。
4. 所有系统命令必须通过统一 executor / orchestrator 路径，不要把 shell 调用散落到全项目。
5. Apache backend 的 PHP runtime 是 `php-fpm`，不要改回 `php-litespeed`。
6. OLS / LSWS 的 PHP runtime 基于 Remi `php-litespeed`，不要引入 LiteSpeed 官方 `lsphp` 安装链路。
7. 默认站点根目录是 `/data/www/<site>`。
8. 不要引入 Web GUI；产品核心仍是 CLI + TUI。
9. 不要绕过 `dev_docs/`；所有关键架构/阶段变更都必须同步文档和 ADR。

## 5. Current Release / Test Baseline

最近稳定基线：

- `go test ./...` 通过
- `go build ./...` 通过
- 真实 Docker functional smoke 全矩阵通过

常用验证命令：

```bash
go test ./...
go build ./...
make smoke
make docker-smoke
make package
make verify-release
```

如果只验证发布链：

```bash
make release-pipeline MODE=validate VERSION=0.1.0-dev
make remote-verify-release RELEASE_BASE_URL=https://example.invalid/releases/0.1.0-dev VERSION=0.1.0-dev
```

Docker smoke 产物：

- [summary.json](/home/web-casa/llstack/dist/docker-smoke/summary.json)

## 6. Current Backlog

### High Priority

1. trusted provenance chain / transparency-log style attestation  
当前已有：
- `checksums.txt`
- `sbom.spdx.json`
- detached signatures
- `provenance.json`

已补全：
- provenance 构建上下文（git commit / Go version / build platform）

仍缺：
- SLSA 格式 / signed envelope
- 透明日志或第三方 attestation 集成

2. deeper OLS / LSWS runtime verification
OLS Docker smoke 已增加运行时验证（安装 openlitespeed、configtest、HTTP 请求）。LSWS 仍为 asset-first（需真实授权）。

3. provider-neutral multi-provider publish orchestration
已实现 `publish.sh`，支持 `github` 和 `directory` provider。GitHub Actions workflow 已改用 `publish.sh`。更多 provider（GitLab / S3 等）按需添加。

### Medium Priority

4. deeper DB / service sanity checks  
Phase 8 主链路已完整，但更深的 provider-specific live checks、repair 和数据层诊断仍可继续扩展。

5. TUI / scenario model 产品化  
当前 scenario profile 仍是 defaults merge，不是完整 scenario graph / dependency solver。

6. diagnostics bundle 扩展  
当前已很完整，但仍可增加更深入的服务级证据和远程校验材料。

## 7. Recommended Next Work Order

建议 Claude Code 按这个顺序推进：

1. `trusted provenance chain`
2. `OLS / LSWS deeper runtime verification`
3. `provider-neutral publish orchestration`
4. `DB / service sanity checks`
5. `scenario model / TUI polish`

## 8. Development Rules For Handoff

1. 不要重做已完成阶段。
2. 不要把 post-Phase-9 backlog 混回 Phase 9 文档里，除非明确说明是 backlog extension。
3. 修改任何关键行为前，先检查是否冲突于：
   - ADR
   - `ARCHITECTURE.md`
   - `KNOWN_LIMITATIONS.md`
4. 改代码后必须同步：
   - 对应阶段文档或 backlog 文档
   - `ROADMAP.md`
   - `TESTING.md`
   - 必要时新增 ADR
5. 如果遇到高影响决策，先向用户确认，不要擅自改变产品边界。

## 9. Suggested First Session For Claude Code

第一轮接手建议只做这些：

1. 阅读 must-read 文档
2. 运行：
   - `go test ./...`
   - `go build ./...`
3. 检查：
   - [pipeline.sh](/home/web-casa/llstack/scripts/release/pipeline.sh)
   - [verify-remote.sh](/home/web-casa/llstack/scripts/release/verify-remote.sh)
   - [service.go](/home/web-casa/llstack/internal/doctor/service.go)
   - [smoke.sh](/home/web-casa/llstack/docker/fixtures/smoke.sh)
4. 从高优先级 backlog 中只选一个主题推进
5. 更新 `dev_docs/`

## 10. Handoff Summary

可以把当前项目理解为：

- 核心产品架构已经稳定
- Phase 0 到 Phase 9 已完成
- 当前不是从 0 到 1，而是从“可交付基线”继续往“更可信、更深验证、更强发布链”推进

Claude Code 最重要的工作不是重写，而是在不破坏现有 canonical architecture 的前提下，继续做高价值深化。
