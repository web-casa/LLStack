# Per-Site 用户隔离与 PHP 进程模型设计

## 1. OLS vs LSWS vs Apache 核心区别

### Apache 兼容性对比

| 特性 | Apache httpd | LSWS Enterprise | OpenLiteSpeed |
|------|-------------|-----------------|---------------|
| .htaccess 完整支持 | 原生 | **自动读取**（drop-in 替换） | **不自动读取**，需在 vhost 中 `autoLoadHtaccess 1` |
| PHP `php_value`/`php_flag` in .htaccess | 仅 mod_php | **支持**（不报 500） | **不支持**（需用 `.user.ini`） |
| `.user.ini` | 支持（php-fpm 模式） | 支持 | **支持**（推荐方式） |
| mod_rewrite 规则 | 原生 | **几乎兼容**（`[L]` 等于 `[END]`） | **语法兼容**，需手动迁移到 vhost context |
| Apache module | 原生 | **不支持**（常用功能内建） | 不支持 |
| `<Directory>`/`<Files>` 指令 | 全支持 | `<Directory>` 支持，`FilesMatch` 不触发 rewrite | 不支持 |
| cPanel/Plesk 插件 | 原生 | **官方插件** | 通过 CyberPanel |
| 配置方式 | httpd.conf + .htaccess | **读取 Apache 配置** + 自有 XML | 自有 conf 格式 |

### PHP 进程模型对比

| 特性 | Apache + php-fpm | LSWS + LSAPI | OLS + LSAPI |
|------|-----------------|--------------|-------------|
| 进程模型 | per-pool 独立进程 | ProcessGroup（per-user parent + fork） | per-vhost extprocessor |
| 用户隔离 | pool config `user=xxx` | suEXEC（`<phpSuExec>1`），自动以 docroot owner 运行 | suEXEC（`suEXEC User: $VH_USER`） |
| 空闲资源 | 每 pool 至少 1 进程常驻 | parent 空闲自动退出，最高效 | 空闲进程按 `LSAPI_MAX_IDLE` 退出 |
| Opcode cache | 全局共享（不安全）或 per-pool | **per-user opcode cache**（ProcessGroup 优势） | per-extprocessor |
| 进程数控制 | `pm.max_children` per pool | `PHP suEXEC Max Conn` 或 `LSPHP_Workers` per user | `maxConns` + `PHP_LSAPI_CHILDREN` per vhost |
| per-site PHP 版本 | 不同 pool 指向不同 PHP | `DedicatePhpHandler` 或 per-vhost handler | per-vhost extprocessor 指向不同 lsphp |
| PHP 配置覆盖 | `php_admin_value` in pool | `.htaccess` + `.user.ini` | `phpIniOverride` in vhconf + `.user.ini` |
| CloudLinux 兼容 | 需额外配置 | **原生兼容** ProcessGroup + CageFS | 基础兼容 |

### 关键结论

1. **LSWS 的最大优势**：自动读取 Apache 配置（drop-in），ProcessGroup 模式资源效率最高，`.htaccess` 中 `php_value` 不报错
2. **OLS 的核心差异**：不自动读取 Apache 配置，需要手动配置 vhost；PHP 配置不能写在 `.htaccess`，必须用 `.user.ini`
3. **LLStack 的策略**：canonical model 编译生成各后端配置，屏蔽了 OLS/LSWS 的差异。用户不需要关心底层 `.htaccess` 兼容性问题

## 2. LLStack Per-Site 隔离方案

### 用户模型

```
站点: wp.example.com
  → Linux 用户: wp_example_com（短名称）
  → 主组: wp_example_com
  → 辅助组: llstack-web（共享组）
  → 主目录: /data/www/wp.example.com
  → Shell: /sbin/nologin（后续可选分配 SSH）
```

### 文件权限

