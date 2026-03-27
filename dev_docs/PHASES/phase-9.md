# Phase 9: 打磨与发布

## 1. Goal

- 建立 LLStack 的正式发布基线
- 补可交付二进制、打包、安装、升级与 smoke 验证入口
- 为后续 UX polish 与 e2e/Docker 扩展建立稳定基础

## 2. Scope

本轮完成：

- 二进制 build metadata 接入
- `version` 命令输出构建元数据
- Makefile 基线
- release build / package 脚本
- install / upgrade 脚本
- install / upgrade 远程 URL 下载模式
- install / upgrade 的 sha256 校验参数
- SPDX SBOM 产物
- provenance baseline manifest
- detached signature 产物与验签入口
- release index 自动解析安装脚本
- 配置文件驱动安装（YAML / JSON）
- install config nested schema（兼容 legacy flat）
- CLI help/usage polish（命令分组、示例、flag error hint）
- 仓库根 README 与 release/operator 指南
- GitHub Actions CI skeleton
- host e2e smoke 脚本
- Docker functional smoke fixture
- Docker functional 执行脚本
- Docker functional artifacts 与 success-marker 断言
- Apache Docker smoke 的真实服务断言
- OLS / LSWS Docker smoke 的真实受管产物断言
- Dockerfile / compose 的 functional baseline
- release/package/install/upgrade/smoke 集成测试
- Docker compose config 校验测试与 opt-in Docker smoke 测试

本轮不做：

- 完整 publish/release automation
- 受信 provenance 链 / transparency log integration
- UX polish 的大规模视觉/交互重构

## 3. Decisions / ADR

- `ADR-0021-release-build-and-packaging-baseline.md`
- `ADR-0022-release-signatures-with-openssl.md`
- `ADR-0023-github-actions-ci-skeleton.md`

## 4. Architecture

新增发布链路：

`go build + ldflags -> build metadata -> release artifacts`

`Makefile -> scripts/release/build.sh -> dist/releases/<version>/<platform>/llstack`

`scripts/release/package.sh -> dist/packages/<version>/llstack-<version>-<platform>.tar.gz`

`scripts/install.sh / scripts/upgrade.sh -> local install path`

`scripts/install.sh --from <url> -> download -> install`

`scripts/release/verify.sh -> validate checksums.txt against packaged archives`

`scripts/release/package.sh -> emit checksums.txt + index.json + sbom.spdx.json`

`scripts/release/package.sh -> emit provenance.json`

`scripts/release/sign.sh -> emit detached signatures + signatures.json`

`scripts/install-release.sh --index <index.json> -> resolve platform archive + sha256 -> install/upgrade`

`scripts/install-release.sh --pubkey <pubkey> -> verify index/archive detached signatures -> install/upgrade`

`llstack install --config <yaml|json> -> profile load -> CLI override -> unified install plan`

`tests/e2e/smoke.sh -> built binary smoke validation`

`README.md -> install/release/docker smoke/documentation entrypoint`

`dev_docs/RELEASE_OPERATIONS.md -> operator-facing release workflow guide`

`scripts/docker/functional-smoke.sh -> sequential Docker functional smoke runner`

`docker/fixtures/smoke.sh -> Docker functional smoke entrypoint`

`dist/docker-smoke/<service>.log -> Docker functional artifacts`

`.github/workflows/ci.yml -> test/build/package/verify baseline + opt-in Docker smoke`

## 5. Data Models

新增：

- `buildinfo.Info`

扩展：

- `cli.Dependencies`
- `versionPayload`

## 6. File Tree Changes

新增：

- `internal/buildinfo/buildinfo.go`
- `README.md`
- `dev_docs/RELEASE_OPERATIONS.md`
- `.github/workflows/ci.yml`
- `scripts/release/build.sh`
- `scripts/release/package.sh`
- `scripts/release/sign.sh`
- `scripts/install.sh`
- `scripts/upgrade.sh`
- `scripts/install-release.sh`
- `tests/e2e/smoke.sh`
- `tests/integration/release/release_test.go`
- `tests/integration/docker/docker_test.go`
- `docker/fixtures/smoke.sh`
- `scripts/docker/functional-smoke.sh`
- `Makefile`
- `examples/quickstart/README.md`

更新：

- `cmd/llstack/main.go`
- `internal/app/app.go`
- `internal/cli/root.go`
- `internal/cli/version.go`
- `docker/images/*/Dockerfile`
- `docker/compose/functional.yaml`

## 7. Commands / UX Flows

新增工作流：

