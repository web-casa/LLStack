-- WordPress plugin/theme asset library + per-instance associations
CREATE TABLE IF NOT EXISTS wp_plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    name TEXT,
    latest_version TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS wp_plugin_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER REFERENCES wp_instances(id) ON DELETE CASCADE,
    plugin_id INTEGER REFERENCES wp_plugins(id) ON DELETE CASCADE,
    version TEXT,
    active INTEGER DEFAULT 0,
    auto_update INTEGER DEFAULT 0,
    UNIQUE(instance_id, plugin_id)
);

CREATE TABLE IF NOT EXISTS wp_themes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    name TEXT,
    latest_version TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS wp_theme_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER REFERENCES wp_instances(id) ON DELETE CASCADE,
    theme_id INTEGER REFERENCES wp_themes(id) ON DELETE CASCADE,
    version TEXT,
    active INTEGER DEFAULT 0,
    UNIQUE(instance_id, theme_id)
);
