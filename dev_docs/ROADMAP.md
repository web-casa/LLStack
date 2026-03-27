# LLStack Roadmap

## Product Scope

LLStack 是面向 EL9 / EL10（Rocky / AlmaLinux / RHEL 兼容）的 CLI + TUI Web Stack Installer 与 Site Lifecycle Manager。它不是 Web 面板，也不是一次性安装脚本；它是一个以可规划、可回滚、可诊断、可测试为核心的服务器生命周期工具。

## Milestones

| Phase | Title | Status | Outcome |
| --- | --- | --- | --- |
| 0 | 产品与架构蓝图 | completed | 固化产品边界、核心模型、测试策略、文档体系 |
| 1 | 项目骨架与 CLI/TUI 基础 | completed | Go 项目初始化，CLI/TUI 外壳，日志/任务/计划输出骨架 |
| 2 | Canonical Model + Apache 后端 | completed | 站点模型、Apache renderer、site/create/list/delete、基础 rollback |
| 3 | OpenLiteSpeed 配置编译器 | completed | OLS 原生配置编译、parity report、golden tests |
| 4 | LiteSpeed Enterprise 后端 | completed | LSWS renderer、feature flags、license/trial 探测 |
| 5 | PHP 子系统 | completed | Remi `php-litespeed` 多版本、per-site PHP runtime |
| 6 | 数据库与缓存子系统 | completed | DB/cache provider 初版、TLS profile、初始化链路 |
| 7 | 站点生命周期体验 | completed | install 编排、site profile、扩展 scaffold、diff、TLS/log/reload/restart、start/stop、TUI install/site/database preview/apply、site actions、site create/edit、logs/TLS/PHP preview 与 apply、certbot detection/test skeleton 已落地 |
| 8 | Doctor / Repair / Rollback | completed | doctor report、repair plan/apply、site reconcile、SELinux/firewalld/listening-port/php-socket/ownership/SELinux-context/DB-TLS/managed-service/provider-port/DB-live-probe preflight、diagnostics bundle、TUI doctor/history/rollback 已落地；更深的 sanity checks 已转入 backlog |
| 9 | 打磨与发布 | completed | 发布链路、SBOM/provenance baseline、config-driven install、Docker smoke matrix 与真实全矩阵 smoke 已完成 |

## Current Phase

- 当前状态：Release Maintenance / Backlog Grooming
- 下一阶段核心目标：
  - 维护 release / smoke 基线
  - 清理 post-Phase-9 backlog
  - 加固 CI / signing / provenance / e2e 基线

## Done