- `make build`
- `make build-cross PLATFORMS="linux/amd64 linux/arm64"`
- `make package`
- `make sign-release SIGNING_KEY=/path/to/key.pem SIGNING_PUBKEY=/path/to/pub.pem`
- `make verify-release`
- `make smoke`
- `make docker-smoke`
- `make install-release INDEX=https://example.invalid/index.json PLATFORM=linux-amd64`
- `LLSTACK_RUN_DOCKER_TESTS=1 go test ./tests/integration/docker -run TestDockerFunctionalSmoke -v`
- `bash scripts/install.sh --from dist/packages/<version>/llstack-<version>-<platform>.tar.gz`
- `bash scripts/install.sh --from dist/packages/<version>/llstack-<version>-<platform>.tar.gz --sha256 <hex>`
- `bash scripts/install.sh --from dist/packages/<version>/llstack-<version>-<platform>.tar.gz --pubkey /path/to/pub.pem --require-signature`
- `bash scripts/install.sh --from https://example.invalid/llstack-<version>-<platform>.tar.gz`
- `bash scripts/upgrade.sh --from dist/packages/<version>/llstack-<version>-<platform>.tar.gz`
- `bash scripts/install-release.sh --index dist/packages/<version>/index.json --platform linux-amd64 --pubkey /path/to/pub.pem --require-signature`
- `llstack install --config examples/install/basic.yaml --json`
- `llstack --help`
- `llstack install --help`

增强：

- `llstack version`
- `llstack version --json`

## 8. Code

当前已落地：

- `llstack version` 现在展示：
  - version
  - commit
  - build date
  - target os/arch
  - go version
- release build 支持 `ldflags` 注入元数据
- package 产物会包含 install/upgrade 脚本和关键文档
- package 现在会额外生成 `index.json`
- package 现在会额外生成 `sbom.spdx.json`
- package 现在会额外生成 `provenance.json`
- release signing 现在支持 OpenSSL detached signatures
- release signing 现在会额外生成 `signatures.json` 与 detached `.sig` files
- package 校验现在由 `checksums.txt` + `scripts/release/verify.sh` 统一处理
- `verify.sh` 在提供 public key 时现在可执行真实 detached signature verification
- release index 安装现在可直接从 `index.json` 解析对应平台包
- release index 安装现在可对 `index.json` 与目标 archive 做 detached signature verification
- install 现在支持 YAML / JSON profile 输入，并允许 CLI flag 覆盖 profile 值
- install profile 现在优先使用 nested schema，同时保持 legacy flat 兼容
- root/install/site:create/doctor/tui 帮助文本现在带有更完整的 example
- root help 现在按 Getting Started / Site Lifecycle / Runtime / Diagnostics / Interfaces 分组展示
- flag error 现在会附带 `run '<command> --help' for usage` 提示
- 仓库现在有正式 README 作为安装、发布、smoke 与文档导航入口
- release/operator 流程现在有独立文档，不再只散落在脚本和 quickstart 示例中
- 仓库现在有 GitHub Actions CI skeleton，覆盖 test/build/package/verify 与 opt-in Docker smoke
- smoke 脚本会验证：
  - `version`
  - `status`
  - `install --dry-run`
  - `site:create --dry-run`
  - `doctor --json`
- Docker functional smoke 会按服务顺序执行 compose `config -> up --build -> logs -> down`
- Docker functional smoke 现在会从 compose 自动发现服务、写出 per-service artifact，并校验 structured passed marker
- Apache Docker smoke 现在会执行真实 `httpd` apply/configtest/reload/vhost/HTTP 响应断言
- OLS / LSWS Docker smoke 现在会断言受管 config asset、listener/include 和 parity report 实际落盘

## 9. Tests

新增：

- release build/package/install/upgrade/smoke integration test
- version build metadata output coverage
- remote URL install/upgrade coverage
- release checksum verification coverage
- SPDX SBOM generation coverage
- provenance manifest coverage
- detached signature sign/verify coverage
- detached signature tamper failure coverage
- install/index detached signature verification coverage
- release metadata consistency failure coverage
- release index install/upgrade coverage
- install profile load + CLI override coverage
- nested install profile coverage
- root/install help coverage
- flag error help-hint coverage
- GitHub Actions workflow baseline
- Docker compose config validation
- Docker compose service-matrix validation
- opt-in Docker functional smoke test
- OLS / LSWS Docker managed-asset assertion design

已验证：

- `go test ./...`
- `go build ./...`

## 10. Acceptance Criteria

当前状态：Phase 9 completed。

本轮已经满足：

- 仓库内已有统一 release build/package 入口
- 二进制具备可追踪构建元数据
- 安装与升级已有可执行脚本
- 安装与升级已支持本地文件和远程 URL 两种来源
- 安装与升级已支持可选 sha256 校验
- 安装与升级已支持可选 detached signature verification
- release package 已生成 SPDX SBOM
- release package 已生成 provenance baseline manifest
- release package 已生成 detached signatures 与 `signatures.json`
- 发布索引安装已支持按平台自动选择包
- 发布索引安装已支持可选 detached signature verification
- 配置文件驱动安装已支持 nested schema，并保留 legacy flat 兼容
- 仓库级文档入口已经建立，安装/升级/发布/Docker smoke 有正式文档路径
- 仓库级 CI baseline 已建立
- host smoke 与 Docker smoke 已有统一入口
- Docker functional 已具备可执行 runner 和 opt-in 测试入口
- Docker functional runner 现在会写出 artifacts 并对成功标记做显式断言
- Docker functional report 现在会生成 `dist/docker-smoke/summary.json`
- Docker build 现在跟随 `TARGETOS/TARGETARCH`，不再把 smoke 镜像内的 `llstack` 固定编译为 `amd64`
- Apache Docker smoke 已具备真实服务级断言；OLS 仍以 compile/smoke-first 为主
- LSWS 现已进入 Docker smoke 矩阵，但仍是 compile/smoke-first，不是 licensed service runtime test
- Docker smoke matrix 现已覆盖 EL9/EL10 × Apache/OLS/LSWS
- release verify 现在不仅校验 tarball checksum，也校验 index/SBOM/provenance 一致性
- GitHub Actions workflow 现在会自动执行 test/build/package/verify，并在配置 secrets 时执行 signing + verify
- release 层已进入仓库测试覆盖

