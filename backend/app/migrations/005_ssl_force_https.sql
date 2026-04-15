-- Add ssl_force_https column to sites
ALTER TABLE sites ADD COLUMN ssl_force_https BOOLEAN DEFAULT 0;
