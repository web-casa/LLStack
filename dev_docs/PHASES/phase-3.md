# Phase 3: OpenLiteSpeed 配置编译器

## 1. Goal

- 将 canonical model 编译为 OLS 原生配置
- 自动创建 vhost / listener map / extprocessor / script handler / TLS 配置
- 输出 parity report
- 让 OLS backend 可独立启用
- 增加 OLS renderer golden tests 与目录生成测试

## 2. Scope

做了什么：

- 为运行时配置新增 OLS managed layout
- 将 `site.Manager` 升级为多 backend 调度层
- 实现 OLS compiler
- 实现 OLS verifier 接线
- 为 OLS 输出 parity report JSON
- 将 manifest 升级为 managed asset 清单
- `site:create --backend ols` 接入统一 plan/apply/rollback 流程
- 增加 OLS golden tests
- 增加 OLS 目录/资产路径测试
- 增加 OLS integration test

没有做什么：

- 未实现 LSWS
- 未实现完整 OLS header/access control 编译
- 未在真实 OLS 环境执行 configtest / reload
- 未实现 Apache -> OLS 反向 parser

## 3. Decisions / ADR

新增 ADR：

- `ADR-0010-ols-managed-layout-and-parity.md`

## 4. Architecture

当前 OLS 链路：

`CLI -> site.Manager -> validate -> ols.Compiler -> OLS assets + parity report -> apply.FileApplier -> ols.Verifier -> rollback.History`

OLS compiler 输出：

- `vhconf.conf`
- listener map
- parity report JSON

当前 OLS managed 路径：

- vhost config: `/usr/local/lsws/conf/vhosts/<site>/vhconf.conf`
- listener map: `/usr/local/lsws/conf/llstack/listeners/<site>.map`
- parity report: `/var/lib/llstack/state/parity/<site>.ols.json`

## 5. Data Models

Phase 3 新增/扩展：

- `render.ParityStatus`
- `render.ParityItem`
- `render.ParityReport`
- `model.SiteManifest.ManagedAssetPaths`
- `model.SiteManifest.ParityReportPath`
- `rollback.Entry.Backend`

## 6. File Tree Changes

新增核心文件：

- `internal/backend/ols/compiler.go`
- `internal/backend/ols/verifier.go`
- `tests/unit/backend/ols/compiler_test.go`
- `testdata/golden/ols/basic_vhconf.conf`
- `testdata/golden/ols/basic_listener.map`
- `testdata/golden/ols/basic_parity.json`

更新核心文件：

- `internal/site/service.go`
- `internal/cli/site.go`
- `internal/config/config.go`
- `internal/core/render/render.go`
- `internal/core/model/site.go`
- `internal/core/validate/site.go`
- `internal/rollback/history.go`
- `tests/integration/site/site_test.go`
- `tests/unit/cli/root_test.go`

## 7. Commands / UX Flows

当前新增能力：

- `llstack site:create example.com --backend ols --non-interactive --dry-run`
- `llstack site:create example.com --backend ols --non-interactive --skip-reload`
- `llstack site:list` 现在输出 backend 列

行为说明：

- `site:create` 默认 backend 仍是 `apache`
- 显式 `--backend ols` 时，输出 OLS 原生配置树而不是 Apache vhost
- delete / rollback 对 Apache 与 OLS 共用同一条 manifest/history 链路

## 8. Code

OLS compiler 当前覆盖：

- server name / aliases
- docroot
- index
- errorlog / accesslog
- extprocessor
- script handler
- rewrite 基础
- reverse proxy context 基础
- TLS 基础
- parity report 输出

当前降级：

- `HeaderRule` 只进 parity report，不生成 OLS 配置
- `AccessControlRule` 只进 parity report，不生成 OLS 配置

## 9. Tests

新增测试：

- OLS compiler golden test
- OLS asset path test
- OLS create integration test
- CLI `site:create --backend ols --dry-run --json` test

已验证：

- `go test ./...`
- `go build ./...`

## 10. Acceptance Criteria

本阶段验收结论：已满足 Phase 3 的最小目标。

- canonical model 已可编译为 OLS 原生配置
- listener map / extprocessor / script handler / TLS 资产已输出
- parity report 已输出
- OLS backend 可通过 `site:create --backend ols` 独立启用
- OLS golden tests 与目录生成测试已存在

## 11. Risks / Tradeoffs

- OLS 当前是 compile-first；真实服务验证仍待 Docker / 裸机补齐
- parity report 目前按 feature 维度输出，不是指令级 diff
- header/access control 仍是显式降级项
- extprocessor `path` 目前是运行时默认值，Phase 5 会替换为 PHP 子系统统一 adapter

## 12. Next Phase

Phase 4 将实现：

- LSWS backend
- enterprise-only flags
- trial / licensed 模式识别
- LSWS capability flags
- LSWS golden tests
