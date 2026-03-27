# LLStack VPS 端到端测试计划

## 目的

在真实 EL9 / EL10 VPS 上以用户视角执行完整测试，验证 LLStack 从安装到运行真实 PHP 应用的全链路，覆盖所有支持的 PHP 版本、数据库 provider 和缓存组件的安装与使用。

## 环境要求

- Rocky Linux 9 x86_64 VPS × 2（最低 2 vCPU / 2GB RAM）
- Rocky Linux 10 x86_64 VPS × 1（同上）
- 如有域名可测 Let's Encrypt，否则跳过 ACME 部分

## 测试总览

```
Phase 1   基础命令验证
Phase 2   PHP 多版本安装矩阵
Phase 3   数据库 provider 安装矩阵
Phase 4   缓存组件安装矩阵
Phase 5   WordPress 真实部署（Apache + PHP 8.3 + MariaDB + Memcached）
Phase 6   Laravel 真实部署（Apache + PHP 8.4 + PostgreSQL + Redis）
Phase 7   OLS 后端 + 静态站点
Phase 8   站点生命周期全操作
Phase 9   Doctor 完整验证 + Repair
Phase 10  Diagnostics Bundle
Phase 11  卸载 / 备份 / 清理
Phase 12  Let's Encrypt（可选，需域名）
```

---

## Phase 1：基础命令验证

```bash
llstack version --json
llstack status --json
llstack doctor --json
llstack install --backend apache --php_version 8.3 --db mariadb --site test.com --dry-run --json
```

预期：version 输出正确、status 显示 unconfigured、doctor 识别 OS、dry-run 生成 plan。

---

## Phase 2：PHP 多版本安装矩阵

### 支持版本：8.2 / 8.3 / 8.4

对每个版本执行：

```bash
VERSION=8.3  # 替换为 8.2 / 8.3 / 8.4

# 安装
llstack php:install $VERSION --json

# 验证
php${VERSION//.} -v                                # CLI 版本
systemctl is-active php${VERSION//.}-php-fpm       # FPM 服务状态
ls /var/opt/remi/php${VERSION//.}/run/php-fpm/     # socket 存在
ls /etc/llstack/php/runtimes/                      # manifest 写入
cat /etc/opt/remi/php${VERSION//.}/php.d/90-llstack-profile.ini  # profile 写入

# 扩展验证
php${VERSION//.} -m | grep -i mbstring
php${VERSION//.} -m | grep -i pdo
php${VERSION//.} -m | grep -i xml
php${VERSION//.} -m | grep -i opcache
```

验证矩阵：

| 项目 | PHP 8.2 | PHP 8.3 | PHP 8.4 |
|------|---------|---------|---------|
| dnf 安装成功 | | | |
| CLI 版本正确 | | | |
| FPM 服务 active | | | |
| FPM socket 存在 | | | |
| runtime manifest 写入 | | | |
| profile ini 写入 | | | |
| 核心扩展加载 | | | |

### PHP 卸载验证

```bash
# 卸载 PHP 8.2（保留 8.3 和 8.4）
llstack php:remove 8.2 --json

# 验证
systemctl is-active php82-php-fpm 2>&1   # 应 inactive
rpm -qa | grep php82                      # 应无输出
ls /etc/llstack/php/runtimes/8-2.json 2>&1  # 应不存在
```

### Pool Tuning 验证

```bash
llstack php:tune 8.3 --max-children 30 --start-servers 5 --min-spare 3 --max-spare 20 --json

# 验证
cat /etc/opt/remi/php83/php-fpm.d/90-llstack-pool.conf
systemctl is-active php83-php-fpm  # 应 active（restart 后）
```

---

## Phase 3：数据库 Provider 安装矩阵

### 支持 provider：MariaDB / MySQL / PostgreSQL / Percona

每个 provider 在独立 VPS 或顺序测试（先卸载再装下一个）：