```
/data/www/wp.example.com/          drwxr-x---  wp_example_com:llstack-web  (750)
/data/www/wp.example.com/index.php -rw-r-----  wp_example_com:llstack-web  (640)
/data/www/wp.example.com/style.css -rw-r-----  wp_example_com:llstack-web  (640)
/data/www/wp.example.com/uploads/  drwxrwx---  wp_example_com:llstack-web  (770)
```

- PHP 进程以 `wp_example_com` 用户运行 → 可读写全部站点文件
- Web 服务器（Apache/OLS/LSWS）通过 `llstack-web` 组 → 可读文件，不可写
- 其他站点用户 → 完全无权限（无 `other` 位）
- `uploads/` 等写入目录设置 `770`，Web 服务器可通过组权限写入（用于静态文件上传场景）

### 安装时一次性设置

```bash
# 创建共享组（install 时执行一次）
groupadd llstack-web

# 将 Web 服务器用户加入共享组
# Apache:
usermod -aG llstack-web apache
# OLS/LSWS:
usermod -aG llstack-web nobody
```

### site:create 时执行

```bash
# 创建站点用户
useradd -r -s /sbin/nologin -d /data/www/wp.example.com -g wp_example_com -G llstack-web wp_example_com

# 设置目录权限
chown -R wp_example_com:llstack-web /data/www/wp.example.com
chmod 750 /data/www/wp.example.com
```

## 3. Per-Site PHP 进程配置

### Apache 后端：Per-Site FPM Pool

```ini
; /etc/opt/remi/php83/php-fpm.d/llstack-wp.example.com.conf
[wp.example.com]
user = wp_example_com
group = wp_example_com
listen = /var/opt/remi/php83/run/php-fpm/wp.example.com.sock
listen.owner = wp_example_com
listen.group = llstack-web
listen.mode = 0660

pm = dynamic
pm.max_children = 5          ; 硬件调优自动计算
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3

; 安全隔离
php_admin_value[open_basedir] = /data/www/wp.example.com:/tmp:/usr/share/php
php_admin_value[session.save_path] = /tmp
php_admin_value[upload_tmp_dir] = /tmp

; per-site 覆盖（用户通过 llstack site:php-config 设置）
php_admin_value[memory_limit] = 256M
php_admin_value[upload_max_filesize] = 64M
php_admin_value[post_max_size] = 64M
php_admin_value[max_execution_time] = 60
```

Apache vhost 引用 per-site socket：

```apache
<VirtualHost *:80>
    ServerName wp.example.com
    DocumentRoot /data/www/wp.example.com

    <FilesMatch \.php$>
        SetHandler "proxy:unix:/var/opt/remi/php83/run/php-fpm/wp.example.com.sock|fcgi://localhost"
    </FilesMatch>
</VirtualHost>
```

### OLS 后端：Per-Vhost Extprocessor + suEXEC

```
# vhconf.conf 中的 extprocessor（已有，需增加 suEXEC 字段）
extprocessor lsphp-wp-example-com {
  type                    lsapi
  address                 uds://tmp/lshttpd/wp.example.com.sock
  maxConns                5
  env                     PHP_LSAPI_CHILDREN=5
  env                     LSAPI_AVOID_FORK=200M
  initTimeout             60
  retryTimeout            0
  persistConn             1
  autoStart               2
  path                    /opt/remi/php83/root/usr/bin/lsphp
}

# vhost 级 suEXEC 设置（新增）
suEXEC User: wp_example_com
suEXEC Group: wp_example_com

# per-site PHP 覆盖（新增）
phpIniOverride {
  php_value memory_limit 256M
  php_value upload_max_filesize 64M
  php_value post_max_size 64M
  php_value max_execution_time 60
  php_admin_value open_basedir /data/www/wp.example.com:/tmp:/usr/share/php
}
```

用户额外可以在站点目录放 `.user.ini`：
```ini
; /data/www/wp.example.com/.user.ini
memory_limit = 512M
```

### LSWS 后端：ProcessGroup + suEXEC

LSWS 的 ProcessGroup 模式最高效：

