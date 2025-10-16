package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VMStatus represents the status of a virtual machine
type VMStatus string

const (
	VMStatusPending   VMStatus = "pending"
	VMStatusStopped   VMStatus = "stopped"
	VMStatusStarting  VMStatus = "starting"
	VMStatusRunning   VMStatus = "running"
	VMStatusStopping  VMStatus = "stopping"
	VMStatusSuspended VMStatus = "suspended"
	VMStatusError     VMStatus = "error"
)

// NetworkType represents the network configuration type
type NetworkType string

const (
	NetworkTypeNAT    NetworkType = "nat"
	NetworkTypeBridge NetworkType = "bridge"
	NetworkTypeHost   NetworkType = "host"
)

// VM represents a virtual machine entity
type VM struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null;size:255"`
	Description string    `json:"description" gorm:"size:1000"`

	// Resource specifications
	Spec VMSpec `json:"spec" gorm:"embedded"`

	// Current state
	Status     VMStatus `json:"status" gorm:"type:varchar(20);default:'stopped';index"`
	PowerState string   `json:"power_state" gorm:"type:varchar(10);default:'off'"`

	// Metadata
	Labels      json.RawMessage `json:"labels,omitempty" gorm:"type:jsonb"`
	Annotations json.RawMessage `json:"annotations,omitempty" gorm:"type:jsonb"`

	// Resource allocation
	NodeID string `json:"node_id" gorm:"size:255;index"`

	// Statistics
	Stats VMStats `json:"stats" gorm:"embedded"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	StoppedAt *time.Time     `json:"stopped_at,omitempty"`

	// Audit fields
	CreatedBy string `json:"created_by" gorm:"size:255"`
	UpdatedBy string `json:"updated_by" gorm:"size:255"`
}

// VMSpec represents the specification of a virtual machine
type VMSpec struct {
	CPUCores    int         `json:"cpu_cores" gorm:"not null;check:cpu_cores > 0 AND cpu_cores <= 64"`
	RAMMb       int         `json:"ram_mb" gorm:"not null;check:ram_mb >= 512 AND ram_mb <= 524288"`
	DiskGb      int         `json:"disk_gb" gorm:"not null;check:disk_gb >= 10 AND disk_gb <= 10240"`
	ImageName   string      `json:"image_name" gorm:"not null;size:255"`
	NetworkType NetworkType `json:"network_type" gorm:"type:varchar(20);default:'nat'"`
	BootOrder   string      `json:"boot_order" gorm:"size:50;default:'hd'"`
}

// VMStats represents runtime statistics of a virtual machine
type VMStats struct {
	CPUUsagePercent  float64   `json:"cpu_usage_percent" gorm:"default:0"`
	RAMUsagePercent  float64   `json:"ram_usage_percent" gorm:"default:0"`
	DiskUsagePercent float64   `json:"disk_usage_percent" gorm:"default:0"`
	NetworkRxBytes   int64     `json:"network_rx_bytes" gorm:"default:0"`
	NetworkTxBytes   int64     `json:"network_tx_bytes" gorm:"default:0"`
	UptimeSeconds    int64     `json:"uptime_seconds" gorm:"default:0"`
	LastStatsUpdate  time.Time `json:"last_stats_update"`
}

// BeforeCreate hook
func (vm *VM) BeforeCreate(tx *gorm.DB) error {
	if vm.ID == uuid.Nil {
		vm.ID = uuid.New()
	}
	vm.CreatedAt = time.Now()
	vm.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate hook
func (vm *VM) BeforeUpdate(tx *gorm.DB) error {
	vm.UpdatedAt = time.Now()
	return nil
}

// TableName returns the table name for VM
func (VM) TableName() string {
	return "virtual_machines"
}

// IsValidStatusTransition checks if status transition is valid
func (vm *VM) IsValidStatusTransition(newStatus VMStatus) bool {
	validTransitions := map[VMStatus][]VMStatus{
		VMStatusPending:   {VMStatusStopped, VMStatusStarting, VMStatusError},
		VMStatusStopped:   {VMStatusStarting, VMStatusPending, VMStatusError},
		VMStatusStarting:  {VMStatusRunning, VMStatusStopped, VMStatusError},
		VMStatusRunning:   {VMStatusStopping, VMStatusSuspended, VMStatusError},
		VMStatusStopping:  {VMStatusStopped, VMStatusError, VMStatusRunning},
		VMStatusSuspended: {VMStatusRunning, VMStatusStopped, VMStatusError},
		VMStatusError:     {VMStatusStopped, VMStatusStarting},
	}

	validNext, exists := validTransitions[vm.Status]
	if !exists {
		return false
	}

	for _, status := range validNext {
		if status == newStatus {
			return true
		}
	}
	return false
}

// CanPerformOperation checks if an operation can be performed
func (vm *VM) CanPerformOperation(operation string) bool {
	switch operation {
	case "start":
		return vm.Status == VMStatusStopped
	case "stop":
		return vm.Status == VMStatusRunning || vm.Status == VMStatusStarting
	case "restart":
		return vm.Status == VMStatusRunning
	case "suspend":
		return vm.Status == VMStatusRunning
	case "resume":
		return vm.Status == VMStatusSuspended
	case "update":
		return vm.Status == VMStatusStopped
	case "delete":
		return vm.Status == VMStatusStopped
	default:
		return false
	}
}

// GetUptime returns uptime in seconds if VM is running
func (vm *VM) GetUptime() int64 {
	if vm.Status == VMStatusRunning && vm.StartedAt != nil {
		return int64(time.Since(*vm.StartedAt).Seconds())
	}
	return 0
}

// AddLabel adds a label to the VM
func (vm *VM) AddLabel(key, value string) error {
	labels := make(map[string]string)
	if vm.Labels != nil {
		if err := json.Unmarshal(vm.Labels, &labels); err != nil {
			return err
		}
	}
	labels[key] = value

	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	vm.Labels = labelsJSON
	return nil
}

// GetLabel gets a label value
func (vm *VM) GetLabel(key string) (string, bool) {
	if vm.Labels == nil {
		return "", false
	}

	labels := make(map[string]string)
	if err := json.Unmarshal(vm.Labels, &labels); err != nil {
		return "", false
	}

	value, exists := labels[key]
	return value, exists
}

// AddAnnotation adds an annotation to the VM
func (vm *VM) AddAnnotation(key, value string) error {
	annotations := make(map[string]string)
	if vm.Annotations != nil {
		if err := json.Unmarshal(vm.Annotations, &annotations); err != nil {
			return err
		}
	}
	annotations[key] = value

	annotationsJSON, err := json.Marshal(annotations)
	if err != nil {
		return err
	}
	vm.Annotations = annotationsJSON
	return nil
}

// UpdateStats updates VM statistics
func (vm *VM) UpdateStats(cpuUsage, ramUsage, diskUsage float64, networkRx, networkTx int64) {
	vm.Stats.CPUUsagePercent = cpuUsage
	vm.Stats.RAMUsagePercent = ramUsage
	vm.Stats.DiskUsagePercent = diskUsage
	vm.Stats.NetworkRxBytes = networkRx
	vm.Stats.NetworkTxBytes = networkTx
	vm.Stats.UptimeSeconds = vm.GetUptime()
	vm.Stats.LastStatsUpdate = time.Now()
}

// VMCreateRequest represents a request to create a VM
type VMCreateRequest struct {
	Name        string            `json:"name" binding:"required,min=3,max=63" example:"web-server-01"`
	Description string            `json:"description" binding:"max=1000" example:"Production web server"`
	CPUCores    int               `json:"cpu_cores" binding:"required,min=1,max=64" example:"4"`
	RAMMb       int               `json:"ram_mb" binding:"required,min=512,max=524288" example:"8192"`
	DiskGb      int               `json:"disk_gb" binding:"required,min=10,max=10240" example:"100"`
	ImageName   string            `json:"image_name" binding:"required" example:"ubuntu:22.04"`
	NetworkType NetworkType       `json:"network_type" binding:"omitempty,oneof=nat bridge host" example:"nat"`
	Labels      map[string]string `json:"labels,omitempty" example:"environment:production,tier:web"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedBy   string            `json:"created_by" binding:"required" example:"user123"`
}

