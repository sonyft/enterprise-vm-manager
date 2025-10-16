package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stackit/enterprise-vm-manager/internal/models"
	"github.com/stackit/enterprise-vm-manager/pkg/errors"
	"gorm.io/gorm"
)

// VMRepository interface defines VM data access operations
type VMRepository interface {
	Create(ctx context.Context, vm *models.VM) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.VM, error)
	GetByName(ctx context.Context, name string) (*models.VM, error)
	Update(ctx context.Context, vm *models.VM) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opts models.VMListOptions) ([]*models.VM, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.VMStatus) error
	UpdateStats(ctx context.Context, id uuid.UUID, stats models.VMStats) error
	GetResourceSummary(ctx context.Context) (*models.ResourceSummary, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	GetByNodeID(ctx context.Context, nodeID string) ([]*models.VM, error)
	CountByStatus(ctx context.Context, status models.VMStatus) (int64, error)
}

// vmRepository implements VMRepository interface
type vmRepository struct {
	db *gorm.DB
}

// NewVMRepository creates a new VM repository
func NewVMRepository(db *gorm.DB) VMRepository {
	return &vmRepository{db: db}
}

// Create creates a new VM
func (r *vmRepository) Create(ctx context.Context, vm *models.VM) error {
	if err := r.db.WithContext(ctx).Create(vm).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.AlreadyExistsError("VM", vm.Name)
		}
		return errors.DatabaseError("create VM", err)
	}
	return nil
}

// GetByID retrieves a VM by ID
func (r *vmRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.VM, error) {
	var vm models.VM
	if err := r.db.WithContext(ctx).First(&vm, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NotFoundError("VM", id.String())
		}
		return nil, errors.DatabaseError("get VM by ID", err)
	}
	return &vm, nil
}

// GetByName retrieves a VM by name
func (r *vmRepository) GetByName(ctx context.Context, name string) (*models.VM, error) {
	var vm models.VM
	if err := r.db.WithContext(ctx).First(&vm, "name = ?", name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NotFoundError("VM", name)
		}
		return nil, errors.DatabaseError("get VM by name", err)
	}
	return &vm, nil
}

// Update updates a VM
func (r *vmRepository) Update(ctx context.Context, vm *models.VM) error {
	if err := r.db.WithContext(ctx).Save(vm).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.AlreadyExistsError("VM", vm.Name)
		}
		return errors.DatabaseError("update VM", err)
	}
	return nil
}

// Delete deletes a VM (soft delete)
func (r *vmRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.VM{}, "id = ?", id)
	if result.Error != nil {
		return errors.DatabaseError("delete VM", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NotFoundError("VM", id.String())
	}
	return nil
}

// List retrieves VMs with pagination and filtering
func (r *vmRepository) List(ctx context.Context, opts models.VMListOptions) ([]*models.VM, int64, error) {
	var vms []*models.VM
	var total int64

	query := r.db.WithContext(ctx).Model(&models.VM{})

	// Apply filters
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	if opts.NodeID != "" {
		query = query.Where("node_id = ?", opts.NodeID)
	}

	if opts.CreatedBy != "" {
		query = query.Where("created_by = ?", opts.CreatedBy)
	}

	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.DatabaseError("count VMs", err)
	}

	// Apply sorting
	orderClause := fmt.Sprintf("%s %s", opts.SortBy, strings.ToUpper(opts.SortOrder))
	query = query.Order(orderClause)

	// Apply pagination
	offset := (opts.Page - 1) * opts.Limit
	query = query.Offset(offset).Limit(opts.Limit)

	// Execute query
	if err := query.Find(&vms).Error; err != nil {
		return nil, 0, errors.DatabaseError("list VMs", err)
	}

	return vms, total, nil
}

