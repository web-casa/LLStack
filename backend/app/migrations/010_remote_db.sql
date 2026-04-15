-- Support remote/cloud database and Redis connections
ALTER TABLE db_instances ADD COLUMN connection_type TEXT DEFAULT 'local';
ALTER TABLE db_instances ADD COLUMN db_username TEXT;
ALTER TABLE db_instances ADD COLUMN db_password_encrypted TEXT;

ALTER TABLE redis_instances ADD COLUMN host TEXT;
ALTER TABLE redis_instances ADD COLUMN port INTEGER DEFAULT 6379;
ALTER TABLE redis_instances ADD COLUMN connection_type TEXT DEFAULT 'local';
