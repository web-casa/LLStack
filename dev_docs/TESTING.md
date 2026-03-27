# LLStack Testing Strategy

## Phase 9 Closeout Gate

- `2026-03-26` 初始直接执行 `docker info` 返回 `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`
- 随后确认 `sudo -n docker info` 可访问 Docker daemon
- 已完成真实 Docker smoke：
  - EL9: `apache / ols / lsws`
  - EL10: `apache / ols / lsws`
- 汇总文件：
  - `/tmp/llstack-docker-smoke-final/summary.json`
- 汇总结果：
  - `overall_status=passed`
- 结论：
  - Phase 9 的真实 Docker smoke release gate 已满足

## Goals

- 在不依赖真实裸机环境的前提下，尽量验证 planner / renderer / rollback 逻辑
- 为 Docker 功能测试预留清晰边界
- 明确哪些是单元测试，哪些是真实服务测试，哪些只能有限验证

## Test Layers

### 1. Unit Tests

覆盖：

- canonical model
- validator
- planner
- renderer / compiler
- capability detection
- CLI 参数解析
- TUI 状态转换

要求：

- 快速执行
- 不依赖 systemd / dnf / 实际服务
- 大量使用 golden / snapshot

### 2. Integration Tests

覆盖：

- plan -> file ops 组装
- 配置写入与备份
- rollback 元数据生成
- provider 初始化逻辑
- PHP runtime binding
- 服务探测逻辑

策略：

- 使用 fake executor / fake fs / temp dir
- 少量使用容器内真实命令

### 3. Docker Functional Tests

目录约定：

- `docker/images/`
- `docker/fixtures/`
- `docker/compose/`

优先级：

1. Apache 真实安装、配置测试、reload
2. OLS 配置编译正确性与基本启动检查
3. LSWS 配置编译正确性、trial/licensed 探测模拟

### 4. Snapshot / Golden Tests

覆盖：

- Apache vhost 输出
- OLS 配置树输出
- LSWS 配置输出
- JSON plan 输出
- Phase 1 后加入 TUI 视图快照

## Docker Matrix

建议矩阵：

- EL9 x86_64 + Apache
- EL9 x86_64 + OLS
- EL9 x86_64 + LSWS compile-only
- EL10 x86_64 + Apache

后续扩展：

- aarch64 cross-validation

当前补充：

- `scripts/docker/functional-report.sh` 会对 `dist/docker-smoke/*.log` 生成 `summary.json`
- `tests/integration/docker/docker_test.go` 已覆盖 Docker smoke summary report 的成功/失败分支
- `tests/integration/docker/docker_test.go` 已覆盖 Dockerfile 不再写死 `GOARCH=amd64` 的回归检查
- `tests/integration/release/release_test.go` 已覆盖 OpenSSL detached signature 的 sign/verify 与 tamper failure
- `.github/workflows/ci.yml` 已覆盖 `go test`、`go build`、release build/package/verify、compose 校验，以及可选 detached signing 与 opt-in Docker smoke
- `.github/workflows/release.yml` 已复用 release build/package/sign/verify 链路，并在 tag 触发时创建 GitHub Release
- `tests/integration/release/release_test.go` 已覆盖 release version guard、release notes render 与 post-release summary script
- `tests/integration/release/release_test.go` 已覆盖 provider-neutral release pipeline 与 remote release verification script
- `tests/unit/install/profile_test.go` 已覆盖 scenario profile 解析与 defaults merge（含 WordPress/Laravel/API/Static/reverse-proxy）
- `tests/unit/doctor/service_test.go` 已覆盖 managed cache live probe
- 多 PHP 版本
- DB provider 组合

## VPS End-to-End Test Plan

完整的真实环境测试流程见 [VPS_TEST_PLAN.md](/home/web-casa/llstack/dev_docs/VPS_TEST_PLAN.md)，覆盖：

- Apache / OLS 真实安装链路
- WordPress / Laravel 真实应用部署
- PHP-FPM + MariaDB + Redis 完整服务栈
- 站点生命周期全操作
- Doctor 23 项 check 的真实环境验证
- Let's Encrypt 真实 ACME（需域名）

## What Docker Can Not Fully Prove

- 所有服务与裸机 systemd 行为完全等价
- LSWS 全授权能力
- 某些 SELinux / firewall 行为

这些场景需要：

- mock / stub / simulation
- 明确 capability caveat

## Phase 1 Test Skeleton

- `tests/unit/...`
- `tests/integration/...`
- `tests/golden/...`
- `testdata/golden/...`
- `docker/images/el9-apache/`
- `docker/images/el9-ols/`
- `docker/images/el10-apache/`
- `docker/compose/functional.yaml`

## Phase 2 Status

已落地：

- Apache renderer golden test
- site validator unit test
- CLI `site:create --dry-run --json` unit test
- site create/delete/rollback integration test（fake executor）

未落地：

- 真实 Apache service reload integration
- Docker 内 Apache 真服务测试

## Phase 3 Status

已落地：

- OLS compiler golden test
- OLS asset path / directory generation test
- OLS site create integration test（fake executor）
- CLI `site:create --backend ols --dry-run --json` test

未落地：

- 真实 OLS configtest / reload integration
- Docker 内 OLS 真服务测试

## Phase 4 Status

已落地：

