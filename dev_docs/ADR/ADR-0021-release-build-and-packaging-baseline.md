# ADR-0021: Release Build And Packaging Baseline

## Status

Accepted

## Context

LLStack 已具备较完整的 CLI/TUI 与生命周期能力，但仓库此前缺少正式的发布层基线：

- 无统一构建元数据
- 无 release build / package 脚本
- 无安装/升级脚本
- 无 e2e smoke 验证入口
- Docker functional 目录只有最薄骨架

Phase 9 需要先补“可交付产物”能力，再继续做更深的 UX polish 和发布流程。

## Decision

采用以下发布基线：

1. 二进制内置 build metadata
   - version
   - commit
   - build_date
   - target_os
   - target_arch
   - go_version

2. 提供仓库内 release scripts
   - `scripts/release/build.sh`
   - `scripts/release/package.sh`
   - `scripts/install.sh`
   - `scripts/upgrade.sh`

3. 提供 Makefile 统一入口
   - `build`
   - `build-cross`
   - `package`
   - `smoke`
   - `docker-smoke`

4. 发布包采用 `tar.gz`
   - 每个平台一个 release archive
   - 包内包含：
     - `bin/llstack`
     - `scripts/install.sh`
     - `scripts/upgrade.sh`
     - 关键兼容性与限制文档

5. 先建立最小可运行 e2e smoke
   - host smoke: `tests/e2e/smoke.sh`
   - docker smoke: `docker/fixtures/smoke.sh`

## Consequences

正面：

- 发布流程从“手工 go build”升级到可重复执行的 release baseline
- 安装/升级路径可测试
- 版本信息可追踪
- Docker functional 测试有统一 smoke 入口

负面：

- 目前仍是仓库内脚本驱动，不是完整 CI/CD pipeline
- release package 仍未包含签名、SBOM、artifact attestations
- install/upgrade 目前基于本地 binary/tarball，不负责网络下载分发
