# OLS .htaccess 兼容性优化设计

## 1. OLS .htaccess 的已知问题

| 问题 | Apache 行为 | OLS 行为 | 影响 |
|------|-------------|----------|------|
| **PHP 指令** | `.htaccess` 中 `php_value`/`php_flag` 有效（mod_php） | **报 500 错误** | WordPress/Laravel 插件常写 php_value，用户不知道为什么报错 |
| **运行时读取** | 每次请求检查 .htaccess 变更 | **仅启动/reload 时读取**，运行时修改无效 | 用户改了 .htaccess 不生效，不知道要 reload |
| **RewriteBase** | 支持 | **不支持** | 子目录安装的 WordPress/Laravel 可能路由失败 |
| **[L] 标志** | 仅终止当前循环，可多次迭代 | **完全终止**（等同 Apache 的 [END]） | 复杂 rewrite 规则可能不按预期工作 |
| **IfModule 条件块** | 完整支持 | **仅支持 `<IfModule LiteSpeed>`** | 某些 .htaccess 的条件逻辑失效 |
| **FilesMatch/Location** | rewrite 在这些指令内有效 | **不触发 rewrite** | 部分安全配置失效 |

## 2. DirectAdmin 的兼容方案（参考）

DirectAdmin 的 OLS 集成用了几个策略：

1. **自动配置转换**：`CB2 converts all existing Apache configurations to OLS native configurations`
2. **.htaccess 变更监控**：cron 定时扫描 .htaccess 变更 → 自动 reload OLS
3. **PHP 配置分离**：引导用户使用 `.user.ini` 替代 `.htaccess` 中的 php_value
4. **模板系统**：per-site vhost 模板自动设置 `autoLoadHtaccess 1`、`phpIniOverride`、`open_basedir`

## 3. LLStack 的 OLS .htaccess 兼容性优化方案

### 3.1 自动 .htaccess → OLS 原生编译

**核心思路**：LLStack 的 canonical model compiler 已经为 OLS 生成原生配置。在此基础上，扫描站点目录的 `.htaccess`，将可翻译的指令编译到 `vhconf.conf` 中。

```
site:create 或 site:sync 时：
  1. 扫描 /data/www/<site>/.htaccess
  2. 提取可翻译的指令：
     - RewriteRule / RewriteCond → 写入 vhconf.conf rewrite 块
     - php_value / php_flag → 转为 phpIniOverride 或 .user.ini
     - ErrorDocument → 写入 vhconf.conf
  3. 不可翻译的指令 → 输出 warning + 建议手动处理
  4. 生成翻译报告（类似 parity report）
```

新增 CLI 命令：

```bash
# 扫描 .htaccess 并显示兼容性报告
llstack site:htaccess-check <site> --json

# 自动翻译 .htaccess 到 OLS 原生配置
llstack site:htaccess-compile <site> --json

# 输出示例：
{
  "site": "wp.example.com",
  "htaccess_path": "/data/www/wp.example.com/.htaccess",
  "translated": [
    {"directive": "RewriteRule", "status": "compiled", "target": "vhconf.conf"},
    {"directive": "php_value memory_limit", "status": "converted", "target": ".user.ini"}
  ],
  "warnings": [
    {"directive": "RewriteBase /blog", "status": "unsupported", "suggestion": "use OLS Context instead"}
  ]
}
```

### 3.2 php_value/php_flag 自动转换

当 OLS 站点的 `.htaccess` 包含 PHP 指令时，LLStack 自动：

1. **提取** php_value/php_flag 指令
2. **写入** `.user.ini`（用户可覆盖的参数）或 `phpIniOverride`（admin 级参数）
3. **注释掉** .htaccess 中的原始指令（加 `# Converted by LLStack: see .user.ini`）
4. **输出提示**：告知用户 OLS 不支持 .htaccess 中的 PHP 指令，已自动转换

```bash
# 自动转换
llstack site:htaccess-compile wp.example.com --json

# .htaccess 中：
# 转换前：
php_value memory_limit 512M
php_value upload_max_filesize 128M
php_flag display_errors Off

# 转换后（.htaccess 被修改）：
# Converted by LLStack to .user.ini (OLS does not support php_value in .htaccess)
# php_value memory_limit 512M
# php_value upload_max_filesize 128M
# php_flag display_errors Off

# 同时生成 .user.ini：
memory_limit = 512M
upload_max_filesize = 128M
display_errors = Off
```

### 3.3 .htaccess 变更监控 + 自动 reload

OLS 不会运行时重新读取 .htaccess。LLStack 提供两种方案：

**方案 A：systemd timer 定时扫描**

```bash
# 启用监控
llstack site:htaccess-watch enable --interval 60

# 实现：
# 1. 创建 systemd timer（每 60 秒）
# 2. 检查所有 OLS 站点的 .htaccess mtime
# 3. 有变更时自动 lswsctrl reload
# 4. 记录日志到 /var/log/llstack/htaccess-watch.log
```