- 建立 Phase 0 文档基线
- 建立 ADR 目录
- 建立阶段文档目录
- 完成 Go 项目初始化
- 完成 CLI/TUI 基础外壳
- 完成 plan / logger / executor / task runner 骨架
- 完成测试与 Docker 目录骨架
- 完成 canonical site model 初版
- 完成 Apache backend renderer 初版
- 完成 `site:create` / `site:list` / `site:delete`
- 完成基础 rollback history 与 replay
- 完成 Apache renderer golden test 与 site integration test
- 完成 OLS compiler 与 parity report
- 完成 OLS golden tests 与目录生成测试
- 完成多 backend site manager 初版
- 完成 LSWS backend 初版
- 完成 LSWS license detection 与 capability flags
- 完成 LSWS golden test 与 integration test
- 完成 PHP runtime model 与 resolver
- 完成 PHP CLI 子系统
- 完成 per-site PHP switch
- 完成 DB provider 初版与 TLS profile 初版
- 完成 cache provider 初版
- 完成 DB/cache CLI 子系统
- 完成 install orchestrator 初版
- 完成 site deploy profile 初版
- 完成 profile 脚手架与 `site:diff`
- 完成 site start/stop state toggle
- 完成 `site:show` / `site:reload` / `site:ssl` / `site:logs`
- 完成 TUI Sites 选择态与 diff 预览
- 完成 TUI Sites start/stop/reload 确认流与反馈
- 完成 TUI Sites logs panel 与 TLS dry-run preview
- 完成 TUI Sites logs line-count 调节与 refresh feedback
- 完成 TUI Sites TLS preview -> apply 流
- 完成 TUI Sites PHP target -> preview -> apply 流
- 完成 TUI Sites 内嵌 site:create wizard
- 完成 CLI/TUI `site restart`
- 完成 CLI/TUI `site:update`
- 完成 `site:update` 的 `index_files` / `upstream` 编辑
- 完成 certbot binary detection 与 Let’s Encrypt test skeleton
- 完成更完整的 WordPress / Laravel scaffold
- 完成 TUI Install plan preview
- 完成 TUI Install apply
- 完成 TUI Database Setup 向导初版、dry-run plan preview 与 apply
- 完成 doctor report 初版
- 完成 repair plan/apply 初版
- 完成 site reconcile 初版
- 完成 SELinux / firewalld / listening-port preflight checks
- 完成 php-fpm socket / managed path permission / DB TLS preflight checks
- 完成 managed path ownership / SELinux context preflight checks
- 完成 managed service active probe
- 完成 managed DB/cache provider listener probe
- 完成 managed DB live probe 初版
- 完成 repair 对控制平面目录权限漂移的修复
- 完成 repair 对 inactive managed service 的 start 规则
- 完成 repair plan warning 聚合与 CLI 细化输出
- 完成 TUI plan preview detail/redaction 展示
- 完成 TUI Doctor/Repair 页面初版
- 完成 TUI History/Rollback 页面初版
- 完成 CLI rollback:list / rollback:show
- 完成 diagnostics bundle 导出
- 完成 diagnostics bundle 对 cache provider metadata 的采集
- 完成 diagnostics bundle 对 probe snapshot 和 managed log tail 的采集
- 完成 diagnostics bundle 对 host runtime / status / managed service snapshot 的采集
- 完成 diagnostics bundle 对原始 command snapshot 的采集
- 完成 diagnostics bundle 对额外 host probe snapshot 的采集
- 完成 diagnostics bundle 对 managed service journal 摘要的采集
- 完成 diagnostics bundle 对 provider config snapshot 的采集
- 完成 managed DB credential file 持久化与 auth probe
- 完成 DB provider metadata/TLS config 的 repair/reconcile
- 完成 managed SELinux suspicious label 的 restorecon repair
- 完成 firewalld 缺失必需 Web 端口的 repair
- 完成 TUI History filter（all/pending/rolled-back）
- 完成 TUI Doctor filter（all/warn/pass）
- 完成 TUI Doctor repair coverage summary（auto/manual）
- 修正 site rollback 不再删除共享控制平面目录
- 完成构建元数据基线
- 完成 release build/package 脚本
- 完成 install/upgrade 脚本
- 完成 install/upgrade 的远程 URL 下载模式
- 完成 release checksum verify 脚本与安装时的 sha256 校验
- 完成 release SPDX SBOM 产物
- 完成 release provenance baseline manifest
- 完成 release detached signature hook（OpenSSL）
- 完成 install / install-release 的 detached signature verification
- 完成 release metadata consistency verify
- 完成 GitHub Actions CI skeleton
- 完成 tag-driven GitHub release workflow
- 完成 release notes template/render
- 完成 tag/version guard
- 完成 post-release artifact verification summary
- 完成 remote release verification
- 完成 provider-neutral release pipeline 脚本
- 完成 install scenario profile 初版（wordpress / laravel / api / static / reverse-proxy defaults）
- 完成 managed cache live probe
- 完成 diagnostics bundle 的 site/php/db/cache 摘要快照扩展
- 完成 Docker smoke 的更深 site lifecycle 覆盖（diff/update/ssl dry-run/start-stop）
- 完成 release index 自动解析安装脚本
- 完成配置文件驱动安装（YAML / JSON）与 CLI 覆盖合并
- 完成 install config nested schema 与 legacy flat 兼容
- 完成 CLI help 分组、example 和 flag error usage hint 打磨
- 完成仓库根 README 与 release/operator 指南
- 完成 Makefile 发布入口
- 完成 host smoke 与 Docker smoke 骨架
- 完成 Docker functional 执行脚本与 opt-in 测试入口
- 完成 Docker functional service matrix 校验、artifact 输出与 passed-marker 断言
- 完成 Docker smoke summary report 生成
- 完成 Apache Docker smoke 的真实服务级断言
- 完成 OLS / LSWS Docker smoke 的真实受管产物断言
- 完成 EL9/EL10 × Apache/OLS/LSWS Docker smoke matrix
- 完成 release/package/install/upgrade/smoke 集成测试
- 完成 quickstart examples 基线

## In Progress

