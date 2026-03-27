# Phase 4: LiteSpeed Enterprise 后端

## 1. Goal

- 为 LSWS 增加 backend
- 保持 Apache 风格配置主导
- 支持 LiteSpeed 专用指令注入层
- 支持 trial / licensed 模式识别
- 增加 enterprise-only feature flags

## 2. Scope

做了什么：

- 为 runtime config 新增 LSWS 配置
- 实现 LSWS license detector
- 实现 LSWS renderer
- 实现 LSWS verifier 接线
- 将 site manager 扩展到 `lsws` backend
- 在 manifest 中记录 backend capabilities
- 支持 `site:create --backend lsws`
- 增加 LSWS golden tests
- 增加 LSWS integration test

没有做什么：

- 未实现真实 LSWS service 验证
- 未实现 LSWS 高级缓存/WAF/QUIC 真实运行时配置
- 未实现 `site:update` 的 directive merge

## 3. Decisions / ADR

新增 ADR：

- `ADR-0011-lsws-license-detection-and-managed-includes.md`

## 4. Architecture

当前 LSWS 链路：

`CLI -> site.Manager -> lsws.Detector -> lsws.Renderer -> managed include + parity report -> apply.FileApplier -> lsws.Verifier -> rollback.History`

LSWS 当前输出：

- Apache 风格主配置 include
- parity report JSON
- capability snapshot（写入 manifest）

当前 LSWS managed 路径：

- include config: `/usr/local/lsws/conf/llstack/includes/<site>.conf`
- parity report: `/var/lib/llstack/state/parity/<site>.lsws.json`
- license serial 默认路径：`/usr/local/lsws/conf/serial.no`

## 5. Data Models

Phase 4 新增/扩展：

- `model.LSWSOptions`
- `model.BackendCapabilities`
- `model.SiteManifest.Capabilities`
- `render.SiteRenderResult.Capabilities`

## 6. File Tree Changes

新增核心文件：

- `internal/backend/lsws/license.go`
- `internal/backend/lsws/renderer.go`
- `internal/backend/lsws/verifier.go`
- `tests/unit/backend/lsws/renderer_test.go`
- `testdata/golden/lsws/basic_vhost.conf`
- `testdata/golden/lsws/basic_parity.json`

更新核心文件：

- `internal/site/service.go`
- `internal/config/config.go`
- `internal/core/model/site.go`
- `internal/core/render/render.go`
- `internal/core/validate/site.go`
- `internal/cli/site.go`
- `tests/integration/site/site_test.go`
- `tests/unit/cli/root_test.go`

## 7. Commands / UX Flows

当前新增能力：

- `llstack site:create example.com --backend lsws --non-interactive --dry-run`
- `llstack site:create example.com --backend lsws --non-interactive --skip-reload`
- `llstack site:list` 可显示 license mode（若 manifest 中存在）

行为说明：

- 若未检测到 license，plan 中会显式 warning
- 当前 `license=unknown` 时 enterprise flags 采用保守值
- LSWS 仍走统一 manifest / rollback 链路

## 8. Code

LSWS 当前覆盖：

- Apache 风格 vhost 渲染
- LiteSpeed 自定义指令注入
- lsphp 命令行注入
- trial / licensed / unknown 模式检测
- enterprise feature flags 快照
- parity report 输出

当前能力 flags：

- `directive_injection`
- `quic`
- `cache`
- `esi`

## 9. Tests

新增测试：

- LSWS license detector unit test
- LSWS renderer golden test
- LSWS create integration test
- CLI `site:create --backend lsws --dry-run --json` test

已验证：

- `go test ./...`
- `go build ./...`
- CLI dry-run 冒烟：`site:create --backend lsws`

## 10. Acceptance Criteria

本阶段验收结论：已满足 Phase 4 的最小目标。

- LSWS backend 已接入
- Apache 风格配置主导已落地
- LiteSpeed directive injection 已落地
- trial / licensed / unknown 检测已落地
- enterprise-only feature flags 已落地

## 11. Risks / Tradeoffs

- `license=unknown` 时能力判定是保守策略，不代表真实 LSWS 环境能力缺失
- 当前 feature flags 是 capability snapshot，不是运行时深度探测
- 未在真实 LSWS 环境执行 configtest/reload
- 目前只有 create/delete/rollback，未实现 update/merge

## 12. Next Phase

Phase 5 将实现：

- Remi `php-litespeed` 子系统
- 多 PHP 版本安装与并存
- per-site PHP runtime binding
- PHP adapter 统一化
