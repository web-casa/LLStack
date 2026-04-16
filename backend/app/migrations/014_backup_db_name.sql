-- Track which database was included in each backup for accurate restore
ALTER TABLE backups ADD COLUMN db_name TEXT DEFAULT NULL;
