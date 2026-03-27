# Phase 2: Canonical Config Model + Apache 后端

## 1. Goal

- 定义 canonical site model 初版
- 支持从内部模型渲染 Apache VirtualHost
- 支持 `site:create` / `site:list` / `site:delete`
- 建立 plan / apply / rollback 基础链路
- 为 Apache renderer 增加 golden tests

## 2. Scope

做了什么：

- 创建 canonical site model
- 创建 site validator
- 创建 Apache renderer 与 verifier
- 创建文件 applier 与 rollback history
- 实现 `site:create` / `site:list` / `site:delete` / `rollback`
- 实现 site manifest store
- 实现交互式与参数式 `site:create`
- 增加 Apache renderer golden test
- 增加 site create/delete/rollback integration test

没有做什么：

- 未实现 OLS / LSWS
- 未实现 site:update
- 未实现完整 header/access rule 映射
- 未在真实 EL9/EL10 + httpd 环境执行服务 reload 验证

## 3. Decisions / ADR

- 延续 Phase 0 / 1 ADR
- 新增 provisional 决策：Apache 托管 vhost 默认目录暂定 `/etc/httpd/conf.d/llstack/sites`

新增 ADR：

- `ADR-0009-apache-managed-vhost-path.md`

## 4. Architecture

当前 Phase 2 主链路：

`CLI -> site.Manager -> defaults -> validate -> apache.Renderer -> plan -> apply.FileApplier -> apache.Verifier -> rollback.History`

新增模块：

- `internal/core/model`
- `internal/core/validate`
- `internal/core/render`
- `internal/core/apply`
- `internal/core/verify`
- `internal/backend/apache`
- `internal/site`
- `internal/rollback`

状态存储：

- canonical site manifest: `/etc/llstack/sites/<site>.json`
- Apache managed vhost: `/etc/httpd/conf.d/llstack/sites/<site>.conf`
- site logs 默认目录：`/var/log/llstack/sites/`
- rollback history: `/var/lib/llstack/history/`
- backups: `/var/lib/llstack/backups/`

## 5. Data Models

本阶段已落地：

- `model.Site`
- `model.DomainBinding`
- `model.TLSConfig`
- `model.PHPRuntimeBinding`
- `model.RewriteRule`
- `model.HeaderRule`
- `model.AccessControlRule`
- `model.ReverseProxyRule`
- `model.LogConfig`
- `model.SiteManifest`
- `apply.Change`
- `rollback.Entry`

## 6. File Tree Changes

新增核心文件：

- `internal/core/model/site.go`
- `internal/core/validate/site.go`
- `internal/core/render/render.go`
- `internal/core/apply/files.go`
- `internal/core/verify/verify.go`
- `internal/backend/apache/renderer.go`
- `internal/backend/apache/verifier.go`
- `internal/site/service.go`
- `internal/rollback/history.go`
- `internal/cli/site.go`
- `tests/unit/backend/apache/renderer_test.go`
- `tests/unit/core/validate/site_test.go`
- `tests/integration/site/site_test.go`
- `testdata/golden/apache/basic_vhost.conf`

## 7. Commands / UX Flows

当前新增命令：

- `llstack site:create`
- `llstack site:list`
- `llstack site:delete`
- `llstack rollback --last`

示例：

- `llstack site:create example.com --non-interactive --dry-run`
- `llstack site:create` 后进入最小交互式输入
- `llstack site:list`
- `llstack site:delete example.com --dry-run`
- `llstack rollback --dry-run`

行为说明：

- `site:create` 默认 apply；加 `--dry-run` / `--plan-only` 则只输出计划
- `site:create` 支持参数与交互输入
- `site:delete` 默认只删除托管配置与 manifest，不 purge docroot，除非显式 `--purge-root`
- `rollback` 当前回滚最近一次未回滚的托管变更

## 8. Code

Phase 2 已实现的能力：

- canonical site 输入模型
- Apache vhost 渲染
- manifest 写入
- vhost 写入
- Apache configtest / reload 命令接线
- rollback metadata 记录与回放

Apache renderer 当前覆盖：

- `ServerName` / `ServerAlias`
- `DocumentRoot`
- `DirectoryIndex`
- access/error log
- TLS 基础
- rewrite 基础
- reverse proxy 基础
- `php-fpm` handler 关联

## 9. Tests

已新增：

- validator unit test
- Apache renderer golden test
- CLI `site:create --dry-run --json` test
- site create/delete/rollback integration test

已验证：

- `go test ./...`
- `go build ./...`

本地环境限制：

- 未在真实 Apache 服务存在的机器上执行 `apachectl configtest` 和 `systemctl reload httpd`
- integration test 使用 fake executor 验证命令编排与文件链路

## 10. Acceptance Criteria

本阶段验收结论：已基本满足。

- canonical site model 已落地
- Apache renderer 已落地
- `site:create` / `site:list` / `site:delete` 已落地
- plan / apply / rollback 基础链路已落地
- golden test 与 integration test 已存在

未完成的验收补项：

- 真实 EL9/EL10 + httpd 环境服务级验证仍待后续 Docker/裸机阶段补完

## 11. Risks / Tradeoffs

- Apache managed vhost 路径当前是 provisional 默认值
- `HeaderRule` / `AccessControlRule` 当前只保留模型与最小输出，不是完整兼容层
- rollback 当前优先覆盖 LLStack 自己生成的文件与目录，不处理外部手工改动合并
- `site:list` 当前依赖 manifest store，不做 Apache 反向扫描

## 12. Next Phase

Phase 3 将实现：

- OLS compiler
- OLS 原生目录/文件生成
- listener map / extprocessor / script handler 生成
- parity report
- OLS golden tests
- OLS 目录生成测试