// UpdateStatus updates VM status
func (r *vmRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.VMStatus) error {
	result := r.db.WithContext(ctx).Model(&models.VM{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": "NOW()",
		})

	if result.Error != nil {
		return errors.DatabaseError("update VM status", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.NotFoundError("VM", id.String())
	}

	return nil
}

// UpdateStats updates VM statistics
func (r *vmRepository) UpdateStats(ctx context.Context, id uuid.UUID, stats models.VMStats) error {
	result := r.db.WithContext(ctx).Model(&models.VM{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"cpu_usage_percent":  stats.CPUUsagePercent,
			"ram_usage_percent":  stats.RAMUsagePercent,
			"disk_usage_percent": stats.DiskUsagePercent,
			"network_rx_bytes":   stats.NetworkRxBytes,
			"network_tx_bytes":   stats.NetworkTxBytes,
			"uptime_seconds":     stats.UptimeSeconds,
			"last_stats_update":  stats.LastStatsUpdate,
			"updated_at":         "NOW()",
		})

	if result.Error != nil {
		return errors.DatabaseError("update VM stats", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.NotFoundError("VM", id.String())
	}

	return nil
}

// GetResourceSummary gets overall resource usage summary
func (r *vmRepository) GetResourceSummary(ctx context.Context) (*models.ResourceSummary, error) {
	summary := &models.ResourceSummary{}

	// Get VM counts by status
	statusCounts := []struct {
		Status models.VMStatus `json:"status"`
		Count  int64           `json:"count"`
	}{}

	if err := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Find(&statusCounts).Error; err != nil {
		return nil, errors.DatabaseError("get VM status counts", err)
	}

	// Map status counts
	for _, sc := range statusCounts {
		switch sc.Status {
		case models.VMStatusRunning:
			summary.VMs.Running = sc.Count
		case models.VMStatusStopped:
			summary.VMs.Stopped = sc.Count
		case models.VMStatusError:
			summary.VMs.Error = sc.Count
		case models.VMStatusSuspended:
			summary.VMs.Suspended = sc.Count
		}
		summary.VMs.Total += sc.Count
	}

	// Get resource usage
	var resourceStats struct {
		TotalCPU   int `json:"total_cpu"`
		UsedCPU    int `json:"used_cpu"`
		TotalRAM   int `json:"total_ram"`
		UsedRAM    int `json:"used_ram"`
		TotalDisk  int `json:"total_disk"`
		UsedDisk   int `json:"used_disk"`
		RunningVMs int `json:"running_vms"`
	}

	if err := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Select(`
			COALESCE(SUM(cpu_cores), 0) as total_cpu,
			COALESCE(SUM(CASE WHEN status = 'running' THEN cpu_cores ELSE 0 END), 0) as used_cpu,
			COALESCE(SUM(ram_mb), 0) as total_ram,
			COALESCE(SUM(CASE WHEN status = 'running' THEN ram_mb ELSE 0 END), 0) as used_ram,
			COALESCE(SUM(disk_gb), 0) as total_disk,
			COALESCE(SUM(CASE WHEN status = 'running' THEN disk_gb ELSE 0 END), 0) as used_disk,
			COUNT(CASE WHEN status = 'running' THEN 1 END) as running_vms
		`).
		Scan(&resourceStats).Error; err != nil {
		return nil, errors.DatabaseError("get resource stats", err)
	}

	// Set CPU stats
	summary.Resources.CPU.Total = resourceStats.TotalCPU
	summary.Resources.CPU.Used = resourceStats.UsedCPU
	summary.Resources.CPU.Available = resourceStats.TotalCPU - resourceStats.UsedCPU
	if resourceStats.TotalCPU > 0 {
		summary.Resources.CPU.Usage = float64(resourceStats.UsedCPU) / float64(resourceStats.TotalCPU) * 100
	}

	// Set RAM stats
	summary.Resources.RAM.Total = resourceStats.TotalRAM
	summary.Resources.RAM.Used = resourceStats.UsedRAM
	summary.Resources.RAM.Available = resourceStats.TotalRAM - resourceStats.UsedRAM
	if resourceStats.TotalRAM > 0 {
		summary.Resources.RAM.Usage = float64(resourceStats.UsedRAM) / float64(resourceStats.TotalRAM) * 100
	}

	// Set Disk stats
	summary.Resources.Disk.Total = resourceStats.TotalDisk
	summary.Resources.Disk.Used = resourceStats.UsedDisk
	summary.Resources.Disk.Available = resourceStats.TotalDisk - resourceStats.UsedDisk
	if resourceStats.TotalDisk > 0 {
		summary.Resources.Disk.Usage = float64(resourceStats.UsedDisk) / float64(resourceStats.TotalDisk) * 100
	}

	// Get node counts (simplified - in real implementation this would query a nodes table)
	uniqueNodes := []string{}
	if err := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Distinct("node_id").
		Where("node_id IS NOT NULL AND node_id != ''").
		Pluck("node_id", &uniqueNodes).Error; err != nil {
		return nil, errors.DatabaseError("get node counts", err)
	}

	summary.Nodes.Total = len(uniqueNodes)
	summary.Nodes.Active = len(uniqueNodes) // Simplified

	return summary, nil
}

// ExistsByName checks if a VM with the given name exists
func (r *vmRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Where("name = ?", name).
		Count(&count).Error; err != nil {
		return false, errors.DatabaseError("check VM exists by name", err)
	}
	return count > 0, nil
}

// GetByNodeID retrieves all VMs on a specific node
func (r *vmRepository) GetByNodeID(ctx context.Context, nodeID string) ([]*models.VM, error) {
	var vms []*models.VM
	if err := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Find(&vms).Error; err != nil {
		return nil, errors.DatabaseError("get VMs by node ID", err)
	}
	return vms, nil
}

// CountByStatus counts VMs by status
func (r *vmRepository) CountByStatus(ctx context.Context, status models.VMStatus) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Where("status = ?", status).
		Count(&count).Error; err != nil {
		return 0, errors.DatabaseError("count VMs by status", err)
	}
	return count, nil
}
