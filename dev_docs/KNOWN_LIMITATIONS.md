# Known Limitations

## Current Phase

Phase 8 / Phase 9 已完成。当前进入 post-Phase-9 的 release maintenance / backlog grooming 阶段。

## Planned Early-Phase Limitations

- Phase 1 不执行真实安装，只提供骨架命令与计划输出占位
- Phase 1 TUI 仅提供导航与占位视图
- `status` 目前是 managed state summary，不是 full health dashboard
- Phase 3 OLS 现已包含 compile + 主配置自动管理（virtualhost + listener map 注册）；真实服务验证通过 Docker smoke 覆盖
- Phase 2 `site:list` 只读取 LLStack manifest，不扫描现有 Apache vhost
- Phase 2 `HeaderRule` / `AccessControlRule` 仅有最小输出，不代表完整 Apache 兼容层
- Phase 2 未在真实 Apache 服务环境执行 configtest/reload 验证
- Phase 3 `HeaderRule` / `AccessControlRule` 在 OLS 中仅进入 parity report，不生成原生配置
- Phase 3 未在真实 OLS 服务环境执行 configtest/reload 验证
- Phase 3 的 OLS parity 先覆盖常见站点能力，不承诺 100% Apache 指令等价
- Phase 4 LSWS 在 `license=unknown` 时采用保守 capability 判定
- Phase 4 未在真实 LSWS 服务环境执行 configtest/reload 验证
- LSWS 授权与 enterprise-only 功能需要后续真实环境验证
- Phase 5 未在真实 EL9/EL10 上执行 Remi repo + dnf 功能测试
- Phase 5 PHP uninstall 和 pool tuning 已实现（Uninstall / TunePool API），health probe 已由 doctor check 覆盖
- Phase 6 未在真实 EL9/EL10 上执行 DB/cache 安装功能测试
- Phase 6 的 MySQL / Percona / PostgreSQL 默认假定 vendor repo 已预配置
- Phase 6 PostgreSQL TLS 仅写 server snippet，不重写 `pg_hba.conf`
- Phase 6 DB uninstall 和 backup hooks 已实现（Uninstall / Backup API，mysqldump / pg_dumpall），health probe 已由 doctor check 覆盖
- Phase 6 cache 配置路径未做发行版级自动探测
- Phase 7 install 虽已可编排 apply，且失败时会报告已完成步骤和清理指引，但尚无自动逆向回滚
- Phase 7 TUI 已有 install/database 参数编辑、preview 和 apply，但仍不是完整表单式 wizard
- Phase 7 TUI Sites 已支持 start/stop/reload，但尚未提供动作级 plan preview
- Phase 7 TUI Sites 已支持 TLS apply，但仍只走默认 letsencrypt/custom 推断，不支持在页面内编辑 ACME 邮箱或证书路径
- Phase 7 TUI Sites 的 PHP switch 依赖已安装 runtime 列表；若 runtime manifest 不存在，则退回支持版本列表做预览
- Phase 7 TUI Sites 的 create wizard 目前覆盖 server_name / aliases / docroot / backend / profile / php_version / upstream，不支持自定义证书和高级 rewrite 输入
- Phase 7 TUI Sites 的 edit wizard 当前覆盖 `docroot` / `aliases` / `index_files` / `upstream` / `php_version` / `tls`
- Phase 7 TUI Sites 的 logs workflow 已支持 tail 行数调节、refresh feedback、follow 自动刷新和 grep 关键词过滤，但仍不支持时间范围过滤
- Phase 7 `site:reload` / `site:restart` 面向 backend service，不提供单站点级隔离进程控制
- Phase 7 Let’s Encrypt 已完成 `certbot` orchestration、binary detection 和测试骨架，但未做真实 ACME 功能测试
- Phase 7 脚手架已扩展为更完整 starter files，但仍不负责安装 WordPress / Laravel 应用本体
- Phase 7 `site:diff` 目前只覆盖 LLStack 受管 asset，不扫描全部手工文件
- Phase 7 `site:start` / `site:stop` 基于配置启停，不提供进程级 pause/resume
- Phase 8 `doctor` 已覆盖 SELinux / firewalld / listening ports / php-fpm sockets / managed path ownership / managed path permissions / managed SELinux context / DB TLS asset presence / managed service active probe / managed provider listener probe / managed DB live probe / managed DB auth probe / managed cache live probe / DB connection saturation / cache memory saturation，但带凭据 DB probe 仍依赖受管 credential file 存在
- Phase 8 `repair` 当前会修控制平面目录、recoverable DB/provider metadata/TLS config、canonical site asset 和明确 inactive/failed 的 managed service，但不处理 DB 数据层修复
- Phase 8 firewalld repair 只在 firewalld 正在运行且当前规则可成功枚举时启用，不会盲目改防火墙
- Phase 8 owner/group 自动修复仅在 root 场景下启用，非 root 运行会保留 warning 而不强制 `chown`
- Phase 8 SELinux `restorecon` 自动修复仅在 root 场景下启用，且只针对 LLStack 控制平面路径
- Phase 8 OLS / LSWS 只有在运行命令可稳定映射到 `systemctl` service 时才进入 active probe；默认仍可能显示 `unprobed`
- Phase 8 diagnostics bundle 当前会打包 LLStack 受管元数据、provider/cache metadata、host/status/service snapshot、raw command snapshot（含 ps aux 进程快照）、managed service journal 摘要、provider config snapshot、probe 快照、TLS 证书到期快照、日志摘要与基础主机信息，但仍不包含完整日志目录和站点源码树
- Phase 8 TUI Doctor 虽已区分 auto-repair / manual-only warning，但 repair 覆盖范围仍以当前 doctor check 的 `Repairable` 标记为准，不代表更深层业务恢复
- Phase 8 TUI History 页当前只允许对 latest pending entry 执行 rollback，不支持在历史列表中任意选旧记录直接回滚
- Phase 8 DB reconcile 只能恢复 provider manifest 衍生的 config/metadata，不能重建丢失的 secret 内容
- Docker 功能测试早期优先 Apache，OLS / LSWS 先做编译正确性和最小启动验证
- Phase 9 release scripts 已支持远程 URL 下载，但当前依赖 `curl` 或 `wget`
- Phase 9 当前提供 detached signing、sha256、SPDX SBOM 和含构建上下文的 provenance manifest，但仍不是 SLSA 格式的 trusted provenance chain
- Phase 9 release verify 目前校验的是元数据一致性与 detached signature，不是自签名 envelope 信任链
- Phase 9 release index 目前只按 `platform -> archive + sha256` 解析，不支持渠道、签名级别或多镜像优先级策略
- Phase 9 配置文件驱动安装已支持 nested schema，但当前仍同时保留 legacy flat 兼容层
- Phase 9 Docker functional baseline 目前是 smoke-first；仓库默认测试会做 compose config 与 service matrix 校验，真实容器 smoke 需要显式启用
- Phase 9 Docker functional 虽已具备 service matrix 校验、artifact 输出、passed-marker 断言，且 Apache 已有真实服务级断言，但仍不是完整服务功能矩阵
- LSWS 的 Docker smoke 仍是 compile/asset-first，不代表真实授权服务环境验证
- OLS Docker smoke 现已包含运行时验证（best-effort），但 vhost 注册到 OLS 主配置仍由 smoke fixture 手动完成
- Phase 9 已覆盖 EL9/EL10 × Apache/OLS/LSWS smoke matrix，OLS 现已具备运行时验证（configtest + HTTP），LSWS 仍为 asset-first（需要真实授权）
- Phase 9 GitHub Actions CI 与 tag-driven release workflow 已可执行，发布链路已通过 provider-neutral `publish.sh` 支持 `github` 和 `directory` provider；密钥生命周期治理尚未纳入
- post-release 校验已可对任意 provider 发布的产物做远程验证，不再仅限 GitHub Release
- 当前 remote verify 仍是“下载远端产物后复用本地 verify”模式，还不是透明日志或受信证明链校验
- install scenario profile 当前提供 defaults merge 和 TUI scenario 选择器，但不是完整 scenario graph / dependency solver

## Non-Goals For Early Releases

- Web GUI
- 多节点集群
- 全量自动化迁移工具
- 所有 Apache 指令无差别兼容
- 一次性覆盖所有 DB provider 的高级参数

## Downgrade Policy

- 能降级时必须输出 warning
- 不能安全降级时必须阻止 apply
- 所有降级必须可审计，可在 plan / report / history 中看到
- Phase 9 已正式结项；当前限制不再包括“未执行真实 Docker smoke”
- 当前发布链路已支持本地 detached signing，`provenance.json` 已包含构建上下文（git commit / Go 版本 / 构建平台），但仍通过 detached signature 保护，尚未升级为自签名 envelope 或 SLSA 格式
- 安装链路支持 detached signature verification，提供 --pubkey 时默认强制验签，可通过 --skip-signature 显式跳过
