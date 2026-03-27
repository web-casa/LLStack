# Phase 7: 站点生命周期体验

## 1. Goal

- 让 install 不再只是 plan 占位
- 为站点创建提供 deploy profile
- 增加 site lifecycle 常用入口：show / reload / ssl / logs
- 提升 TUI 的状态可见性，减少纯占位页面

## 2. Scope

这轮完成：

- 新增 `internal/install` 统一编排服务
- `llstack install` 接入 PHP / DB / cache / site manager
- 新增 deploy profile：`generic` / `wordpress` / `laravel` / `static` / `reverse-proxy`
- `site:create` 支持 `--profile` 与 `--upstream`
- 新增 `site:show` / `site:reload` / `site:ssl` / `site:logs`
- `site:ssl` 支持 custom cert 与 Let's Encrypt plan/apply 路径
- TUI Dashboard / Sites / PHP / Services / Logs 读取真实 manifest，而不是固定占位

本轮追加完成：

- profile 脚手架文件纳入受管 asset
- `site:diff` 漂移预览
- `site:create --profile static|wordpress|laravel|reverse-proxy` 会输出对应 starter files
- `site:start` / `site:stop` 启停入口
- `site:restart` 入口
- `site:update` 入口
- TUI Dashboard / Sites 增加站点 state 与缺失 asset 可见性
- TUI Sites 页支持选择态与单站点 diff preview
- TUI Install 页支持字段切换与真实 plan preview
- TUI Database Setup 页支持 provider/TLS 切换、文本输入与 DB lifecycle plan preview
- TUI Database Setup 页支持 preview 后直接确认 apply
- TUI Sites 页支持 start/stop/reload 的二次确认与执行反馈
- TUI Sites 页支持 restart 的二次确认与执行反馈
- TUI Sites 页支持 per-site logs panel 与 TLS dry-run plan preview
- TUI Sites 页支持 per-site logs panel 的行数调节与显式刷新反馈
- TUI Sites 页支持 per-site TLS preview 后直接确认 apply
- Let’s Encrypt 增加 `certbot` 发行版路径探测、plan warning 和 Docker/集成测试骨架
- TUI Sites 页支持 per-site PHP version target / preview / apply
- TUI Sites 页新增内嵌 site:create 向导，支持 preview / confirm / apply
- TUI Install 页支持 preview 后直接确认 apply
- TUI Sites 页新增内嵌 site:update 向导，支持 `docroot` / `aliases` / `index_files` / `upstream` 的 preview / confirm / apply
- WordPress / Laravel profile scaffold 扩展为更完整的 starter file tree

这轮没有完成：

- TUI 仍不是完整表单式 wizard
- 未接入真正的 Let's Encrypt 功能测试
- WordPress / Laravel 仍不生成真实应用代码树

## 3. Decisions / ADR

新增 ADR：

- `ADR-0014-install-orchestration-and-site-profiles.md`
- `ADR-0015-certbot-detection-and-scaffold-scope.md`

## 4. Architecture

新增 install 编排链路：

`CLI install -> install.Service -> php/db/cache/site manager -> aggregate plan/apply`

新增 site profile 链路：

`site:create input -> ApplyProfile -> canonical Site -> backend renderer/compiler`

新增 site lifecycle 入口：

- `Show`
- `Reload`
- `UpdateTLS`
- `ReadLogs`

## 5. Data Models

Phase 7 新增/扩展：

- `model.Site.Profile`
- `model.DeployProfile`
- `install.Options`
- `site.UpdateTLSOptions`
- `site.LogReadOptions`

## 6. File Tree Changes

新增核心文件：

- `internal/install/service.go`
- `internal/site/profiles.go`
- `internal/tui/views/state.go`
- `tests/unit/site/profile_test.go`
- `tests/integration/install/install_test.go`
- `tests/integration/site/phase7_test.go`

更新核心文件：

- `internal/config/config.go`
- `internal/cli/install.go`
- `internal/cli/site.go`
- `internal/cli/root.go`
- `internal/site/service.go`
- `internal/site/scaffold.go`
- `internal/site/diff.go`
- `internal/core/model/site.go`
- `internal/ssl/certbot.go`
- `internal/tui/model.go`
- `internal/tui/views/*.go`

## 7. Commands / UX Flows

新增或增强命令：

- `llstack install --backend apache --php_version 8.3 --db mariadb --with_memcached --site example.com`
- `llstack site:create example.com --profile wordpress`
- `llstack site:create proxy.example.com --profile reverse-proxy --upstream http://127.0.0.1:8080`
- `llstack site:show example.com`
- `llstack site:start example.com`
- `llstack site:stop example.com`
- `llstack site:reload example.com`
- `llstack site:ssl example.com --letsencrypt --email admin@example.com`
- `llstack site:logs example.com --kind error --lines 50`
- `llstack site:diff example.com`