```bash
PROVIDER=mariadb  # 替换为 mariadb / mysql / postgresql / percona

# 安装
llstack db:install $PROVIDER --tls enabled --json

# 验证服务
systemctl is-active $SERVICE_NAME  # mariadb / mysqld / postgresql-16

# 初始化
llstack db:init $PROVIDER \
  --admin-user llstack_admin \
  --admin-pass test_admin_pass \
  --json

# 创建数据库
llstack db:create-database $PROVIDER --name testdb --json

# 创建用户
llstack db:create-user $PROVIDER \
  --name testuser \
  --password testpass \
  --database testdb \
  --json

# 连接验证
```

#### MariaDB 连接验证

```bash
mysql -u testuser -ptestpass -e "SHOW DATABASES;" | grep testdb
mysql -u testuser -ptestpass testdb -e "CREATE TABLE test_table (id INT PRIMARY KEY, name VARCHAR(100));"
mysql -u testuser -ptestpass testdb -e "INSERT INTO test_table VALUES (1, 'hello');"
mysql -u testuser -ptestpass testdb -e "SELECT * FROM test_table;"
```

#### MySQL 连接验证

```bash
mysql -u testuser -ptestpass -e "SHOW DATABASES;" | grep testdb
mysql -u testuser -ptestpass testdb -e "CREATE TABLE test_table (id INT PRIMARY KEY, name VARCHAR(100));"
mysql -u testuser -ptestpass testdb -e "SELECT * FROM test_table;"
```

#### PostgreSQL 连接验证

```bash
PGPASSWORD=testpass psql -h 127.0.0.1 -U testuser -d testdb -c "\dt"
PGPASSWORD=testpass psql -h 127.0.0.1 -U testuser -d testdb -c "CREATE TABLE test_table (id INT PRIMARY KEY, name VARCHAR(100));"
PGPASSWORD=testpass psql -h 127.0.0.1 -U testuser -d testdb -c "INSERT INTO test_table VALUES (1, 'hello');"
PGPASSWORD=testpass psql -h 127.0.0.1 -U testuser -d testdb -c "SELECT * FROM test_table;"
```

#### Percona 连接验证

```bash
mysql -u testuser -ptestpass -e "SHOW DATABASES;" | grep testdb
```

验证矩阵：

| 项目 | MariaDB | MySQL | PostgreSQL | Percona |
|------|---------|-------|------------|---------|
| dnf 安装成功 | | | | |
| 服务 active | | | | |
| init 成功 | | | | |
| create database 成功 | | | | |
| create user 成功 | | | | |
| 客户端连接成功 | | | | |
| CRUD 操作正常 | | | | |
| TLS 配置写入 | | | | |
| manifest 写入 | | | | |
| credential file 写入 | | | | |

### DB 备份验证

```bash
llstack db:backup mariadb --json
ls /var/lib/llstack/backups/db/mariadb/  # 应有 .sql 文件
head -5 /var/lib/llstack/backups/db/mariadb/*.sql  # 应为有效 SQL
```

### DB 卸载验证

```bash
llstack db:remove mariadb --json
systemctl is-active mariadb 2>&1  # 应 inactive/dead
```

---

## Phase 4：缓存组件安装矩阵

### Memcached

```bash
llstack cache:install memcached --json
systemctl is-active memcached

llstack cache:configure memcached --bind 127.0.0.1 --port 11211 --max-memory 128 --json
cat /etc/sysconfig/memcached

# 连接验证
echo "stats" | nc -q1 127.0.0.1 11211 | head -5
```

### Redis

```bash
llstack cache:install redis --json
systemctl is-active redis

llstack cache:configure redis --bind 127.0.0.1 --port 6379 --max-memory 128 --json
grep maxmemory /etc/redis.conf

# 连接验证
redis-cli PING          # 应返回 PONG
redis-cli SET test ok
redis-cli GET test      # 应返回 ok
redis-cli INFO memory | grep used_memory_human
```

验证矩阵：

