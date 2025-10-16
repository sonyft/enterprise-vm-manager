-- Initial schema for VM Manager
-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Create custom types
CREATE TYPE vm_status AS ENUM (
    'pending',
    'stopped', 
    'starting',
    'running',
    'stopping',
    'suspended',
    'error'
);

CREATE TYPE network_type AS ENUM (
    'nat',
    'bridge',
    'host'
);

-- Create virtual_machines table
CREATE TABLE virtual_machines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,

    -- VM Specification (embedded)
    cpu_cores INTEGER NOT NULL CHECK (cpu_cores > 0 AND cpu_cores <= 64),
    ram_mb INTEGER NOT NULL CHECK (ram_mb >= 512 AND ram_mb <= 524288),
    disk_gb INTEGER NOT NULL CHECK (disk_gb >= 10 AND disk_gb <= 10240),
    image_name VARCHAR(255) NOT NULL,
    network_type network_type DEFAULT 'nat',
    boot_order VARCHAR(50) DEFAULT 'hd',

    -- VM State
    status vm_status DEFAULT 'stopped',
    power_state VARCHAR(10) DEFAULT 'off',

    -- Metadata
    labels JSONB,
    annotations JSONB,

    -- Resource allocation
    node_id VARCHAR(255),

    -- VM Statistics (embedded)
    cpu_usage_percent DOUBLE PRECISION DEFAULT 0,
    ram_usage_percent DOUBLE PRECISION DEFAULT 0,
    disk_usage_percent DOUBLE PRECISION DEFAULT 0,
    network_rx_bytes BIGINT DEFAULT 0,
    network_tx_bytes BIGINT DEFAULT 0,
    uptime_seconds BIGINT DEFAULT 0,
    last_stats_update TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    stopped_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Create indexes for better performance
CREATE INDEX idx_virtual_machines_name ON virtual_machines(name);
CREATE INDEX idx_virtual_machines_status ON virtual_machines(status);
CREATE INDEX idx_virtual_machines_node_id ON virtual_machines(node_id);
CREATE INDEX idx_virtual_machines_created_by ON virtual_machines(created_by);
CREATE INDEX idx_virtual_machines_created_at ON virtual_machines(created_at);
CREATE INDEX idx_virtual_machines_updated_at ON virtual_machines(updated_at);
CREATE INDEX idx_virtual_machines_deleted_at ON virtual_machines(deleted_at);

-- GIN indexes for JSONB fields
CREATE INDEX idx_virtual_machines_labels ON virtual_machines USING GIN(labels);
CREATE INDEX idx_virtual_machines_annotations ON virtual_machines USING GIN(annotations);

-- Text search index for name and description
CREATE INDEX idx_virtual_machines_search ON virtual_machines USING GIN(
    to_tsvector('english', COALESCE(name, '') || ' ' || COALESCE(description, ''))
);

-- Trigram indexes for fuzzy search
CREATE INDEX idx_virtual_machines_name_trgm ON virtual_machines USING GIN(name gin_trgm_ops);
CREATE INDEX idx_virtual_machines_desc_trgm ON virtual_machines USING GIN(description gin_trgm_ops);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
CREATE TRIGGER update_virtual_machines_updated_at 
    BEFORE UPDATE ON virtual_machines 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Function to validate status transitions
CREATE OR REPLACE FUNCTION validate_vm_status_transition()
RETURNS TRIGGER AS $$
BEGIN
    -- Allow all transitions during INSERT
    IF TG_OP = 'INSERT' THEN
        RETURN NEW;
    END IF;

    -- Validate status transitions during UPDATE
    IF OLD.status != NEW.status THEN
        CASE OLD.status
            WHEN 'pending' THEN
                IF NEW.status NOT IN ('stopped', 'starting', 'error') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'stopped' THEN
                IF NEW.status NOT IN ('starting', 'pending', 'error') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'starting' THEN
                IF NEW.status NOT IN ('running', 'stopped', 'error') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'running' THEN
                IF NEW.status NOT IN ('stopping', 'suspended', 'error') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'stopping' THEN
                IF NEW.status NOT IN ('stopped', 'error', 'running') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'suspended' THEN
                IF NEW.status NOT IN ('running', 'stopped', 'error') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
            WHEN 'error' THEN
                IF NEW.status NOT IN ('stopped', 'starting') THEN
                    RAISE EXCEPTION 'Invalid status transition from % to %', OLD.status, NEW.status;
                END IF;
        END CASE;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for status validation