**方案 B：inotifywait 实时监控（如果安装了 inotify-tools）**

```bash
# 实时监控模式
llstack site:htaccess-watch enable --mode realtime

# 实现：使用 inotifywait 监控 .htaccess 文件变更
# 比 cron 更及时，但需要额外依赖
```

**建议默认用方案 A**（systemd timer），如果用户安装了 `inotify-tools` 可选择方案 B。

### 3.4 RewriteBase 兼容

OLS 不支持 `RewriteBase`。LLStack 的处理：

1. **site:htaccess-check** 检测到 `RewriteBase` 时输出明确建议
2. **site:htaccess-compile** 自动将 `RewriteBase /path` 转为 OLS Context 配置

```
# .htaccess 中：
RewriteBase /blog

# LLStack 自动生成 vhconf.conf Context：
context /blog {
  location            /data/www/wp.example.com/blog
  allowBrowse         1
  rewrite {
    enable            1
    inherit           1
    # 原始 rewrite rules 放这里
  }
}
```

### 3.5 [L] 标志兼容提示

检测到可能受 `[L]` 行为差异影响的规则时，输出 warning：

```
warning: .htaccess line 15: RewriteRule with [L] flag
  OLS treats [L] as [END] (terminates all rewrite processing).
  Apache allows [L] to loop. If your rules depend on looping,
  consider reordering rules or using [L,E=END:1] workaround.
```

### 3.6 site:create 时自动配置

当用户创建 OLS 站点时，LLStack 自动：

```
1. vhconf.conf 已包含 autoLoadHtaccess 1（当前已有）
2. 如果 profile 是 wordpress/laravel，预置已知的 rewrite 规则到 vhconf.conf（当前已有）
3. 新增：生成 .user.ini 骨架文件（包含 profile 推荐的 PHP 参数）
4. 新增：如果站点目录已有 .htaccess，自动运行 htaccess-check 输出兼容性报告
```

### 3.7 TUI 集成

Doctor 页面新增 OLS .htaccess 兼容性检查：

```
doctor check: ols_htaccess_compat
  - 扫描所有 OLS 站点的 .htaccess
  - 检测 php_value/php_flag（应迁移到 .user.ini）
  - 检测 RewriteBase（不支持）
  - 检测 FilesMatch 内的 rewrite（不触发）
  - status: pass（无问题）/ warn（有可翻译项）
  - repairable: true（可自动转换）
```

## 4. 支持的 .htaccess 指令翻译矩阵

| .htaccess 指令 | OLS 原生支持 | LLStack 自动转换 | 目标 |
|---------------|-------------|-----------------|------|
| `RewriteRule` / `RewriteCond` | ✓（autoLoadHtaccess） | 可选编译到 vhconf | vhconf rewrite 块 |
| `php_value` / `php_flag` | ✗（报 500） | **自动转换** | `.user.ini` |
| `php_admin_value` / `php_admin_flag` | ✗ | **自动转换** | `phpIniOverride` |
| `ErrorDocument` | ✓ | 直通 | .htaccess |
| `RewriteBase` | ✗ | **自动转换** | vhconf Context |
| `Header` / `RequestHeader` | ✓（部分） | 直通 | .htaccess |
| `ExpiresActive` / `ExpiresByType` | ✓ | 直通 | .htaccess |
| `SetEnvIf` | ✓ | 直通 | .htaccess |
| `Options -Indexes` | ✓ | 直通 | .htaccess |
| `Deny from` / `Allow from` | ✓ | 直通 | .htaccess |
| `<FilesMatch>` + rewrite | ✗ | **输出 warning** | 需手动迁移到 vhconf |
| `<IfModule mod_xxx>` | ✗（除 LiteSpeed） | **输出 warning** | 建议改为 `<IfModule LiteSpeed>` |

## 5. 需要确认

### Q1：htaccess-compile 是否修改用户的 .htaccess？

- **选项 A**：直接修改（注释掉不兼容指令）+ 生成 .user.ini
- **选项 B**：不修改原文件，只在 vhconf/phpIniOverride 中覆盖
- **选项 C**：默认不修改，加 `--apply` 才修改

建议选 C（最安全）。

### Q2：htaccess-watch 是否默认启用？

建议：OLS 站点默认不启用。用户通过 `llstack site:htaccess-watch enable` 手动开启。创建 OLS 站点时输出提示：

```
hint: OLS reads .htaccess only at startup. Run `llstack site:htaccess-watch enable`
      to auto-reload when .htaccess changes, or manually `llstack site:reload` after edits.
```

### Q3：htaccess-check 是否纳入 doctor？

建议纳入。`ols_htaccess_compat` 作为新的 doctor check，发现问题时 warn + repairable。