| 项目 | Memcached | Redis |
|------|-----------|-------|
| dnf 安装成功 | | |
| 服务 active | | |
| configure 写入配置 | | |
| 客户端连接成功 | | |
| 数据读写正常 | | |

---

## Phase 5：WordPress 真实部署

环境：Apache + PHP 8.3 + MariaDB + Memcached

```bash
# 创建站点
llstack site:create wp.test.com \
  --backend apache \
  --profile wordpress \
  --non-interactive \
  --json

# 创建数据库
llstack db:create-database mariadb --name wordpress_db --json
llstack db:create-user mariadb --name wp_user --password wp_secret --database wordpress_db --json

# 安装 WordPress 需要的额外 PHP 扩展
dnf -y install php83-php-gd php83-php-zip php83-php-curl php83-php-imagick
systemctl restart php83-php-fpm

# 部署 WordPress
cd /data/www/wp.test.com
curl -O https://wordpress.org/latest.tar.gz
tar xzf latest.tar.gz --strip-components=1
rm latest.tar.gz

# 配置
cp wp-config-sample.php wp-config.php
sed -i "s/database_name_here/wordpress_db/" wp-config.php
sed -i "s/username_here/wp_user/" wp-config.php
sed -i "s/password_here/wp_secret/" wp-config.php

# 设置权限
chown -R apache:apache /data/www/wp.test.com

# 验证
curl -sS -H "Host: wp.test.com" http://127.0.0.1/ | head -20
```

验证项：

- [ ] WordPress 安装向导页面显示（不是 PHP 错误）
- [ ] 数据库连接正常（无 "Error establishing a database connection"）
- [ ] PHP-FPM 处理请求（不是源码显示）
- [ ] 静态资源加载（wp-includes/css/、wp-includes/js/）
- [ ] wp-cron 可访问（`curl -sS -H "Host: wp.test.com" http://127.0.0.1/wp-cron.php`）

### WordPress + Memcached 缓存验证

```bash
# 安装 memcached PHP 扩展
dnf -y install php83-php-pecl-memcached
systemctl restart php83-php-fpm

# 安装 object-cache 插件（可选）
# 验证 Memcached 扩展加载
php83 -m | grep memcached
```

---

## Phase 6：Laravel 真实部署

环境：Apache + PHP 8.4 + PostgreSQL + Redis

```bash
# 安装 PHP 8.4
llstack php:install 8.4 --json

# 安装 PostgreSQL
llstack db:install postgresql --tls enabled --json
llstack db:init postgresql --admin-user postgres --admin-pass pg_secret --json
llstack db:create-database postgresql --name laravel_db --json
llstack db:create-user postgresql --name laravel_user --password laravel_pass --database laravel_db --json

# 创建站点
llstack site:create laravel.test.com \
  --backend apache \
  --profile laravel \
  --non-interactive \
  --json

# 安装 PHP 扩展
dnf -y install php84-php-pgsql php84-php-mbstring php84-php-xml php84-php-curl php84-php-zip php84-php-bcmath php84-php-tokenizer
systemctl restart php84-php-fpm

# 安装 Composer
curl -sS https://getcomposer.org/installer | php84 -- --install-dir=/usr/local/bin --filename=composer

# 部署 Laravel
cd /data/www/laravel.test.com
rm -rf public
composer create-project laravel/laravel .

# 配置
sed -i "s/DB_CONNECTION=.*/DB_CONNECTION=pgsql/" .env
sed -i "s/DB_HOST=.*/DB_HOST=127.0.0.1/" .env
sed -i "s/DB_DATABASE=.*/DB_DATABASE=laravel_db/" .env
sed -i "s/DB_USERNAME=.*/DB_USERNAME=laravel_user/" .env
sed -i "s/DB_PASSWORD=.*/DB_PASSWORD=laravel_pass/" .env
sed -i "s/CACHE_STORE=.*/CACHE_STORE=redis/" .env
sed -i "s/SESSION_DRIVER=.*/SESSION_DRIVER=redis/" .env

# 权限
chown -R apache:apache /data/www/laravel.test.com
chmod -R 775 storage bootstrap/cache

# 迁移
php84 artisan migrate --force
php84 artisan key:generate

# 验证
curl -sS -H "Host: laravel.test.com" http://127.0.0.1/ | head -20
```

