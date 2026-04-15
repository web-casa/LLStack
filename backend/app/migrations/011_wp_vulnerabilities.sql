-- Panel config key-value store (used by vuln db sync, notifications, etc.)
CREATE TABLE IF NOT EXISTS panel_config (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Wordfence Intelligence vulnerability database cache
CREATE TABLE IF NOT EXISTS wp_vulnerabilities (
    id TEXT PRIMARY KEY,
    software_type TEXT NOT NULL,
    software_slug TEXT NOT NULL,
    title TEXT,
    description TEXT,
    cvss_score REAL,
    cvss_vector TEXT,
    cve_id TEXT,
    cwe_id TEXT,
    affected_versions TEXT,
    patched_version TEXT,
    remediation TEXT,
    published_at TEXT,
    updated_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_vuln_slug ON wp_vulnerabilities(software_slug);
CREATE INDEX IF NOT EXISTS idx_vuln_type_slug ON wp_vulnerabilities(software_type, software_slug);
CREATE INDEX IF NOT EXISTS idx_vuln_cve ON wp_vulnerabilities(cve_id);