```xml
<!-- httpd_config.xml 全局设置 -->
<phpSuExec>1</phpSuExec>
<phpSuExecMaxConn>5</phpSuExecMaxConn>
```

- PHP 进程自动以 docroot owner 运行（无需 per-site 配置 user/group）
- 每用户有独立 opcode cache
- 空闲 parent 自动退出
- per-site PHP 覆盖通过 `.user.ini` 或 `LSPHP_Workers` directive

per-account 并发控制（在 Apache 格式的 include 中）：
```apache
<IfModule LiteSpeed>
LSPHP_Workers 5
</IfModule>
```

## 4. 三后端 per-site PHP 覆盖参数白名单

| 参数 | Apache (php_admin_value) | OLS (phpIniOverride) | LSWS (.user.ini) | 说明 |
|------|-------------------------|---------------------|-------------------|------|
| `memory_limit` | ✓ | ✓ | ✓ | 内存上限 |
| `upload_max_filesize` | ✓ | ✓ | ✓ | 上传文件大小 |
| `post_max_size` | ✓ | ✓ | ✓ | POST 数据大小 |
| `max_execution_time` | ✓ | ✓ | ✓ | 执行超时 |
| `max_input_vars` | ✓ | ✓ | ✓ | 表单变量数 |
| `max_input_time` | ✓ | ✓ | ✓ | 输入解析超时 |
| `display_errors` | ✓ | ✓ | ✓ | 显示错误 |
| `error_reporting` | ✓ | ✓ | ✓ | 错误级别 |
| `open_basedir` | ✓ (admin only) | ✓ (admin only) | ✗ (不可覆盖) | 安全限制 |
| `session.save_path` | ✓ | ✓ | ✓ | Session 路径 |
| `opcache.memory_consumption` | ✓ | ✓ | ✓ | Opcode 缓存大小 |

## 5. 硬件感知调优公式

### 全局分配（基于总 RAM）

```
总 RAM = detect()

PHP 份额 = RAM * 0.40
DB 份额  = RAM * 0.25
Cache    = RAM * 0.10
Web      = RAM * 0.10
OS 保留  = RAM * 0.15
```

### Per-Site PHP Pool

```
每 worker 内存 ≈ avg_worker_mem（默认 80MB，WordPress ~120MB，Laravel ~100MB）
总 PHP workers = PHP 份额 / avg_worker_mem
per-site max_children = max(3, 总 PHP workers / 站点数)
per-site start_servers = max(1, per-site max_children / 4)
per-site min_spare = 1
per-site max_spare = max(2, per-site max_children / 2)
```

### 示例

| 配置 | 1GB RAM / 1站点 | 2GB RAM / 3站点 | 8GB RAM / 5站点 | 32GB RAM / 20站点 |
|------|----------------|----------------|----------------|------------------|
| PHP 总额 | 400MB | 800MB | 3200MB | 12800MB |
| 每站点 max_children | 5 | 3 | 8 | 8 |
| DB buffer_pool | 250MB | 500MB | 2000MB | 8000MB |
| Redis maxmemory | 100MB | 200MB | 800MB | 3200MB |
| Apache MaxRequestWorkers | 50 | 100 | 400 | 400 |

## 6. 代码改动清单

### 新增文件

| 文件 | 功能 |
|------|------|
| `internal/system/user.go` | Linux 用户创建/删除/查询 |
| `internal/system/hardware.go` | CPU/RAM 检测 |
| `internal/tuning/tuning.go` | 硬件感知参数计算 |
| `internal/tuning/profiles.go` | per-component 调优 profile |

### 修改文件