验证项：

- [ ] Laravel 欢迎页显示
- [ ] `artisan migrate` 成功（PostgreSQL 连接正常）
- [ ] DocumentRoot 指向 public/（Laravel profile 自动处理）
- [ ] rewrite 规则生效（访问 /non-existent 不返回 Apache 404，而是 Laravel 404 页面）
- [ ] Redis session/cache 可用（`redis-cli KEYS '*'` 应有 laravel session/cache key）

---

## Phase 7：OLS 后端 + PHP 站点

```bash
# 安装 OLS 核心
printf '[litespeed]\nname=LiteSpeed Tech\nbaseurl=http://rpms.litespeedtech.com/centos/$releasever/$basearch/\nenabled=1\ngpgcheck=0\n' > /etc/yum.repos.d/litespeed.repo
dnf download openlitespeed --destdir=/tmp/ols-rpms
rpm -ivh --nodeps /tmp/ols-rpms/openlitespeed-*.rpm

# 创建 OLS 站点
llstack site:create ols.test.com \
  --backend ols \
  --profile static \
  --non-interactive \
  --skip-reload \
  --json

# 验证配置
cat /usr/local/lsws/conf/vhosts/ols.test.com/vhconf.conf
cat /usr/local/lsws/conf/llstack/listeners/ols.test.com.map
grep LLSTACK /usr/local/lsws/conf/httpd_config.conf

# 启动 OLS
/usr/local/lsws/bin/lswsctrl start
sleep 2
curl -sS -H "Host: ols.test.com" http://127.0.0.1:80/ | head -5
```

---

## Phase 8：站点生命周期全操作

```bash
for site in wp.test.com laravel.test.com; do
  echo "=== $site ==="

  # 显示
  llstack site:show $site --json | head -5

  # 漂移检测
  llstack site:diff $site --json | head -5

  # 停止 → 验证不可访问 → 启动
  llstack site:stop $site --json
  curl -sS -o /dev/null -w "%{http_code}" -H "Host: $site" http://127.0.0.1/
  llstack site:start $site --json
  curl -sS -o /dev/null -w "%{http_code}" -H "Host: $site" http://127.0.0.1/

  # 重启
  llstack site:restart $site --json

  # 更新设置
  llstack site:update $site --alias "www.$site" --json

  # 日志
  llstack site:logs $site --kind access --lines 5
  llstack site:logs $site --kind error --lines 5
done
```

---

## Phase 9：Doctor 完整验证

```bash
# 完整报告
llstack doctor --json

# 逐项验证预期
# os_support:              pass (rocky 9.x / 10.x)
# managed_directories:     pass
# managed_sites:           pass
# runtime_binaries:        pass (httpd + php)
# selinux_state:           pass/warn
# firewalld_state:         pass/warn
# listening_ports:         pass (80 开放)
# php_fpm_sockets:         pass
# php_fpm_process_health:  pass
# php_config_drift:        pass
# managed_path_ownership:  pass
# managed_path_permissions: pass
# managed_selinux_contexts: pass/warn
# db_tls_state:            pass
# db_managed_artifacts:    pass
# managed_services:        pass
# managed_provider_ports:  pass
# managed_db_live_probe:   pass
# managed_db_auth_probe:   pass
# managed_cache_live_probe: pass
# db_connection_saturation: pass
# cache_memory_saturation:  pass
# rollback_history:        pass

# Repair dry-run
llstack repair --dry-run --json
```

---

## Phase 10：Diagnostics Bundle

```bash
llstack doctor:bundle --json
tar tzf /var/lib/llstack/state/diagnostics/llstack-diagnostics-*.tar.gz | head -40
```

