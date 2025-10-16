package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/internal/models"
	"github.com/stackit/enterprise-vm-manager/internal/repositories"
	"github.com/stackit/enterprise-vm-manager/pkg/errors"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
)

// VMService interface defines VM business operations
type VMService interface {
	CreateVM(ctx context.Context, req *models.VMCreateRequest) (*models.VM, error)
	GetVM(ctx context.Context, id uuid.UUID) (*models.VM, error)
	GetVMByName(ctx context.Context, name string) (*models.VM, error)
	UpdateVM(ctx context.Context, id uuid.UUID, req *models.VMUpdateRequest) (*models.VM, error)
	DeleteVM(ctx context.Context, id uuid.UUID) error
	ListVMs(ctx context.Context, opts models.VMListOptions) (*models.VMListResponse, error)
	StartVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error
	StopVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error
	RestartVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error
	SuspendVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error
	ResumeVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error
	GetResourceSummary(ctx context.Context) (*models.ResourceSummary, error)
	UpdateVMStats(ctx context.Context, id uuid.UUID) error
}

// vmService implements VMService interface
type vmService struct {
	vmRepo repositories.VMRepository
	cfg    *config.Config
	logger *logger.Logger
}

// NewVMService creates a new VM service
func NewVMService(vmRepo repositories.VMRepository, cfg *config.Config, logger *logger.Logger) VMService {
	return &vmService{
		vmRepo: vmRepo,
		cfg:    cfg,
		logger: logger.WithComponent("vm-service"),
	}
}

// CreateVM creates a new virtual machine
func (s *vmService) CreateVM(ctx context.Context, req *models.VMCreateRequest) (*models.VM, error) {
	log := s.logger.WithOperation("create-vm")

	// Validate resource limits
	if err := s.validateResourceLimits(req.CPUCores, req.RAMMb, req.DiskGb); err != nil {
		log.Warnf("Resource validation failed: %v", err)
		return nil, err
	}

	// Check if VM name already exists
	exists, err := s.vmRepo.ExistsByName(ctx, req.Name)
	if err != nil {
		log.Errorf("Failed to check VM name existence: %v", err)
		return nil, err
	}
	if exists {
		return nil, errors.AlreadyExistsError("VM", req.Name)
	}

	// Create VM model
	vm := req.ToVM()
	vm.NodeID = s.assignNodeID()

	// Create VM in database
	if err := s.vmRepo.Create(ctx, vm); err != nil {
		log.Errorf("Failed to create VM: %v", err)
		return nil, err
	}

	log.Infof("VM created successfully: %s (ID: %s)", vm.Name, vm.ID)

	// Start async provisioning (simulate)
	go s.simulateProvisioning(vm.ID)

	return vm, nil
}

// GetVM retrieves a VM by ID
func (s *vmService) GetVM(ctx context.Context, id uuid.UUID) (*models.VM, error) {
	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.WithOperation("get-vm").Errorf("Failed to get VM %s: %v", id, err)
		return nil, err
	}
	return vm, nil
}

// GetVMByName retrieves a VM by name
func (s *vmService) GetVMByName(ctx context.Context, name string) (*models.VM, error) {
	vm, err := s.vmRepo.GetByName(ctx, name)
	if err != nil {
		s.logger.WithOperation("get-vm-by-name").Errorf("Failed to get VM %s: %v", name, err)
		return nil, err
	}
	return vm, nil
}

// UpdateVM updates a VM
func (s *vmService) UpdateVM(ctx context.Context, id uuid.UUID, req *models.VMUpdateRequest) (*models.VM, error) {
	log := s.logger.WithOperation("update-vm")

	// Get existing VM
	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check if VM can be updated
	if !vm.CanPerformOperation("update") {
		return nil, errors.VMStateError(id.String(), string(vm.Status), "stopped")
	}

	// Validate new resource limits if changed
	newCPU := req.CPUCores
	newRAM := req.RAMMb
	newDisk := req.DiskGb

	if newCPU == 0 {
		newCPU = vm.Spec.CPUCores
	}
	if newRAM == 0 {
		newRAM = vm.Spec.RAMMb
	}
	if newDisk == 0 {
		newDisk = vm.Spec.DiskGb
	}

	if err := s.validateResourceLimits(newCPU, newRAM, newDisk); err != nil {
		return nil, err
	}

	// Check name uniqueness if changed
	if req.Name != "" && req.Name != vm.Name {
		exists, err := s.vmRepo.ExistsByName(ctx, req.Name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.AlreadyExistsError("VM", req.Name)
		}
	}

	// Apply updates
	if err := req.ApplyToVM(vm); err != nil {
		log.Errorf("Failed to apply updates to VM: %v", err)
		return nil, errors.InternalError("Failed to apply updates", err)
	}

	// Update in database
	if err := s.vmRepo.Update(ctx, vm); err != nil {
		log.Errorf("Failed to update VM: %v", err)
		return nil, err
	}

	log.Infof("VM updated successfully: %s (ID: %s)", vm.Name, vm.ID)
	return vm, nil
}