- release maintenance
- CI / release pipeline hardening
- provenance build context 已补全（git commit / Go version / build platform）
- OLS Docker runtime verification 已增加（configtest + HTTP 请求验证）
- provider-neutral publish 已实现（github + directory provider）
- doctor 新增 DB connection saturation + cache memory saturation checks
- TUI install wizard 增加 scenario 选择（wordpress/laravel/api/static/reverse-proxy）
- scenario profile 测试补全（WordPress/Laravel/API/Static）
- diagnostics bundle 增加 ps aux 进程快照和 TLS 证书到期快照
- doctor 新增 php_fpm_process_health check（检测 zombie FPM 服务）
- doctor 新增 php_config_drift check（检测 managed ini 漂移）
- TUI site create wizard 扩展 aliases / docroot 字段
- certbot 测试骨架增强（7 个新测试覆盖失败/缺失参数/webroot 场景）
- OLS 主配置自动管理（site:create/delete 自动注册/注销 virtualhost + listener map）
- 跨子系统 install rollback 错误上下文（失败时报告已完成步骤 + 清理指引）
- repair 支持 PHP config drift 自动修复（重写 managed ini snippet）
- TUI site edit wizard 扩展 php_version / tls 字段
- 签名策略默认强制化（--pubkey 提供时默认 require-signature，增加 --skip-signature）
- aarch64 Docker smoke 支持（LLSTACK_DOCKER_PLATFORM 环境变量）
- PHP uninstall + pool tuning（php:remove / TunePool API）
- DB uninstall + backup hooks（db:remove / db:backup API，mysqldump / pg_dumpall）
- TUI logs follow 模式（2s 自动刷新）+ grep 关键词过滤
- Per-site Linux 用户隔离（site:create 创建用户 + 权限 + Web 用户加组）
- SiteManifest 增加 SystemUser 字段
- Apache renderer 增加 per-site FPM pool 生成函数
- OLS compiler 增加 phpIniOverride（open_basedir）
- LSWS renderer 增加 IfModule LiteSpeed suEXEC 块
- PHP 版本扩展到 7.4-8.5（EOL warning）
- Valkey cache provider（与 Redis 互斥，API 兼容）
- 硬件检测（CPU/RAM）+ 调优参数计算引擎
- Per-site PHP 配置覆盖（Apache php_admin_value / OLS .user.ini / LSWS .user.ini）
- SSL 证书生命周期管理（status / renew / auto-renew systemd timer / 到期检测）
- Cron 任务管理（add/list/remove / wp-cron + laravel-scheduler 预设）
- 安全加固（fail2ban 默认 jail / IP 黑名单 / rate limiting 统一抽象 / firewalld 管理）
- OLS .htaccess 兼容性（check 诊断 / compile 转换 / doctor ols_htaccess_compat check）
- CLI 命令注册：19 个新命令（ssl:* / cron:* / security:* / firewall:* / tune / site:php-config / site:htaccess-*）
- TUI 新增 SSL / Cron / Security 三个页面
- Dashboard 集成硬件检测 + 调优摘要
- CLI 注册 php:remove / php:tune / db:remove / db:backup 命令
- 新子系统单元测试（cron 5 个 / ssl lifecycle 5 个 / ols htaccess 5 个 / ols configmanager 3 个）
- doctor 新增 ssl_certificate_expiry check（到期 14 天内 warn）
- repair 自动转换 OLS .htaccess 中的 php_value 到 .user.ini
- OLS/LSWS rate limiting perClientConnLimit 配置写入
- wp-cron 预设自动写入 DISABLE_WP_CRON 到 wp-config.php
- htaccess-watch systemd timer 单元生成函数
- TUI SSL/Cron/Security 交互操作（SSL renew-all / Cron add-hint / feedback 显示）
- Per-site FPM pool 完整接入（site:create 时在 ProfileRoot 存在时生成 pool config）
- 定时备份 + 保留策略（systemd timer + CleanupOldBackups 自动清理）
- site:batch-create 从 YAML/JSON 批量创建站点
- site:stats 访问日志分析（Top URL / Top IP / 状态码分布）
- install 编排自动安装 Apache（dnf install httpd + IncludeOptional 配置）
- install 编排自动 SELinux fcontext + restorecon
- PHP install 自动修改 FPM listen.acl_users 加入 apache
- README 更新为完整功能列表
- 交互式安装向导（无参数 `llstack install` 进入问答式安装）
- 安装后欢迎页面（PHP 页面 + x-prober + Adminer）
- `welcome:remove` 命令清理欢迎页面
- DB root 密码支持手动设置和自动生成
- fail2ban 默认 monitor-only 模式（不自动封禁）
- install 时自动检测服务器 IP 显示在结果中

## Not Started

- post-Phase-8：更深的 DB/service sanity check / diagnostics bundle 扩展
- SLSA 格式 provenance / signed envelope / transparency log integration
- 更深层的 Docker / e2e service matrix
- 更多 publish provider（GitLab / S3 API 等）

## Exit Criteria For Phase 7

- install aggregate apply 落地
- site profile / tls / logs / reload 入口落地
- Phase 7 tests 存在
- `go test ./...` 与 `go build ./...` 通过
- 一键部署应用（app:install wordpress/drupal/joomla/nextcloud/matomo/mediawiki/typecho/laravel）
- 站点备份/还原（site:backup + site:restore，文件+配置+可选DB dump → tar.gz）
- 站点级 PHP 版本切换（site:php-switch --version 8.4）
- SFTP 账号管理（sftp:create/list/remove，chroot 到站点目录，密码/密钥认证）
- Logrotate 自动配置（site:create 时自动生成 /etc/logrotate.d/ 规则）
- 服务自动重启守护（systemd Restart=on-failure + watchdog timer + webhook 告警）
- 数据库参数调优（db:tune 根据 RAM 自动生成 my.cnf / postgresql.conf 关键参数）