CREATE TRIGGER validate_vm_status_transition_trigger
    BEFORE INSERT OR UPDATE ON virtual_machines
    FOR EACH ROW
    EXECUTE FUNCTION validate_vm_status_transition();

-- Utility functions for reporting
CREATE OR REPLACE FUNCTION get_vm_count_by_status()
RETURNS TABLE(status vm_status, count BIGINT) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        vm.status,
        COUNT(*) as count
    FROM virtual_machines vm 
    WHERE vm.deleted_at IS NULL
    GROUP BY vm.status
    ORDER BY vm.status;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_resource_utilization()
RETURNS TABLE(
    total_vms BIGINT,
    running_vms BIGINT,
    total_cpu_cores BIGINT,
    used_cpu_cores BIGINT,
    total_ram_mb BIGINT,
    used_ram_mb BIGINT,
    total_disk_gb BIGINT,
    used_disk_gb BIGINT,
    cpu_usage_percent DOUBLE PRECISION,
    ram_usage_percent DOUBLE PRECISION,
    disk_usage_percent DOUBLE PRECISION
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) as total_vms,
        COUNT(*) FILTER (WHERE vm.status = 'running') as running_vms,
        COALESCE(SUM(vm.cpu_cores), 0) as total_cpu_cores,
        COALESCE(SUM(CASE WHEN vm.status = 'running' THEN vm.cpu_cores ELSE 0 END), 0) as used_cpu_cores,
        COALESCE(SUM(vm.ram_mb), 0) as total_ram_mb,
        COALESCE(SUM(CASE WHEN vm.status = 'running' THEN vm.ram_mb ELSE 0 END), 0) as used_ram_mb,
        COALESCE(SUM(vm.disk_gb), 0) as total_disk_gb,
        COALESCE(SUM(CASE WHEN vm.status = 'running' THEN vm.disk_gb ELSE 0 END), 0) as used_disk_gb,
        CASE 
            WHEN SUM(vm.cpu_cores) > 0 THEN 
                (SUM(CASE WHEN vm.status = 'running' THEN vm.cpu_cores ELSE 0 END)::DOUBLE PRECISION / SUM(vm.cpu_cores)::DOUBLE PRECISION) * 100
            ELSE 0
        END as cpu_usage_percent,
        CASE 
            WHEN SUM(vm.ram_mb) > 0 THEN 
                (SUM(CASE WHEN vm.status = 'running' THEN vm.ram_mb ELSE 0 END)::DOUBLE PRECISION / SUM(vm.ram_mb)::DOUBLE PRECISION) * 100
            ELSE 0
        END as ram_usage_percent,
        CASE 
            WHEN SUM(vm.disk_gb) > 0 THEN 
                (SUM(CASE WHEN vm.status = 'running' THEN vm.disk_gb ELSE 0 END)::DOUBLE PRECISION / SUM(vm.disk_gb)::DOUBLE PRECISION) * 100
            ELSE 0
        END as disk_usage_percent
    FROM virtual_machines vm 
    WHERE vm.deleted_at IS NULL;
END;
$$ LANGUAGE plpgsql;

-- Comment on table and important columns
COMMENT ON TABLE virtual_machines IS 'Virtual machines managed by the VM Manager';
COMMENT ON COLUMN virtual_machines.id IS 'Unique identifier for the virtual machine';
COMMENT ON COLUMN virtual_machines.name IS 'Human-readable name for the virtual machine (must be unique)';
COMMENT ON COLUMN virtual_machines.status IS 'Current operational status of the virtual machine';
COMMENT ON COLUMN virtual_machines.node_id IS 'Identifier of the physical node hosting this VM';
COMMENT ON COLUMN virtual_machines.labels IS 'Key-value labels for categorization and selection';
COMMENT ON COLUMN virtual_machines.annotations IS 'Key-value annotations for metadata';
