-- WordPress instance tracking
CREATE TABLE IF NOT EXISTS wp_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    uuid TEXT UNIQUE NOT NULL,
    path TEXT NOT NULL,
    version TEXT,
    site_url TEXT,
    title TEXT,
    status TEXT DEFAULT 'active',  -- active / error / not_found
    detected_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT
);
