# ADR-0022: Release Signatures With OpenSSL Detached Signatures

## Status

Accepted

## Context

Phase 9 已完成 `checksums.txt`、`index.json`、`sbom.spdx.json` 和 `provenance.json`，但仓库仍缺少一个可在本地环境直接执行、且不依赖外部托管服务的 artifact signing 基线。

我们需要一条满足下面条件的路线：

- 可在本地 shell / release operator 环境直接执行
- 可为 archives 与 metadata 生成 detached signatures
- 可在仓库测试中稳定验证
- 不强制引入 CI/CD、KMS 或第三方签名平台

## Decision

采用 `OpenSSL detached signatures` 作为当前 release signing 基线：

- 新增 `scripts/release/sign.sh`
- 使用 `openssl dgst -sha256 -sign <private-key>`
- 为 archives 与关键 metadata 文件生成 `.sig`
- 生成 `signatures.json` 作为签名清单
- `scripts/release/verify.sh` 在提供 `LLSTACK_VERIFY_PUBKEY` 时执行真实验签

## Consequences

优点：

- 无需外部服务
- 测试环境可本地生成临时 RSA keypair
- 与当前 `checksums + sbom + provenance` 基线兼容

限制：

- 这不是 trusted provenance chain
- 仍依赖 operator 侧私钥管理
- 不替代未来的 cosign / Sigstore / KMS 集成

## Follow-Up

后续可演进：

- 支持更多签名 provider
- 在 release pipeline 中强制 `REQUIRE_SIGNATURES`
- 将 detached signatures 升级为更强的 provenance / attestation 工作流
