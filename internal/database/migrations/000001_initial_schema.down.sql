-- Drop VM Manager schema

-- Drop triggers
DROP TRIGGER IF EXISTS validate_vm_status_transition_trigger ON virtual_machines;
DROP TRIGGER IF EXISTS update_virtual_machines_updated_at ON virtual_machines;

-- Drop functions
DROP FUNCTION IF EXISTS get_resource_utilization();
DROP FUNCTION IF EXISTS get_vm_count_by_status();
DROP FUNCTION IF EXISTS validate_vm_status_transition();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_virtual_machines_desc_trgm;
DROP INDEX IF EXISTS idx_virtual_machines_name_trgm;
DROP INDEX IF EXISTS idx_virtual_machines_search;
DROP INDEX IF EXISTS idx_virtual_machines_annotations;
DROP INDEX IF EXISTS idx_virtual_machines_labels;
DROP INDEX IF EXISTS idx_virtual_machines_deleted_at;
DROP INDEX IF EXISTS idx_virtual_machines_updated_at;
DROP INDEX IF EXISTS idx_virtual_machines_created_at;
DROP INDEX IF EXISTS idx_virtual_machines_created_by;
DROP INDEX IF EXISTS idx_virtual_machines_node_id;
DROP INDEX IF EXISTS idx_virtual_machines_status;
DROP INDEX IF EXISTS idx_virtual_machines_name;

-- Drop table
DROP TABLE IF EXISTS virtual_machines;

-- Drop types
DROP TYPE IF EXISTS network_type;
DROP TYPE IF EXISTS vm_status;

-- Drop extensions (be careful - other schemas might use them)
-- DROP EXTENSION IF EXISTS "pg_trgm";
-- DROP EXTENSION IF EXISTS "uuid-ossp";