真实收口记录：

- `2026-03-26` 初始直接访问 `docker.sock` 失败
- 随后确认 `sudo -n docker info` 可访问 Docker daemon
- 已完成真实全矩阵 smoke：
  - `el9-apache`
  - `el9-ols`
  - `el9-lsws`
  - `el10-apache`
  - `el10-ols`
  - `el10-lsws`
- 汇总结果：`/tmp/llstack-docker-smoke-final/summary.json`
- 结果：`overall_status=passed`

## 11. Risks / Tradeoffs

- 远程安装目前依赖 `curl` 或 `wget`
- Docker functional baseline 目前是 smoke-first；默认测试会校验 compose config 和 service matrix，真实容器 smoke 仍需显式 opt-in
- detached signing 已完成，但 trusted provenance chain / transparency log integration 仍未完成

## 12. Next Phase

Phase 9 已正式结项。

后续如继续推进，转入 post-Phase-9 backlog：

1. SLSA 格式 provenance / signed envelope / transparency log
2. 更广泛的 Docker / e2e matrix
3. CI/CD workflow 完整化

### Post-Phase-9 已完成增量

- `provenance.json` 补全构建上下文：git commit、repository、ref、Go version、build OS/arch/host
- `provenance.json` 字段结构调整：`sources` 重命名为 `references`，新增 `source` 字段
- 测试增加 provenance build context 字段覆盖
- ADR-0026 记录决策
- OLS Docker smoke 增加运行时验证：安装 openlitespeed、systemctl shim、configtest、HTTP 请求
- OLS Dockerfile 从 asset-only 升级为含真实 OLS 服务
- LSWS 仍保持 asset-first（需真实授权）
- ADR-0027 记录决策
- 新增 `scripts/release/publish.sh`，支持 `github` 和 `directory` provider
- `pipeline.sh` 增加可选 `RUN_PUBLISH` 步骤
- `release.yml` 改用 `publish.sh` 替代 `softprops/action-gh-release`
- `Makefile` 增加 `publish-release` 目标
- ADR-0028 记录决策
- doctor 新增 `db_connection_saturation` 和 `cache_memory_saturation` checks
- TUI install wizard 新增 scenario 选择器（wordpress/laravel/api/static/reverse-proxy）
- scenario profile 测试补全（WordPress/Laravel/API/Static 4 个新测试）
- diagnostics bundle 新增 `ps aux` 进程快照和 TLS 证书到期快照
- doctor 新增 `php_fpm_process_health` 和 `php_config_drift` checks
- TUI site create wizard 扩展 aliases / docroot 字段（7 字段）
- certbot 测试骨架增强：失败场景、参数验证、webroot 覆盖（7 个新测试）
- OLS 主配置自动管理：site:create/delete 自动注册/注销 virtualhost + listener map（ADR-0029）
- 跨子系统 install rollback：失败时报告已完成步骤 + 清理指引
- repair 支持 PHP config drift 自动修复
- TUI site edit wizard 扩展 php_version / tls 字段
- 签名策略默认强制化：--pubkey 提供时默认 require-signature，增加 --skip-signature
- aarch64 Docker smoke 支持（LLSTACK_DOCKER_PLATFORM 环境变量）
- PHP uninstall + pool tuning API
- DB uninstall + backup API（mysqldump / pg_dumpall）
- TUI logs follow 模式（tea.Tick 2s 自动刷新）+ grep 关键词过滤
- Per-site 用户隔离 + 硬件检测 + 调优引擎
- PHP 7.4-8.5 + Valkey + OLS/LSWS suEXEC
- Per-site PHP 配置覆盖（三后端统一）
- SSL 证书生命周期管理（systemd timer 自动续签）
- Cron 任务管理（预设 wp-cron + laravel-scheduler）
- 安全加固子系统（fail2ban / IP 黑名单 / rate limiting / firewalld）
- OLS .htaccess 兼容性检测和自动转换

当前正式结项条件见：

- [closeout-review.md](/home/web-casa/llstack/dev_docs/PHASES/closeout-review.md)
- [phase-9-final-closeout-runbook.md](/home/web-casa/llstack/dev_docs/PHASES/phase-9-final-closeout-runbook.md)
