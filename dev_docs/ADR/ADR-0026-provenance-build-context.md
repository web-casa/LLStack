# ADR-0026: Provenance Build Context

## Status

Accepted

## Context

Phase 9 已生成 `provenance.json`，但内容只有 version、timestamp、builder tool 名称和 artifact 列表。无法回答"这个二进制是从哪个 commit 编译的"和"用什么工具链构建的"。

当前 `provenance.json` 缺少：

- git commit hash
- git ref / tag
- git repository URL
- Go 版本
- 构建平台信息

## Decision

在 `scripts/release/package.sh` 中补全 `provenance.json` 的构建上下文：

1. 新增 `source` 字段，包含 `repository`、`commit`、`ref`
2. 扩展 `builder` 字段，包含 `go_version`、`build_os`、`build_arch`、`build_host`
3. 将原 `sources` 字段重命名为 `references`，避免与 `source`（源码来源）混淆

所有值优先从环境变量读取（`LLSTACK_GIT_COMMIT` 等），fallback 到 `git` / `go` / `uname` 命令自动探测。

## Consequences

优点：

- 发布产物可追溯到具体 commit 和构建环境
- 不引入新工具、新格式、新依赖
- 与现有签名链完全兼容（`verify.sh` 使用 `grep -F` 校验，不依赖字段结构）

限制：

- 这仍然不是 SLSA 格式或 in-toto attestation
- `provenance.json` 仍依赖 detached signature，不是自签名 envelope
- 构建上下文基于 operator 机器，不是隔离构建环境
