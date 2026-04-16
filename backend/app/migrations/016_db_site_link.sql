-- Link databases to sites for permission cascading
ALTER TABLE db_instances ADD COLUMN site_id INTEGER REFERENCES sites(id) ON DELETE SET NULL DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_db_instances_site ON db_instances(site_id);
