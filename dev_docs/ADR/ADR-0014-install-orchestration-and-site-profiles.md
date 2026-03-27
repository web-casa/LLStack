# ADR-0014: Install Orchestration And Site Profiles

## Status

accepted

## Context

到 Phase 6 为止，LLStack 已有独立的 PHP、DB、cache、site manager，但 `llstack install` 仍然只是 plan 占位，site lifecycle 也缺少模板化输入与 TLS/log/reload 入口。

Phase 7 需要在不破坏既有模块边界的前提下，把这些能力编排成更接近正式产品的使用路径：

- install 不应再是单纯的 CLI 拼接输出
- CLI / 后续 TUI 必须复用相同的 orchestration
- site:create 需要 profile/template，而不是要求用户手工拼规则

## Decision

新增：

- `internal/install`
- `internal/site/profiles.go`

设计约束：

- `install.Service` 只编排现有 manager，不重复实现 provider 逻辑
- site profile 只负责 canonical site defaults，不直接写 backend-specific 配置
- reload / tls / logs 仍通过 `site.Manager` 暴露，保持 site lifecycle 的统一入口

内建 deploy profile：

- `generic`
- `wordpress`
- `laravel`
- `static`
- `reverse-proxy`

## Consequences

优点：

- `llstack install` 开始具备真实 apply 能力
- 站点模板与 canonical model 绑定，后端 renderer 仍然保持纯映射职责
- Phase 7 后续的 TUI wizard 可直接复用 install/site manager

代价：

- install 流程目前仍按顺序线性编排，不含事务级整体回滚
- Let’s Encrypt 目前通过 `certbot` executor 调用，真实环境验证仍需后续功能测试
- TUI 这轮先提升为“状态感知界面”，还不是完整表单式 wizard