## 8. Code

当前新增能力：

- install aggregate apply
- site deploy profile defaults
- site scaffolded starter files
- managed asset drift preview
- site enabled/disabled state toggling
- Let's Encrypt command plan/apply shell-out via `certbot`
- certbot binary detection with plan warnings
- TUI 基于 manifest 的状态展示
- TUI Sites detail/diff browser
- TUI Sites action confirm + feedback
- TUI Sites restart action
- TUI Sites editable settings wizard
- TUI Sites extended editable settings: `index_files` / `upstream`
- TUI Sites logs panel + TLS plan preview
- TUI Sites richer logs workflow（line count + refresh）
- TUI Sites TLS apply confirm + feedback
- TUI Sites PHP switch preview + apply
- TUI Sites create wizard preview + apply
- TUI Install wizard state + plan preview
- TUI Install apply confirm + feedback
- TUI Database Setup wizard state + text editing + plan preview
- TUI Database Setup apply confirm + feedback

## 9. Tests

新增测试：

- site profile unit test
- install orchestrator integration test
- site TLS/log lifecycle integration test
- certbot detection / command integration test
- static profile scaffold + diff integration test
- wordpress / laravel scaffold integration test
- site start/stop integration test
- CLI install dry-run JSON test
- TUI site selection state test
- TUI install plan preview state test
- TUI database text input + plan preview state test
- TUI site stop confirmation/action state test
- TUI site restart state test
- TUI site logs panel + TLS preview state test
- TUI site logs line-count/refresh state test
- TUI site TLS apply state test
- TUI site PHP switch state test
- TUI site create wizard state test
- TUI install apply state test
- TUI database apply state test
- site restart integration test
- site settings update integration test
- reverse proxy upstream update integration test
- TUI site edit wizard state test

已验证：

- `go test ./...`
- `go build ./...`
- `go test ./tests/integration/site -run TestUpdateTLSLetsEncryptAndReadLogs -v`
- `go test ./tests/integration/install -run TestInstallServiceAggregatesSubsystemPlans -v`
- CLI 冒烟：`install --backend apache --php_version 8.3 --db mariadb --db_tls enabled --with_memcached --site example.com --dry-run`
- CLI 冒烟：`site:create proxy.example.com --profile reverse-proxy --upstream http://127.0.0.1:8080 --dry-run`

## 10. Acceptance Criteria

当前状态：Phase 7 completed。

本轮已经满足的子目标：

- install 不再是空壳
- site template 已落地
- 脚手架与 drift preview 已落地
- site start/stop 已落地
- TLS / logs / reload 入口已落地
- TUI 不再完全依赖占位文案，Install/Sites 已可交互查看 plan/diff
- Install Wizard 已在 TUI 中支持 preview/apply
- Database Setup 已在 TUI 中可交互编辑并查看 install/init/create plan
- Database Setup 已在 TUI 中支持 preview/apply
- Sites 已在 TUI 中可触发 start/stop/reload，并提供最小确认与结果反馈
- Sites 已在 TUI 中可触发 restart
- Sites 已在 TUI 中可查看 access/error logs，并预览 TLS 变更计划
- Sites 已在 TUI 中可对 access/error logs 调整 tail 行数，并提供显式 refresh 反馈
- Sites 已在 TUI 中可对预览中的 TLS 计划执行 apply
- Sites 已在 TUI 中可选择 PHP target version，并执行 preview/apply
- Sites 已在 TUI 中可直接创建新站点，并复用 `site:create` 的 preview/apply 链路
- Sites 已在 TUI 中可编辑 `docroot` / `aliases` / `index_files` / `upstream`
- Let’s Encrypt 已具备 `certbot` 探测、plan warning 和集成测试骨架
- WordPress / Laravel scaffold 已补到更完整的 starter file tree

仍待后续补齐：

- Let's Encrypt 真服务功能测试
- 更深的 site detail/edit workflow

## 11. Risks / Tradeoffs

- `certbot` 当前只做 binary 路径探测，不做 plugin/autorenew/scheduler 管理
- install aggregate apply 目前没有跨子系统事务级回滚
- TUI 这轮已支持 install/database preview/apply、site action confirm、site create/edit、logs/TLS/PHP preview/apply，但仍不是完整操作式工作流
- site start/stop 当前通过受管 config asset 启停，不是 per-site service 进程模型
- site reload/restart 当前作用于 backend service，而不是单站点隔离进程
- site settings 编辑当前仍只覆盖少量高频字段，不含 rewrite / headers / access rules / TLS detail

## 12. Next Phase

进入 Phase 8：

- doctor / repair / rollback
- preflight checks
- service sanity checks
- 诊断包与修复建议