---

## Phase 11：卸载 / 备份 / 清理

```bash
# 备份数据库
llstack db:backup mariadb --json
ls -la /var/lib/llstack/backups/db/mariadb/

# 删除站点
llstack site:delete wp.test.com --json
llstack site:delete laravel.test.com --json
httpd -S 2>&1 | grep -c "wp.test.com"  # 应为 0

# 卸载 PHP
llstack php:remove 8.2 --json
rpm -qa | grep php82  # 应为空

# 卸载数据库
llstack db:remove mariadb --json
systemctl is-active mariadb 2>&1  # 应 inactive
```

---

## Phase 12：Let's Encrypt（需要真实域名）

前置条件：域名 A 记录指向 VPS IP。

```bash
dnf -y install certbot

llstack site:ssl realsite.example.com \
  --letsencrypt \
  --email admin@example.com \
  --json

# 验证
curl -I https://realsite.example.com
openssl s_client -connect realsite.example.com:443 -servername realsite.example.com < /dev/null 2>/dev/null | openssl x509 -noout -dates
```

---

## 完整验证矩阵

### PHP 版本 × 功能

| 功能 | PHP 8.2 | PHP 8.3 | PHP 8.4 |
|------|---------|---------|---------|
| dnf 安装 | | | |
| FPM 启动 | | | |
| socket 存在 | | | |
| CLI 执行 | | | |
| 扩展加载 | | | |
| WordPress 可运行 | | ← 主测 | |
| Laravel 可运行 | | | ← 主测 |
| pool tuning | | ← 主测 | |
| 卸载 | ← 主测 | | |

### DB Provider × 功能

| 功能 | MariaDB | MySQL | PostgreSQL | Percona |
|------|---------|-------|------------|---------|
| dnf 安装 | | | | |
| 服务启动 | | | | |
| init | | | | |
| create database | | | | |
| create user | | | | |
| CRUD 验证 | | | | |
| TLS 配置 | | | | |
| WordPress 连接 | ← 主测 | | | |
| Laravel 连接 | | | ← 主测 | |
| 备份 | ← 主测 | | ← 主测 | |
| 卸载 | ← 主测 | | | |

### Cache × 功能

| 功能 | Memcached | Redis |
|------|-----------|-------|
| dnf 安装 | | |
| 服务启动 | | |
| configure | | |
| 连接验证 | | |
| WordPress 集成 | ← 主测 | |
| Laravel session/cache | | ← 主测 |

### 后端 × 功能

| 功能 | Apache | OLS | LSWS |
|------|--------|-----|------|
| 安装 | ← 主测 | ← 主测 | 跳过（需授权） |
| site:create | ✓ | ✓ | |
| HTTP 响应 | ✓ | ✓ | |
| PHP 执行 | ✓ WordPress + Laravel | | |
| configtest | ✓ | ✓ | |
| 主配置管理 | IncludeOptional | 自动注册 | |
| site lifecycle | ✓ | ✓ | |

### OS × 整体

| 测试 | EL9 (Rocky 9) | EL10 (Rocky 10) |
|------|---------------|-----------------|
| 完整安装链路 | ← 主测 | ← 精简 |
| WordPress | ✓ | |
| Laravel | ✓ | |
| OLS 后端 | ✓ | |
| Doctor 全项 | ✓ | ✓ |

---

## 执行策略

建议用 3 台 VPS：

| VPS | OS | 用途 |
|-----|----|------|
| VPS-1 | EL9 | Apache + 全部 PHP 版本 + MariaDB + Memcached + WordPress |
| VPS-2 | EL9 | Apache + PostgreSQL + Redis + Laravel + OLS 测试 |
| VPS-3 | EL10 | Apache + PHP 8.3 + MariaDB + 基础验证 |

预计耗时：~30 分钟自动化执行 + 人工验证关键页面。

测试完成后立即销毁 VPS。