| 文件 | 改动 |
|------|------|
| `internal/site/service.go` | Create/Delete 增加用户管理 + FPM pool 生成 |
| `internal/backend/apache/renderer.go` | vhost 引用 per-site FPM socket |
| `internal/backend/ols/compiler.go` | extprocessor 增加 suEXEC user/group + phpIniOverride |
| `internal/backend/lsws/renderer.go` | 增加 suEXEC 配置 |
| `internal/php/service.go` | Install/TunePool 适配 per-site pool |
| `internal/install/service.go` | 安装时创建 llstack-web 组 + 硬件调优 |
| `internal/config/config.go` | 增加 tuning 相关配置字段 |
| `internal/tui/model.go` | install wizard 增加调优预览 |

## 7. 需要确认的决策

### Q1：共享组名

建议 `llstack-web`。

### Q2：open_basedir

建议默认启用，限制为 `/data/www/<site>:/tmp:/usr/share/php`。

### Q3：OLS PHP 进程模型

建议 per-vhost extprocessor + suEXEC（当前 OLS compiler 已是 per-vhost，只需增加 suEXEC 字段和 phpIniOverride）。

### Q4：FPM pool 命名

建议用域名 `[wp.example.com]`。

### Q5：硬件调优比例

PHP 40% / DB 25% / Cache 10% / Web 10% / OS 15%。

### Q6：Rate Limiting 统一抽象

建议统一为**请求频率限制**语义（req/s）：
- Apache：使用 `mod_evasive`（限请求频率，非 `mod_ratelimit` 限带宽）
- OLS：`perClientConnLimit { dynReqPerSec N }`
- LSWS：同 OLS 格式

## 8. 实现顺序

```
第一步：system/user.go + system/hardware.go（基础设施）
第二步：Per-site 用户隔离（site:create 创建用户 + 权限设置）
第三步：Per-site FPM pool（Apache 后端）
第四步：OLS suEXEC + phpIniOverride（OLS 后端）
第五步：LSWS ProcessGroup suEXEC（LSWS 后端）
第六步：硬件感知调优（tuning 子系统）
第七步：Per-site PHP 配置覆盖 CLI/TUI
第八步：PHP 7.4-8.5 版本扩展 + Valkey
第九步：SSL 证书生命周期
第十步：Cron 任务管理
第十一步：安全加固（fail2ban / ratelimit / firewalld）
第十二步：OLS .htaccess 兼容性优化（htaccess-check / htaccess-compile / htaccess-watch）
```

## 9. 已确认决策记录

| 决策 | 结论 | 确认来源 |
|------|------|----------|
| 文件权限模型 | **方案 B：per-site 组**（每建站点将 Web 服务器用户加入该站点组） | 用户确认 |
| 共享组 | 不使用共享组 | 用户选方案 B |
| open_basedir | 默认启用 | 建议 |
| OLS PHP 进程模型 | per-vhost extprocessor + suEXEC | 建议 |
| LSWS PHP 进程模型 | ProcessGroup + suEXEC（全局 `<phpSuExec>1`） | 文档研究 |
| FPM pool 命名 | 域名 `[wp.example.com]` | 建议 |
| 硬件调优比例 | PHP 40% / DB 25% / Cache 10% / Web 10% / OS 15% | 建议 |
| Rate limiting | 统一请求频率语义，Apache mod_evasive / OLS perClientConnLimit | 用户确认 |
| 站点用户命名 | 最长 12 字符：`{域名前缀≤7}_{hash4}`，如 `wp_a3f2` | 用户确认 |
| 文件权限模型 | 方案 B：per-site 组 | 用户确认 |
| htaccess-compile | 默认不修改，`--apply` 才改 | 用户确认 |
| htaccess-watch | 不默认启用，OLS 站点创建时输出 hint | 用户确认 |
| htaccess-check | 纳入 doctor（`ols_htaccess_compat`） | 用户确认 |
| SSH 权限 | 后续可选分配 | 用户确认 |
| 旧站点迁移 | 不需要（开发阶段） | 用户确认 |

详细的 OLS .htaccess 兼容性优化方案见 [DESIGN_OLS_HTACCESS_COMPAT.md](DESIGN_OLS_HTACCESS_COMPAT.md)。
