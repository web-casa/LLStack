# Phase 0

## Goal

- 明确产品边界
- 固化总体架构与技术选型
- 设计 canonical model、backend adapter / renderer / compiler
- 设计 installer 参数模型、database provider / TLS model
- 设计测试分层与 Docker 策略
- 建立 `dev_docs/` 文档体系

## In Scope

- 文档
- ADR
- 目录结构草案
- 命令设计草案
- TUI 信息架构

## Out of Scope

- 真实安装逻辑
- 真实站点创建逻辑
- 真实 backend renderer 代码

## Decisions

- 语言：Go
- CLI：Cobra
- TUI：Bubble Tea + Bubbles + Lip Gloss
- 配置文件：YAML
- 模板：`text/template`
- 执行模型：统一 executor
- 写入模型：planner -> applier -> verifier -> rollback metadata
- 默认站点根目录：`/data/www/<site>`
- Apache backend 的 PHP 运行时：`php-fpm`
- LSWS 早期阶段接受 compile-first / detect-first，真实授权验证后置

## Deliverables

- `dev_docs/ROADMAP.md`
- `dev_docs/ARCHITECTURE.md`
- `dev_docs/TUI_UX.md`
- `dev_docs/TESTING.md`
- `dev_docs/COMPATIBILITY.md`
- `dev_docs/KNOWN_LIMITATIONS.md`
- `dev_docs/reference-teddysun-lamp.md`
- ADR 初版

## Repository Draft

```text
.
├── cmd/
│   └── llstack/
├── internal/
│   ├── app/
│   ├── cli/
│   ├── tui/
│   ├── core/
│   │   ├── model/
│   │   ├── validate/
│   │   ├── capability/
│   │   ├── plan/
│   │   ├── render/
│   │   ├── apply/
│   │   └── verify/
│   ├── backend/
│   │   ├── apache/
│   │   ├── ols/
│   │   └── lsws/
│   ├── php/
│   ├── db/
│   ├── cache/
│   ├── ssl/
│   ├── doctor/
│   ├── system/
│   ├── rollback/
│   ├── logging/
│   └── config/
├── pkg/
├── templates/
│   ├── apache/
│   ├── ols/
│   ├── lsws/
│   └── php/
├── tests/
│   ├── unit/
│   ├── integration/
│   └── golden/
├── testdata/
│   ├── fixtures/
│   └── golden/
├── docker/
│   ├── images/
│   ├── fixtures/
│   └── compose/
└── dev_docs/
```

## Phase 1 Execution Plan

1. 初始化 Go 模块与基础目录。
2. 搭建 `cmd/llstack` 入口与 Cobra 根命令。
3. 搭建 Bubble Tea TUI 外壳与 Dashboard/Install/Services/Sites/PHP/Logs 占位页。
4. 建立统一 executor、logger、status reporter、task runner 接口。
5. 建立 plan JSON 输出最小模型。
6. 实现 `version`、`doctor`、`status`、`init`、`install` 占位命令。
7. 新建测试骨架、golden 目录、Docker fixtures 占位。
8. 更新 `dev_docs/` 以反映真实代码状态。

## Phase 1 Files To Create

文档：

- `dev_docs/PHASES/phase-1.md`
- `dev_docs/ADR/ADR-0006-cli-and-tui-framework.md`（若无新决策可不建）

工程骨架：

- `go.mod`
- `cmd/llstack/main.go`
- `internal/app/app.go`
- `internal/cli/root.go`
- `internal/cli/version.go`
- `internal/cli/doctor.go`
- `internal/cli/status.go`
- `internal/cli/init.go`
- `internal/cli/install.go`
- `internal/tui/program.go`
- `internal/tui/model.go`
- `internal/tui/views/dashboard.go`
- `internal/tui/views/install.go`
- `internal/tui/views/services.go`
- `internal/tui/views/sites.go`
- `internal/tui/views/php.go`
- `internal/tui/views/logs.go`
- `internal/logging/logger.go`
- `internal/system/executor.go`
- `internal/core/plan/plan.go`
- `internal/core/plan/report.go`
- `internal/config/config.go`

测试骨架：

- `tests/unit/cli/root_test.go`
- `tests/unit/core/plan/plan_test.go`
- `tests/unit/tui/model_test.go`
- `tests/golden/.keep`
- `testdata/golden/.keep`
- `docker/images/el9-apache/Dockerfile`
- `docker/images/el9-ols/Dockerfile`
- `docker/images/el10-apache/Dockerfile`
- `docker/compose/functional.yaml`

## Acceptance

- Phase 1 可以直接按文档创建工程骨架
- 高风险决策已进入 ADR
- 命令树、目录结构、测试骨架足够具体

## Risks

- LSWS 授权环境与自动化测试可得性
- OLS 对 Apache 语义的映射边界
- DB TLS 初始化在不同 provider 上的差异

## Next Phase Input

- 创建 Go 模块
- 初始化 CLI/TUI 外壳
- 引入日志、状态、任务与 plan JSON 输出骨架