// ToVM converts create request to VM model
func (req *VMCreateRequest) ToVM() *VM {
	vm := &VM{
		Name:        req.Name,
		Description: req.Description,
		Spec: VMSpec{
			CPUCores:    req.CPUCores,
			RAMMb:       req.RAMMb,
			DiskGb:      req.DiskGb,
			ImageName:   req.ImageName,
			NetworkType: req.NetworkType,
		},
		Status:    VMStatusPending,
		CreatedBy: req.CreatedBy,
		UpdatedBy: req.CreatedBy,
	}

	if req.Labels != nil {
		if labelsJSON, err := json.Marshal(req.Labels); err == nil {
			vm.Labels = labelsJSON
		}
	}

	if req.Annotations != nil {
		if annotationsJSON, err := json.Marshal(req.Annotations); err == nil {
			vm.Annotations = annotationsJSON
		}
	}

	return vm
}

// VMUpdateRequest represents a request to update a VM
type VMUpdateRequest struct {
	Name        string            `json:"name,omitempty" binding:"omitempty,min=3,max=63"`
	Description string            `json:"description,omitempty" binding:"omitempty,max=1000"`
	CPUCores    int               `json:"cpu_cores,omitempty" binding:"omitempty,min=1,max=64"`
	RAMMb       int               `json:"ram_mb,omitempty" binding:"omitempty,min=512,max=524288"`
	DiskGb      int               `json:"disk_gb,omitempty" binding:"omitempty,min=10,max=10240"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	UpdatedBy   string            `json:"updated_by,omitempty"`
}

// ApplyToVM applies update request to VM model
func (req *VMUpdateRequest) ApplyToVM(vm *VM) error {
	if req.Name != "" {
		vm.Name = req.Name
	}
	if req.Description != "" {
		vm.Description = req.Description
	}
	if req.CPUCores > 0 {
		vm.Spec.CPUCores = req.CPUCores
	}
	if req.RAMMb > 0 {
		vm.Spec.RAMMb = req.RAMMb
	}
	if req.DiskGb > 0 {
		vm.Spec.DiskGb = req.DiskGb
	}
	if req.UpdatedBy != "" {
		vm.UpdatedBy = req.UpdatedBy
	}

	if req.Labels != nil {
		if labelsJSON, err := json.Marshal(req.Labels); err != nil {
			return err
		} else {
			vm.Labels = labelsJSON
		}
	}

	if req.Annotations != nil {
		if annotationsJSON, err := json.Marshal(req.Annotations); err != nil {
			return err
		} else {
			vm.Annotations = annotationsJSON
		}
	}

	return nil
}

// VMStateChangeRequest represents a request to change VM state
type VMStateChangeRequest struct {
	Force     bool   `json:"force,omitempty" example:"false"`
	Reason    string `json:"reason,omitempty" example:"Scheduled maintenance"`
	UpdatedBy string `json:"updated_by,omitempty" example:"user123"`
}

// VMListOptions represents options for listing VMs
type VMListOptions struct {
	Page         int      `form:"page,default=1" binding:"min=1"`
	Limit        int      `form:"limit,default=20" binding:"min=1,max=100"`
	Status       VMStatus `form:"status" binding:"omitempty,oneof=pending stopped starting running stopping suspended error"`
	NodeID       string   `form:"node_id"`
	CreatedBy    string   `form:"created_by"`
	Search       string   `form:"search"`
	SortBy       string   `form:"sort_by,default=created_at" binding:"omitempty,oneof=created_at updated_at name status"`
	SortOrder    string   `form:"sort_order,default=desc" binding:"omitempty,oneof=asc desc"`
	IncludeStats bool     `form:"include_stats,default=false"`
}

// VMResponse represents VM response data
type VMResponse struct {
	*VM
	Uptime int64 `json:"uptime_seconds"`
}

// NewVMResponse creates a VM response
func NewVMResponse(vm *VM) *VMResponse {
	return &VMResponse{
		VM:     vm,
		Uptime: vm.GetUptime(),
	}
}

// VMListResponse represents paginated VM list response
type VMListResponse struct {
	VMs        []*VMResponse `json:"vms"`
	Pagination Pagination    `json:"pagination"`
}

// Pagination represents pagination information
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// ResourceSummary represents overall resource usage
type ResourceSummary struct {
	VMs struct {
		Total     int64 `json:"total"`
		Running   int64 `json:"running"`
		Stopped   int64 `json:"stopped"`
		Error     int64 `json:"error"`
		Suspended int64 `json:"suspended"`
	} `json:"vms"`
	Resources struct {
		CPU struct {
			Total     int     `json:"total"`
			Used      int     `json:"used"`
			Available int     `json:"available"`
			Usage     float64 `json:"usage_percent"`
		} `json:"cpu"`
		RAM struct {
			Total     int     `json:"total_mb"`
			Used      int     `json:"used_mb"`
			Available int     `json:"available_mb"`
			Usage     float64 `json:"usage_percent"`
		} `json:"ram"`
		Disk struct {
			Total     int     `json:"total_gb"`
			Used      int     `json:"used_gb"`
			Available int     `json:"available_gb"`
			Usage     float64 `json:"usage_percent"`
		} `json:"disk"`
	} `json:"resources"`
	Nodes struct {
		Total  int `json:"total"`
		Active int `json:"active"`
	} `json:"nodes"`
	GeneratedAt time.Time `json:"generated_at"`
}
