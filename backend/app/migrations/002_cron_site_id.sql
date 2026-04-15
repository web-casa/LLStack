-- Add site_id to cron_jobs for per-site cron management
ALTER TABLE cron_jobs ADD COLUMN site_id INTEGER REFERENCES sites(id);
