<div align="center">
  <h1>LLStack</h1>
  <p><strong>基于 LiteHttpd 的开源服务器管理面板</strong></p>
  <p>Apache 级别的 .htaccess 兼容性 · LiteSpeed 级别的性能</p>
  <p>
    <a href="https://llstack.com">文档站</a> ·
    <a href="https://llstack.com/guide/getting-started/">快速开始</a> ·
    <a href="https://llstack.com/reference/changelog/">更新日志</a> ·
    <a href="README.md">English</a>
  </p>
</div>

---

## 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/web-casa/LLStack/main/scripts/install.sh | sudo bash
```

安装完成后访问 `https://你的服务器IP:30333` 创建管理员账户。

**系统要求**：AlmaLinux / Rocky Linux / CentOS Stream 9 或 10 · 1GB 内存 · 5GB 磁盘

## 功能特性

### 核心服务

| 功能 | 说明 |
|------|------|
| **LiteHttpd 引擎** | OpenLiteSpeed + 80 种 .htaccess 指令，2.5 倍 Apache 性能 |
| **PHP 多版本** | PHP 7.4 ~ 8.4 (REMI 仓库)，php-litespeed SAPI（非 php-fpm） |
| **数据库管理** | MariaDB / PostgreSQL，导入导出克隆维护，Adminer SSO |
| **Redis 管理** | 用户隔离实例，对象缓存，ACL 权限管理 (6.0+) |
| **SSL 证书** | Let's Encrypt 自动签发续期，手动上传，强制 HTTPS |

### WordPress 工具箱（24 个 API 端点）

- 一键安装，插件/主题管理，SSO 免密登录
- **Wordfence CVE 漏洞扫描** — 33,000+ 漏洞数据，CVSS 评分
- Smart Update（克隆 → 测试 → 应用）+ 自动更新调度
- Redis 对象缓存集成
- 失败自动回滚

### 运维功能

| 功能 | 说明 |
|------|------|
| **Staging 环境** | 一键克隆，Push/Pull（文件/数据库/全部），域名自动替换 |
| **增量备份** | restic 去重 + AES-256 加密，1h-24h 定时调度，选择性恢复 |
| **CDN 集成** | Cloudflare 一键配置，缓存清除 |
| **系统监控** | CPU/内存/磁盘，Redis 趋势，cgroup 压力 |

### 安全防护

- **RBAC 多角色** — 4 角色：Owner / Admin / Developer / Viewer
- **ALTCHA** — PoW 工作量证明防暴力破解（无验证码）
- **2FA 双因素** — TOTP，支持 Google Authenticator / Authy
- JWT 鉴权，审计日志，per-site `disable_functions`
- Plan 资源配额（站点数、数据库数、磁盘配额）

### 更多功能

- 文件管理器 + WebSocket 在线终端 (xterm.js)
- Cron 定时任务，防火墙 (firewalld)，日志轮转
- Apache 一键迁移 (litehttpd-confconv)
- 应用商店（WordPress / Laravel / Typecho）
- 国际化：简体中文 / 繁體中文 / English

## 系统架构

```
浏览器 → LiteHttpd :30333 (HTTPS)
            ├── /api/*  → gunicorn :8001 (Flask + SQLite)
            ├── /ws/*   → gunicorn :8001 (WebSocket 终端)
            └── /*      → dist/ (React 19 + Radix UI)
```

## 文档

完整文档请访问 **[llstack.com](https://llstack.com)**

- [IT 管理员指南](https://llstack.com/guide/for-admins/) — 安装、配置、运维
- [站点用户指南](https://llstack.com/guide/for-users/) — 站点管理、WordPress、数据库
- [独立开发者指南](https://llstack.com/guide/for-developers/) — 全栈工作流、API 参考
- [LiteHttpd 引擎](https://llstack.com/guide/litehttpd/) — .htaccess 兼容性、性能基准
- [常见问题](https://llstack.com/reference/faq/)

## 性能对比

| 指标 | Apache 2.4 | Stock OLS | LiteHttpd |
|------|:----------:|:---------:|:---------:|
| 静态 RPS | 23,909 | 63,140 | **58,891** |
| PHP RPS (wp-login) | 274 | 258 | **292** |
| 内存占用 | 818 MB | 654 MB | **689 MB** |
| .htaccess 兼容 | 10/10 | 6/10 | **10/10** |

*测试环境：Linode 4C/8G，EL9，PHP 8.3，MariaDB 10.11*

## 竞品对比

| 功能 | LLStack | 宝塔 | CyberPanel | Plesk |
|------|:-------:|:----:|:----------:|:-----:|
| .htaccess 兼容 | 80+ 指令 | Nginx | 部分 | Apache |
| WordPress 工具箱 | 完整 | 基础 | 基础 | 完整 |
| CVE 漏洞扫描 | Wordfence | 无 | 无 | 有 |
| Staging 环境 | 有 | 无 | 无 | 有 |
| 增量备份 | restic | tar.gz | tar.gz | 有 |
| RBAC 多角色 | 4 角色 | 无 | 无 | 有 |
| 开源 | 是 | 部分 | 是 | 否 |
| 价格 | 免费 | 免费/付费 | 免费 | 付费 |

## 相关项目

- [LiteHttpd](https://litehttpd.com) — 高度兼容 Apache 的轻量化 Web Server
- [WebCasa](https://web.casa) — 更 AI Native 的开源服务器控制面板

## 许可证

GPLv3 — 详见 [LICENSE](LICENSE)
