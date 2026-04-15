-- Add is_admin_value column to site_php_config
-- 1 = php_admin_value (cannot be overridden by ini_set())
-- 0 = php_value (can be overridden)
ALTER TABLE site_php_config ADD COLUMN is_admin_value INTEGER NOT NULL DEFAULT 0;
