-- Staging environment support
ALTER TABLE sites ADD COLUMN staging_of INTEGER REFERENCES sites(id) ON DELETE SET NULL;