// DeleteVM deletes a VM
func (s *vmService) DeleteVM(ctx context.Context, id uuid.UUID) error {
	log := s.logger.WithOperation("delete-vm")

	// Get VM to check status
	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Check if VM can be deleted
	if !vm.CanPerformOperation("delete") {
		return errors.VMStateError(id.String(), string(vm.Status), "stopped")
	}

	// Delete VM
	if err := s.vmRepo.Delete(ctx, id); err != nil {
		log.Errorf("Failed to delete VM: %v", err)
		return err
	}

	log.Infof("VM deleted successfully: %s (ID: %s)", vm.Name, vm.ID)
	return nil
}

// ListVMs lists VMs with pagination and filtering
func (s *vmService) ListVMs(ctx context.Context, opts models.VMListOptions) (*models.VMListResponse, error) {
	vms, total, err := s.vmRepo.List(ctx, opts)
	if err != nil {
		s.logger.WithOperation("list-vms").Errorf("Failed to list VMs: %v", err)
		return nil, err
	}

	// Convert to response format
	vmResponses := make([]*models.VMResponse, len(vms))
	for i, vm := range vms {
		vmResponses[i] = models.NewVMResponse(vm)

		// Update stats if requested
		if opts.IncludeStats && vm.Status == models.VMStatusRunning {
			s.UpdateVMStats(ctx, vm.ID)
		}
	}

	// Calculate pagination
	totalPages := (total + int64(opts.Limit) - 1) / int64(opts.Limit)

	pagination := models.Pagination{
		Page:       opts.Page,
		Limit:      opts.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    int64(opts.Page) < totalPages,
		HasPrev:    opts.Page > 1,
	}

	return &models.VMListResponse{
		VMs:        vmResponses,
		Pagination: pagination,
	}, nil
}

// StartVM starts a VM
func (s *vmService) StartVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error {
	return s.changeVMState(ctx, id, models.VMStatusStarting, req, "start")
}

// StopVM stops a VM
func (s *vmService) StopVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error {
	if req.Force {
		return s.changeVMState(ctx, id, models.VMStatusStopped, req, "force-stop")
	}
	return s.changeVMState(ctx, id, models.VMStatusStopping, req, "stop")
}

// RestartVM restarts a VM
func (s *vmService) RestartVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error {
	log := s.logger.WithOperation("restart-vm")

	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !vm.CanPerformOperation("restart") {
		return errors.VMStateError(id.String(), string(vm.Status), "running")
	}

	// First stop, then start
	if err := s.changeVMState(ctx, id, models.VMStatusStopping, req, "restart-stop"); err != nil {
		return err
	}

	log.Infof("VM restart initiated: %s (ID: %s)", vm.Name, vm.ID)

	// Start async restart process
	go s.simulateRestart(id)

	return nil
}

// SuspendVM suspends a VM
func (s *vmService) SuspendVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error {
	return s.changeVMState(ctx, id, models.VMStatusSuspended, req, "suspend")
}

// ResumeVM resumes a suspended VM
func (s *vmService) ResumeVM(ctx context.Context, id uuid.UUID, req *models.VMStateChangeRequest) error {
	return s.changeVMState(ctx, id, models.VMStatusRunning, req, "resume")
}

// GetResourceSummary gets resource usage summary
func (s *vmService) GetResourceSummary(ctx context.Context) (*models.ResourceSummary, error) {
	summary, err := s.vmRepo.GetResourceSummary(ctx)
	if err != nil {
		s.logger.WithOperation("get-resource-summary").Errorf("Failed to get resource summary: %v", err)
		return nil, err
	}

	summary.GeneratedAt = time.Now()
	return summary, nil
}

// UpdateVMStats updates VM statistics
func (s *vmService) UpdateVMStats(ctx context.Context, id uuid.UUID) error {
	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if vm.Status != models.VMStatusRunning {
		return nil // Only update stats for running VMs
	}

	// Simulate realistic statistics
	stats := models.VMStats{
		CPUUsagePercent:  10 + rand.Float64()*80,                           // 10-90%
		RAMUsagePercent:  20 + rand.Float64()*70,                           // 20-90%
		DiskUsagePercent: 10 + rand.Float64()*50,                           // 10-60%
		NetworkRxBytes:   vm.Stats.NetworkRxBytes + rand.Int63n(1024*1024), // Add random traffic
		NetworkTxBytes:   vm.Stats.NetworkTxBytes + rand.Int63n(512*1024),
		UptimeSeconds:    vm.GetUptime(),
		LastStatsUpdate:  time.Now(),
	}

	return s.vmRepo.UpdateStats(ctx, id, stats)
}

