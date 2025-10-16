package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stackit/enterprise-vm-manager/internal/api/middleware"
	"github.com/stackit/enterprise-vm-manager/internal/models"
	"github.com/stackit/enterprise-vm-manager/internal/services"
	"github.com/stackit/enterprise-vm-manager/pkg/errors"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
)

// VMHandler handles VM-related HTTP requests
type VMHandler struct {
	vmService services.VMService
	logger    *logger.Logger
}

// NewVMHandler creates a new VM handler
func NewVMHandler(vmService services.VMService, logger *logger.Logger) *VMHandler {
	return &VMHandler{
		vmService: vmService,
		logger:    logger.WithComponent("vm-handler"),
	}
}

// CreateVM creates a new virtual machine
// @Summary Create a new virtual machine
// @Description Create a new virtual machine with specified configuration
// @Tags VMs
// @Accept json
// @Produce json
// @Param request body models.VMCreateRequest true "VM creation request"
// @Success 201 {object} models.VM "VM created successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 409 {object} map[string]interface{} "VM already exists"
// @Failure 422 {object} map[string]interface{} "Resource limits exceeded"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms [post]
func (h *VMHandler) CreateVM(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("create-vm")

	var req models.VMCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf("Invalid request body: %v", err)
		appErr := errors.ErrValidationFailed.WithContext("request_id", requestID).WithDetails(err.Error())
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	// Set created_by from context
	if userID := middleware.GetUserID(c); userID != "" {
		req.CreatedBy = userID
	} else {
		req.CreatedBy = "system"
	}

	vm, err := h.vmService.CreateVM(c.Request.Context(), &req)
	if err != nil {
		log.Errorf("Failed to create VM: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	log.Infof("VM created successfully: %s", vm.Name)
	c.JSON(http.StatusCreated, gin.H{
		"data":       models.NewVMResponse(vm),
		"message":    "VM created successfully",
		"request_id": requestID,
	})
}

// GetVM retrieves a virtual machine by ID
// @Summary Get virtual machine by ID
// @Description Get detailed information about a specific virtual machine
// @Tags VMs
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Success 200 {object} models.VMResponse "VM details"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id} [get]
func (h *VMHandler) GetVM(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("get-vm")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		log.Warnf("Invalid VM ID format: %s", idParam)
		appErr := errors.ErrInvalidInput.WithContext("request_id", requestID).WithDetails("Invalid UUID format")
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	vm, err := h.vmService.GetVM(c.Request.Context(), id)
	if err != nil {
		log.Errorf("Failed to get VM: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       models.NewVMResponse(vm),
		"request_id": requestID,
	})
}

// ListVMs retrieves a list of virtual machines
// @Summary List virtual machines
// @Description Get a paginated list of virtual machines with optional filtering
// @Tags VMs
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param limit query int false "Items per page" default(20) minimum(1) maximum(100)
// @Param status query string false "Filter by status" Enums(pending,stopped,starting,running,stopping,suspended,error)
// @Param node_id query string false "Filter by node ID"
// @Param created_by query string false "Filter by creator"
// @Param search query string false "Search in name and description"
// @Param sort_by query string false "Sort field" default(created_at) Enums(created_at,updated_at,name,status)
// @Param sort_order query string false "Sort order" default(desc) Enums(asc,desc)
// @Param include_stats query bool false "Include runtime statistics" default(false)
// @Success 200 {object} models.VMListResponse "List of VMs"
// @Failure 400 {object} map[string]interface{} "Invalid query parameters"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms [get]
func (h *VMHandler) ListVMs(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("list-vms")

	var opts models.VMListOptions
	if err := c.ShouldBindQuery(&opts); err != nil {
		log.Warnf("Invalid query parameters: %v", err)
		appErr := errors.ErrValidationFailed.WithContext("request_id", requestID).WithDetails(err.Error())
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	response, err := h.vmService.ListVMs(c.Request.Context(), opts)
	if err != nil {
		log.Errorf("Failed to list VMs: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	log.Debugf("Listed %d VMs (total: %d)", len(response.VMs), response.Pagination.Total)
	c.JSON(http.StatusOK, gin.H{
		"data":       response,
		"request_id": requestID,
	})
}

// UpdateVM updates a virtual machine
// @Summary Update virtual machine
// @Description Update virtual machine configuration (only when stopped)
// @Tags VMs
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMUpdateRequest true "VM update request"
// @Success 200 {object} models.VMResponse "Updated VM"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be updated in current state"
// @Failure 422 {object} map[string]interface{} "Resource limits exceeded"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id} [put]
func (h *VMHandler) UpdateVM(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("update-vm")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		log.Warnf("Invalid VM ID format: %s", idParam)
		appErr := errors.ErrInvalidInput.WithContext("request_id", requestID).WithDetails("Invalid UUID format")
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	var req models.VMUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf("Invalid request body: %v", err)
		appErr := errors.ErrValidationFailed.WithContext("request_id", requestID).WithDetails(err.Error())
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	// Set updated_by from context
	if userID := middleware.GetUserID(c); userID != "" {
		req.UpdatedBy = userID
	} else {
		req.UpdatedBy = "system"
	}

	vm, err := h.vmService.UpdateVM(c.Request.Context(), id, &req)
	if err != nil {
		log.Errorf("Failed to update VM: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	log.Infof("VM updated successfully: %s", vm.Name)
	c.JSON(http.StatusOK, gin.H{
		"data":       models.NewVMResponse(vm),
		"message":    "VM updated successfully",
		"request_id": requestID,
	})
}

// DeleteVM deletes a virtual machine
// @Summary Delete virtual machine
// @Description Delete a virtual machine (only when stopped)
// @Tags VMs
// @Param id path string true "VM ID" format(uuid)
// @Success 204 "VM deleted successfully"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be deleted in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id} [delete]
func (h *VMHandler) DeleteVM(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("delete-vm")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		log.Warnf("Invalid VM ID format: %s", idParam)
		appErr := errors.ErrInvalidInput.WithContext("request_id", requestID).WithDetails("Invalid UUID format")
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	err = h.vmService.DeleteVM(c.Request.Context(), id)
	if err != nil {
		log.Errorf("Failed to delete VM: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	log.Infof("VM deleted successfully: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"message":    "VM deleted successfully",
		"request_id": requestID,
	})
}

// StartVM starts a virtual machine
// @Summary Start virtual machine
// @Description Start a stopped virtual machine
// @Tags VM Operations
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMStateChangeRequest false "State change options"
// @Success 202 {object} map[string]interface{} "VM start initiated"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be started in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/start [post]
func (h *VMHandler) StartVM(c *gin.Context) {
	h.changeVMState(c, "start", h.vmService.StartVM)
}

// StopVM stops a virtual machine
// @Summary Stop virtual machine
// @Description Stop a running virtual machine
// @Tags VM Operations
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMStateChangeRequest false "State change options"
// @Success 202 {object} map[string]interface{} "VM stop initiated"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be stopped in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/stop [post]
func (h *VMHandler) StopVM(c *gin.Context) {
	h.changeVMState(c, "stop", h.vmService.StopVM)
}

// RestartVM restarts a virtual machine
// @Summary Restart virtual machine
// @Description Restart a running virtual machine
// @Tags VM Operations
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMStateChangeRequest false "State change options"
// @Success 202 {object} map[string]interface{} "VM restart initiated"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be restarted in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/restart [post]
func (h *VMHandler) RestartVM(c *gin.Context) {
	h.changeVMState(c, "restart", h.vmService.RestartVM)
}

// SuspendVM suspends a virtual machine
// @Summary Suspend virtual machine
// @Description Suspend a running virtual machine
// @Tags VM Operations
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMStateChangeRequest false "State change options"
// @Success 202 {object} map[string]interface{} "VM suspend initiated"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be suspended in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/suspend [post]
func (h *VMHandler) SuspendVM(c *gin.Context) {
	h.changeVMState(c, "suspend", h.vmService.SuspendVM)
}

// ResumeVM resumes a suspended virtual machine
// @Summary Resume virtual machine
// @Description Resume a suspended virtual machine
// @Tags VM Operations
// @Accept json
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Param request body models.VMStateChangeRequest false "State change options"
// @Success 202 {object} map[string]interface{} "VM resume initiated"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 409 {object} map[string]interface{} "VM cannot be resumed in current state"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/resume [post]
func (h *VMHandler) ResumeVM(c *gin.Context) {
	h.changeVMState(c, "resume", h.vmService.ResumeVM)
}

// GetVMStats retrieves virtual machine statistics
// @Summary Get VM statistics
// @Description Get real-time statistics for a virtual machine
// @Tags VM Stats
// @Produce json
// @Param id path string true "VM ID" format(uuid)
// @Success 200 {object} models.VMStats "VM statistics"
// @Failure 400 {object} map[string]interface{} "Invalid VM ID"
// @Failure 404 {object} map[string]interface{} "VM not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/vms/{id}/stats [get]
func (h *VMHandler) GetVMStats(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("get-vm-stats")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		log.Warnf("Invalid VM ID format: %s", idParam)
		appErr := errors.ErrInvalidInput.WithContext("request_id", requestID).WithDetails("Invalid UUID format")
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	// Get VM to ensure it exists and get its stats
	vm, err := h.vmService.GetVM(c.Request.Context(), id)
	if err != nil {
		log.Errorf("Failed to get VM for stats: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	// Update stats if VM is running
	if vm.Status == models.VMStatusRunning {
		h.vmService.UpdateVMStats(c.Request.Context(), id)
		// Refresh VM data to get updated stats
		vm, _ = h.vmService.GetVM(c.Request.Context(), id)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       vm.Stats,
		"request_id": requestID,
	})
}

// GetResourceSummary gets overall resource usage summary
// @Summary Get resource summary
// @Description Get overall resource usage and VM statistics
// @Tags System
// @Produce json
// @Success 200 {object} models.ResourceSummary "Resource usage summary"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/stats/summary [get]
func (h *VMHandler) GetResourceSummary(c *gin.Context) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation("get-resource-summary")

	summary, err := h.vmService.GetResourceSummary(c.Request.Context())
	if err != nil {
		log.Errorf("Failed to get resource summary: %v", err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       summary,
		"request_id": requestID,
	})
}

// Helper method for state change operations
func (h *VMHandler) changeVMState(c *gin.Context, operation string, serviceFunc func(context.Context, uuid.UUID, *models.VMStateChangeRequest) error) {
	requestID := requestid.Get(c)
	log := h.logger.WithRequestID(requestID).WithOperation(operation + "-vm")

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		log.Warnf("Invalid VM ID format: %s", idParam)
		appErr := errors.ErrInvalidInput.WithContext("request_id", requestID).WithDetails("Invalid UUID format")
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	var req models.VMStateChangeRequest
	// Optional body - ignore binding errors
	c.ShouldBindJSON(&req)

	// Set updated_by from context
	if userID := middleware.GetUserID(c); userID != "" {
		req.UpdatedBy = userID
	} else {
		req.UpdatedBy = "system"
	}

	err = serviceFunc(c.Request.Context(), id, &req)
	if err != nil {
		log.Errorf("Failed to %s VM: %v", operation, err)
		appErr := errors.ToAppError(err).WithContext("request_id", requestID)
		c.JSON(appErr.HTTPCode, gin.H{
			"error":      appErr,
			"request_id": requestID,
		})
		return
	}

	log.Infof("VM %s operation initiated successfully: %s", operation, id)
	c.JSON(http.StatusAccepted, gin.H{
		"message":    fmt.Sprintf("VM %s operation initiated", operation),
		"request_id": requestID,
		"vm_id":      id,
	})
}
