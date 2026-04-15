-- LLStack Panel Initial Schema

-- Users
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'user',
    system_user TEXT UNIQUE,
    home_dir TEXT,
    plan_id INTEGER REFERENCES plans(id),
    totp_secret TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Plans (resource quotas)
CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    max_sites INTEGER DEFAULT 10,
    max_databases INTEGER DEFAULT 10,
    disk_quota_mb INTEGER DEFAULT 10240,
    php_versions_allowed TEXT,
    php_can_switch_version BOOLEAN DEFAULT 1,
    php_can_edit_settings BOOLEAN DEFAULT 1,
    php_memory_limit_max TEXT DEFAULT '256M',
    php_max_execution_time_max INTEGER DEFAULT 300,
    php_upload_max_filesize_max TEXT DEFAULT '64M',
    php_post_max_size_max TEXT DEFAULT '64M',
    redis_enabled BOOLEAN DEFAULT 0,
    redis_maxmemory_mb INTEGER DEFAULT 64,
    redis_max_connections INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Sites
CREATE TABLE IF NOT EXISTS sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    domain TEXT UNIQUE NOT NULL,
    aliases TEXT,
    doc_root TEXT NOT NULL,
    php_version TEXT,
    ssl_enabled BOOLEAN DEFAULT 0,
    status TEXT DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Database instances
CREATE TABLE IF NOT EXISTS db_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    engine TEXT NOT NULL,
    name TEXT NOT NULL,
    host TEXT,
    port INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User-level PHP config overrides
CREATE TABLE IF NOT EXISTS user_php_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    php_version TEXT NOT NULL,
    config_key TEXT NOT NULL,
    config_value TEXT NOT NULL,
    UNIQUE(user_id, php_version, config_key)
);

-- Site-level PHP config overrides
CREATE TABLE IF NOT EXISTS site_php_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER REFERENCES sites(id),
    config_key TEXT NOT NULL,
    config_value TEXT NOT NULL,
    UNIQUE(site_id, config_key)
);

-- Redis user instances
CREATE TABLE IF NOT EXISTS redis_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) UNIQUE,
    socket_path TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    maxmemory_mb INTEGER DEFAULT 64,
    maxmemory_policy TEXT DEFAULT 'allkeys-lru',
    status TEXT DEFAULT 'stopped',
    pid INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Redis DB assignments
CREATE TABLE IF NOT EXISTS redis_db_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    redis_instance_id INTEGER REFERENCES redis_instances(id),
    db_number INTEGER NOT NULL,
    site_id INTEGER REFERENCES sites(id),
    label TEXT,
    UNIQUE(redis_instance_id, db_number)
);

-- Audit logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    action TEXT NOT NULL,
    target TEXT,
    detail TEXT,
    ip TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Backups
CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER REFERENCES sites(id),
    type TEXT,
    path TEXT,
    size INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cron jobs
CREATE TABLE IF NOT EXISTS cron_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    expression TEXT NOT NULL,
    command TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 1,
    description TEXT
);
