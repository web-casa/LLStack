-- RBAC: Expand roles + site-level permissions for developer role
-- Roles: owner (first user), admin, developer, viewer
-- Existing 'admin' users become 'admin', existing 'user' users become 'developer'

-- Migrate existing 'user' role to 'developer'
UPDATE users SET role = 'developer' WHERE role = 'user';

-- Promote the first user (id=1) to 'owner' if they are currently 'admin'
UPDATE users SET role = 'owner' WHERE id = (SELECT MIN(id) FROM users) AND role = 'admin';

-- Site-level permissions for developer/viewer roles
CREATE TABLE IF NOT EXISTS user_site_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    site_id INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, site_id)
);

-- Add quota fields to plans (if not present)
-- max_db_size_mb: per-database size limit
-- max_bandwidth_gb: monthly bandwidth quota
ALTER TABLE plans ADD COLUMN max_db_size_mb INTEGER DEFAULT 0;
ALTER TABLE plans ADD COLUMN max_bandwidth_gb INTEGER DEFAULT 0;

-- DB size tracking
CREATE TABLE IF NOT EXISTS db_size_tracking (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    db_instance_id INTEGER REFERENCES db_instances(id) ON DELETE CASCADE,
    size_mb REAL NOT NULL,
    tracked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_db_size_instance ON db_size_tracking(db_instance_id);
