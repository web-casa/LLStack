-- Phase 4: Operations Enhancement

-- PHP version registry
CREATE TABLE IF NOT EXISTS php_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT UNIQUE NOT NULL,
    display_version TEXT,
    lsphp_path TEXT,
    ini_path TEXT,
    installed_at TEXT DEFAULT (datetime('now')),
    status TEXT DEFAULT 'active'
);

-- Bandwidth usage tracking
CREATE TABLE IF NOT EXISTS bandwidth_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    month TEXT NOT NULL,
    bytes_in INTEGER DEFAULT 0,
    bytes_out INTEGER DEFAULT 0,
    updated_at TEXT,
    UNIQUE(site_id, month)
);

-- Plan bandwidth quota (add column if not exists)
-- Note: SQLite ALTER TABLE ADD COLUMN is idempotent with IF NOT EXISTS via try/catch in Python
