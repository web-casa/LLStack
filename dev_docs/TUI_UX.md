# LLStack TUI UX

## Design Principles

- 终端里的产品，而不是 shell 数字菜单
- 以任务和状态为中心，而不是以命令记忆为中心
- 所有写操作先预览 plan / diff
- 错误必须有恢复建议
- 快捷键统一、页面切换成本低

## Information Architecture

主导航：

1. Dashboard（含硬件 + 调优摘要）
2. Install（含 scenario 选择器）
3. Sites
4. Database Setup
5. Services
6. PHP
7. Logs
8. Doctor
9. History
10. SSL（证书状态 + 到期预警）
11. Cron（任务管理）
12. Security（fail2ban + 防火墙 + IP 封禁）

## Key Screens

### Dashboard

- 当前 backend
- OS / arch / hostname
- Web / PHP / DB / Cache 服务状态
- 最近操作
- 风险提示

### Install Wizard

- backend 选择
- PHP versions 选择
- DB provider + TLS policy
- cache 组件
- 初始站点
- 计划预览
- 当前实现支持字段切换、实时 plan preview 和 apply 确认

### Database Setup

- DB provider 选择
- TLS policy 选择
- admin user / password 输入
- 初始 database / app user / password 输入
- install / init / create database / create user 的组合 plan preview
- 当前实现支持字段选择、最小文本编辑、dry-run plan preview 和 apply 确认

### Sites List

- 域名
- backend
- PHP 版本
- TLS 状态
- 状态摘要
- 支持 `j/k` 选择站点
- 当前实现：Sites 页支持 `c` 打开内嵌 create wizard，`n` 确认创建
- 当前实现：Sites 页支持 `m` 打开内嵌 edit wizard，`n` 确认更新

### Site Detail

- canonical site summary
- rendered backend summary
- PHP runtime binding
- TLS / rewrite / logs / actions
- diff preview / reload / rollback
- 当前实现：Sites 页右侧/下方显示选中站点 detail + drift preview 摘要
- 当前实现：Sites 页支持 `s` 触发 start/stop、`r` 触发 reload，并要求二次确认
- 当前实现：Sites 页支持 `x` 触发 restart，并要求二次确认
- 当前实现：Sites 页支持 `g` 循环 access/error logs，`[` / `]` 调整日志 tail 行数，`ctrl+r` 刷新当前日志面板，`t` 查看 TLS dry-run plan
- 当前实现：Sites 页支持 `a` 对当前 TLS plan 执行 apply，并要求二次确认
- 当前实现：Sites 页支持 `v` 选择 PHP target version，`u` 执行 PHP switch
- 当前实现：create wizard 支持 `server_name` / `backend` / `profile` / `php_version` / `upstream`
- 当前实现：edit wizard 支持 `docroot` / `aliases` / `index_files` / `upstream`

### Doctor / Repair

- preflight checks
- 失败项
- 风险级别
- status filter
- 修复建议
- repair plan preview
- 当前实现支持 `j/k` 选择 check，查看 detail
- 当前实现支持 `f` 循环 `all / warn / pass`
- 当前实现支持 `p` 打开 repair plan preview
- 当前实现支持 `a` 发起 repair apply，并要求二次确认
- 当前实现的 repair preview 会显示 operation details，并沿用 detail 脱敏规则
- 当前实现会把 warning 分成 `auto-repair` 和 `manual-only` 两组，减少用户误判 repair 覆盖范围

### History / Rollback

- 最近 rollback history
- latest pending entry 标记
- selected entry detail
- history filter
- rollback plan preview
- rollback apply confirmation
- 当前实现支持 `j/k` 选择历史记录
- 当前实现支持 `f` 循环 `all / pending / rolled-back`
- 当前实现支持 `p` 查看 latest pending rollback plan
- 当前实现支持 `a` 发起 rollback apply，并要求二次确认
- 当前实现只允许对 latest pending entry 执行 rollback，不支持任意挑选旧记录直接回滚

## Layout Pattern

- 顶部：页面标题、上下文路径、连接状态
- 左侧：主导航
- 主区：表格 / 向导 / 详情
- 底部：快捷键提示、当前任务状态
- 右侧可选：详情 / warnings / parity report

## Keyboard Conventions

- `q`: 返回 / 退出
- `tab` / `shift+tab`: 区块切换
- `j` / `k`: 上下移动
- `enter`: 进入 / 确认
- `e`: 编辑
- `p`: 查看 plan
- `a`: apply 当前屏幕的 plan（需二次确认）
- `d`: 查看 diff
- `s`: start / stop（需二次确认）
- `x`: restart backend（需二次确认）
- `g`: 查看 access/error logs
- `[`, `]`: 调整 logs tail 行数
- `t`: 查看 TLS plan
- `ctrl+r`: 刷新当前 logs 视图
- `a`: apply 当前 TLS plan（需二次确认）
- `v`: 切换 PHP target version
- `u`: apply 当前 PHP target（需二次确认）
- `c`: 打开/关闭 site:create wizard
- `m`: 打开/关闭 site:update wizard
- `n`: apply 当前 site:create plan（需二次确认）
- `r`: reload / restart / repair（需二次确认）
- `ctrl+r`: 刷新状态
- `?`: 帮助

## Error & Recovery UX

- 错误消息必须包含：失败动作、对象、原因、建议下一步
- 可恢复错误优先提供 `Retry`、`Open logs`、`View plan`、`Rollback`
- 能自动降级时，必须显示 capability warning，而不是静默处理

## Phase 1 Deliverables

- Dashboard 壳
- Install Wizard 占位
- Services 页
- Sites / PHP / Logs 占位页
- 底部快捷键与状态栏

## Current Phase 7 Status

- Dashboard / Sites / PHP / Services / Logs 已读取真实 manifest
- Sites 页已支持选中站点和 diff preview 摘要
- Sites 页已支持 start/stop/reload 的确认与反馈
- Sites 页已支持 restart
- Sites 页已支持 logs panel 与 TLS plan preview
- Sites 页已支持 logs panel 的 line count 调整与 refresh feedback
- Sites 页已支持 TLS plan apply
- Sites 页已支持 PHP switch preview 与 apply
- Sites 页已支持内嵌 site:create wizard
- Sites 页已支持内嵌 site:update wizard
- Sites 页已支持在 edit wizard 中编辑 `index_files` / `upstream`
- Install 页已支持参数切换和 plan preview
- Install 页已支持 apply
- Database Setup 页已支持 provider/TLS 切换、文本输入、plan preview 和 apply
- 当前所有主要 plan preview 已显示 operation details，并对 `sql` / `password` / `secret` 默认脱敏
- 当前已新增独立 Doctor 页，显示 report、selected check detail 和 repair preview/apply
- 当前已新增 Doctor filter，可按 warn / pass 切换
- 当前已新增 Doctor repair coverage summary，可快速区分自动修复项和手动处理项
- 当前已新增独立 History 页，显示 rollback history、selected entry detail 和 rollback preview/apply
- 当前已新增 History filter，可按 pending / rolled-back 切换
- 完整 install/site/db wizard 仍未完成