// Helper methods

// validateResourceLimits validates resource limits against configuration
func (s *vmService) validateResourceLimits(cpu, ram, disk int) error {
	if cpu > s.cfg.Limits.MaxCPUCores {
		return errors.ResourceLimitError("CPU cores", cpu, s.cfg.Limits.MaxCPUCores)
	}

	if ram > s.cfg.Limits.MaxRAMMB {
		return errors.ResourceLimitError("RAM MB", ram, s.cfg.Limits.MaxRAMMB)
	}

	if disk > s.cfg.Limits.MaxDiskGB {
		return errors.ResourceLimitError("Disk GB", disk, s.cfg.Limits.MaxDiskGB)
	}

	return nil
}

// assignNodeID assigns a node ID (simplified implementation)
func (s *vmService) assignNodeID() string {
	nodeCount := 10 // In real implementation, this would query available nodes
	nodeID := rand.Intn(nodeCount) + 1
	return fmt.Sprintf("node-%02d", nodeID)
}

// changeVMState changes VM state with validation
func (s *vmService) changeVMState(ctx context.Context, id uuid.UUID, newStatus models.VMStatus, req *models.VMStateChangeRequest, operation string) error {
	log := s.logger.WithOperation(operation)

	vm, err := s.vmRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Check if operation is allowed
	if !vm.CanPerformOperation(operation) {
		return errors.VMStateError(id.String(), string(vm.Status), string(newStatus))
	}

	// Validate state transition
	if !vm.IsValidStatusTransition(newStatus) {
		return errors.VMStateError(id.String(), string(vm.Status), string(newStatus))
	}

	// Update status
	if err := s.vmRepo.UpdateStatus(ctx, id, newStatus); err != nil {
		log.Errorf("Failed to update VM status: %v", err)
		return err
	}

	log.Infof("VM %s operation initiated: %s (ID: %s)", operation, vm.Name, vm.ID)

	// Start async state transition simulation
	if newStatus == models.VMStatusStarting {
		go s.simulateStartup(id)
	} else if newStatus == models.VMStatusStopping {
		go s.simulateShutdown(id)
	}

	return nil
}

// Simulation methods (in real implementation, these would interact with hypervisor)

func (s *vmService) simulateProvisioning(vmID uuid.UUID) {
	time.Sleep(2 * time.Second) // Simulate provisioning time

	ctx := context.Background()
	if err := s.vmRepo.UpdateStatus(ctx, vmID, models.VMStatusStopped); err != nil {
		s.logger.Errorf("Failed to update VM status after provisioning: %v", err)
	}
}

func (s *vmService) simulateStartup(vmID uuid.UUID) {
	time.Sleep(3 * time.Second) // Simulate startup time

	ctx := context.Background()
	if err := s.vmRepo.UpdateStatus(ctx, vmID, models.VMStatusRunning); err != nil {
		s.logger.Errorf("Failed to update VM status after startup: %v", err)
		return
	}

	// Start stats updates for running VM
	go s.startStatsUpdater(vmID)
}

func (s *vmService) simulateShutdown(vmID uuid.UUID) {
	time.Sleep(2 * time.Second) // Simulate shutdown time

	ctx := context.Background()
	if err := s.vmRepo.UpdateStatus(ctx, vmID, models.VMStatusStopped); err != nil {
		s.logger.Errorf("Failed to update VM status after shutdown: %v", err)
	}
}

func (s *vmService) simulateRestart(vmID uuid.UUID) {
	time.Sleep(2 * time.Second) // Shutdown time

	ctx := context.Background()
	if err := s.vmRepo.UpdateStatus(ctx, vmID, models.VMStatusStarting); err != nil {
		s.logger.Errorf("Failed to update VM status during restart: %v", err)
		return
	}

	time.Sleep(3 * time.Second) // Startup time

	if err := s.vmRepo.UpdateStatus(ctx, vmID, models.VMStatusRunning); err != nil {
		s.logger.Errorf("Failed to update VM status after restart: %v", err)
		return
	}

	// Start stats updates
	go s.startStatsUpdater(vmID)
}

func (s *vmService) startStatsUpdater(vmID uuid.UUID) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := context.Background()

	for range ticker.C {
		vm, err := s.vmRepo.GetByID(ctx, vmID)
		if err != nil || vm.Status != models.VMStatusRunning {
			return // Stop updating if VM is not running
		}

		if err := s.UpdateVMStats(ctx, vmID); err != nil {
			s.logger.Errorf("Failed to update VM stats: %v", err)
		}
	}
}
