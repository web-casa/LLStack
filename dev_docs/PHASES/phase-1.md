# Phase 1: 项目骨架与 CLI/TUI 基础

## 1. Goal

- 初始化 Go 项目
- 搭建 CLI 主入口
- 搭建 TUI 外壳
- 提供统一日志、配置、状态输出框架
- 提供基础命令：
  - `llstack version`
  - `llstack doctor`
  - `llstack status`
  - `llstack init`
  - `llstack install`
- 建立错误处理、任务执行器、变更计划输出骨架
- 建立测试和 Docker 目录骨架

## 2. Scope

做了什么：

- 创建 `go.mod`
- 创建 `cmd/llstack` 入口
- 创建 `internal/app`、`internal/cli`、`internal/tui`、`internal/core/plan`
- 实现基础命令和参数占位
- 实现统一 executor、logger、task runner 骨架
- 实现 plan JSON / 文本输出
- 实现 TUI 顶层导航壳与占位页
- 创建 `tests/`、`testdata/`、`docker/` 骨架

没有做什么：

- 未执行真实安装
- 未落地系统配置写入
- 未实现 backend renderer
- 未实现真实状态探测

## 3. Decisions / ADR

- 延续 Phase 0 的 ADR-0001 ~ ADR-0008
- Phase 1 未新增必须独立建档的架构决策

## 4. Architecture

当前已落地模块：

- `cmd/llstack`: 二进制入口
- `internal/app`: 应用组装与 task runner
- `internal/cli`: Cobra 命令树
- `internal/tui`: Bubble Tea 壳与页面切换
- `internal/core/plan`: 计划模型与 JSON 输出
- `internal/logging`: 统一 logger
- `internal/system`: 统一 executor
- `internal/config`: 默认路径和运行时配置

当前执行流：

`main -> app.New -> cli.Root -> command -> plan/status payload -> stdout or tui`

## 5. Data Models

Phase 1 已落地的最小模型：

- `config.RuntimeConfig`
- `config.Paths`
- `plan.Plan`
- `plan.Operation`
- `plan.Report`
- `app.Task`
- `app.TaskRunner`
- `system.Command`
- `system.Result`

## 6. File Tree Changes

新增核心文件：

- `go.mod`
- `cmd/llstack/main.go`
- `internal/app/app.go`
- `internal/app/tasks.go`
- `internal/cli/*.go`
- `internal/tui/*.go`
- `internal/tui/views/*.go`
- `internal/core/plan/*.go`
- `internal/system/executor.go`
- `internal/logging/logger.go`
- `internal/config/config.go`
- `tests/unit/...`
- `docker/images/...`
- `docker/compose/functional.yaml`

## 7. Commands / UX Flows

当前可运行命令：

- `llstack version`
- `llstack status`
- `llstack doctor`
- `llstack init --dry-run`
- `llstack init --json`
- `llstack install --backend apache --php_version 8.3 --db mariadb --dry-run`
- `llstack tui`

行为说明：

- `init` 与 `install` 当前只输出 plan，不执行 apply
- `status` 与 `doctor` 当前输出 Phase 1 占位状态
- `tui` 当前提供 Dashboard / Install / Services / Sites / PHP / Logs 导航壳

## 8. Code

Phase 1 代码状态：

- 可编译
- 可运行
- 可执行基础命令
- TUI 可启动并切换占位页

## 9. Tests

已实现：

- `tests/unit/cli/root_test.go`
- `tests/unit/core/plan/plan_test.go`
- `tests/unit/tui/model_test.go`

已验证命令：

- `go test ./...`
- `go build ./...`

Docker 与 golden 当前仅建目录骨架，未接入真实测试用例。

## 10. Acceptance Criteria

本阶段验收结论：已满足。

- Go 项目已初始化
- CLI 主入口已建立
- TUI 外壳已建立
- 统一日志、配置、executor、task runner 骨架已建立
- 基础命令已建立
- 测试骨架已建立
- Docker 骨架已建立

## 11. Risks / Tradeoffs

- 当前 `status` / `doctor` 还不依赖真实探测，属于产品壳而非功能完成
- `init` / `install` 仅有 plan，没有 apply / verify
- Phase 2 开始要防止 site / apache 实现绕过 plan 模型直接写文件

## 12. Next Phase

Phase 2 将实现：

- canonical site model 初版
- Apache backend renderer 初版
- `site:create`
- `site:list`
- `site:delete`
- plan / apply / rollback 基础链路