- LSWS license detector unit test
- LSWS renderer golden test
- LSWS site create integration test（fake executor）
- CLI `site:create --backend lsws --dry-run --json` test

未落地：

- 真实 LSWS configtest / reload integration
- Docker 内 LSWS 真服务测试

## Phase 5 Status

已落地：

- Remi resolver unit test
- PHP adapter unit test
- PHP install integration test（fake executor）
- `site:php` integration test
- CLI `php:install --dry-run --json` test

未落地：

- 真实 EL9/EL10 dnf + Remi repo 安装功能测试
- PHP service health probe / version probe

## Phase 6 Status

已落地：

- DB provider resolver / TLS profile unit test
- DB install unit test
- DB lifecycle integration test（fake executor）
- CLI `db:install --dry-run --json` test
- cache install/configure unit test
- cache lifecycle integration test（fake executor）
- CLI `cache:install --dry-run --json` test

未落地：

- 真实 EL9/EL10 上的 MariaDB / MySQL / PostgreSQL / Percona 安装测试
- 真服务级 DB TLS 验证
- Redis / Memcached health probe

## Phase 7 Status

已落地：

- site profile unit test
- install orchestrator integration test
- site TLS/log lifecycle integration test
- certbot detection / command integration test
- scaffold + drift diff integration test
- wordpress / laravel scaffold integration test
- site start/stop integration test
- CLI `install --dry-run --json` test
- TUI site selection state test
- TUI site action confirm/state transition test
- TUI install plan preview state test
- TUI database setup text input + plan preview state test
- TUI site logs panel + TLS preview state test
- TUI site logs line-count/refresh state test
- TUI site TLS apply state test
- TUI site PHP switch state test
- TUI site create wizard state test
- TUI install apply state test
- TUI database apply state test
- site restart integration test
- TUI site restart state test
- site settings update integration test
- reverse proxy upstream update integration test
- TUI site edit wizard state test

未落地：

- `certbot` 真 ACME 功能测试

## Phase 8 Status

已落地：

- doctor service unit test
- repair dry-run unit test
- repair CLI `--dry-run --json` test
- repair integration test for missing managed asset recovery
- doctor preflight warning test for missing web ports
- doctor preflight warning test for missing php-fpm sockets
- doctor preflight warning test for suspicious managed SELinux labels
- doctor preflight warning test for missing DB TLS assets
- doctor preflight warning test for inactive managed service
- doctor preflight warning test for missing managed provider listener ports
- doctor preflight warning test for managed DB live probe failure
- doctor preflight warning test for managed cache live probe failure
- repair integration test for managed service auto-start
- repair managed directory permission normalization tests
- repair CLI warning/detail text coverage
- TUI plan preview detail/redaction coverage
- TUI doctor page preview/apply coverage
- TUI history page preview/apply coverage
- CLI rollback:list/show coverage
- diagnostics bundle unit test
- diagnostics bundle cache metadata coverage
- diagnostics bundle probe snapshot and log tail coverage
- diagnostics bundle host/status/service snapshot coverage
- diagnostics bundle raw command snapshot coverage
- diagnostics bundle extended host probe snapshot coverage
- diagnostics bundle managed service journal snapshot coverage
- diagnostics bundle provider config snapshot coverage
- doctor DB connection saturation check coverage
- doctor cache memory saturation check coverage
- diagnostics bundle site/php/db/cache snapshot coverage
- diagnostics bundle ps-aux process snapshot coverage
- diagnostics bundle TLS cert expiration snapshot coverage
- doctor PHP-FPM process health check coverage
- doctor PHP config drift check coverage
- certbot failure/missing-param/webroot test coverage (7 new tests)
- PHP uninstall / TunePool API coverage
- DB uninstall / backup API coverage
- CLI doctor bundle JSON test
- DB credential file lifecycle coverage
- doctor DB auth probe coverage
- doctor DB reconcile plan coverage
- TUI history filter coverage
- TUI doctor filter coverage
- TUI doctor repair coverage grouping coverage
- firewalld repair plan/apply coverage

未落地：

- socket / permission / DB/TLS checks integration
- diagnostics bundle 内容完整性扩展测试

## Phase 9 Status

已落地：

- version build metadata output coverage
- release build/package/install/upgrade/smoke integration test
- remote URL install/upgrade coverage
- release checksum verification coverage
- SPDX SBOM generation coverage
- provenance manifest coverage
- release metadata consistency failure coverage
- release index install/upgrade coverage
- install profile load + CLI override coverage
- nested install profile coverage
- host e2e smoke script
- Docker functional smoke fixture
- Docker compose config validation test
- Docker compose service matrix validation test
- opt-in Docker functional smoke test
- Docker smoke artifact + passed-marker assertion design
- Apache Docker smoke service-level assertion design
- OLS / LSWS Docker managed-asset assertion design
- OLS Docker runtime verification (configtest + HTTP request)
- EL9/EL10 × Apache/OLS/LSWS Docker service matrix
- README/release guide examples aligned to shipped scripts and Makefile entrypoints
- publish directory provider coverage
- publish pipeline integration coverage
- publish provider validation coverage

- provenance build context field coverage (commit, ref, repository, go_version, build_os, build_arch)

未落地：

- 更广的 Docker functional 执行矩阵
- transparency log 测试
