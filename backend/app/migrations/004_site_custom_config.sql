-- Per-site custom vhost config injection points
CREATE TABLE IF NOT EXISTS site_custom_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    hook_point TEXT NOT NULL,  -- CUSTOM_HEAD, CUSTOM_HANDLER, CUSTOM_REWRITE, CUSTOM_PHP, CUSTOM_TAIL
    content TEXT NOT NULL DEFAULT '',
    UNIQUE(site_id, hook_point)
);
