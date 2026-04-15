-- Add cgroup resource limit fields to plans
ALTER TABLE plans ADD COLUMN cpu_limit_percent INTEGER DEFAULT 0;    -- 0=unlimited, 100=1 core, 200=2 cores
ALTER TABLE plans ADD COLUMN memory_limit_mb INTEGER DEFAULT 0;      -- 0=unlimited
ALTER TABLE plans ADD COLUMN io_limit_mbps INTEGER DEFAULT 0;        -- 0=unlimited
ALTER TABLE plans ADD COLUMN tasks_max INTEGER DEFAULT 0;            -- 0=unlimited
ALTER TABLE plans ADD COLUMN cgroup_enabled BOOLEAN DEFAULT 0;       -- master switch
