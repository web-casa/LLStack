# ADR-0028: Provider-Neutral Publish Orchestration

## Status

Accepted

## Context

LLStack 的发布链路中，build/package/sign/verify 已由 provider-neutral 的仓库脚本驱动。但"发布到哪"这一步仍绑定在 GitHub Actions workflow 中：

- 使用 `softprops/action-gh-release` 创建 GitHub Release
- 使用 `gh release view` 获取已发布 asset 列表
- 无法从本地 CLI 或非 GitHub 环境执行发布

如果要发布到 GitLab、自建镜像站或 S3，需要重写 workflow。

## Decision

新增 `scripts/release/publish.sh`，作为 provider-neutral 的发布入口：

1. **github provider** — 使用 `gh` CLI 创建 Release 并上传 asset，不依赖 GitHub Actions 特有 API
2. **directory provider** — 复制 release artifact 到目标目录，适用于本地 web server / S3 挂载 / 任意文件系统

输出约定：

- `release-assets.txt` — 已发布 asset 列表（与 `post-release-report.sh` 兼容）
- `release-url.txt` — 发布 URL

集成方式：

- `pipeline.sh` 增加 `RUN_PUBLISH` 步骤，可选调用 `publish.sh`
- `release.yml` 改用 `publish.sh --provider github` 替代 `softprops/action-gh-release`
- `Makefile` 增加 `publish-release` 目标

## Consequences

优点：

- 发布全链路可从任意环境执行，不绑定 GitHub Actions
- 新增 provider 只需在 `publish.sh` 增加一个 case 分支
- 与现有 verify-remote / post-release-report 链路完全兼容

限制：

- `github` provider 依赖 `gh` CLI 和有效 token
- `directory` provider 不负责远程同步（rsync / s3 sync 等由 operator 自行处理）
- 当前只有 `github` 和 `directory` 两个 provider，更多 provider 按需添加
