package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stackit/enterprise-vm-manager/internal/api/handlers"
	"github.com/stackit/enterprise-vm-manager/internal/api/middleware"
	"github.com/stackit/enterprise-vm-manager/internal/api/routes"
	"github.com/stackit/enterprise-vm-manager/internal/config"
	"github.com/stackit/enterprise-vm-manager/internal/models"
	"github.com/stackit/enterprise-vm-manager/internal/repositories"
	"github.com/stackit/enterprise-vm-manager/internal/services"
	"github.com/stackit/enterprise-vm-manager/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// VMHandlerTestSuite defines the test suite
type VMHandlerTestSuite struct {
	suite.Suite
	db        *gorm.DB
	router    *gin.Engine
	vmRepo    repositories.VMRepository
	vmService services.VMService
	vmHandler *handlers.VMHandler
	logger    *logger.Logger
	cfg       *config.Config
}

// SetupSuite runs once before the test suite
func (suite *VMHandlerTestSuite) SetupSuite() {
	// Initialize logger
	logConfig := logger.Config{
		Level:  "error", // Suppress logs in tests
		Format: "console",
		Output: "stdout",
	}

	var err error
	suite.logger, err = logger.New(logConfig)
	suite.Require().NoError(err)

	// Create test configuration
	suite.cfg = &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
			Mode: "test",
		},
		Limits: config.LimitsConfig{
			MaxCPUCores: 32,
			MaxRAMMB:    65536,
			MaxDiskGB:   5120,
			MaxVMs:      100,
		},
	}
}

// SetupTest runs before each test
func (suite *VMHandlerTestSuite) SetupTest() {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	suite.Require().NoError(err)
	suite.db = db

	// Auto migrate
	err = db.AutoMigrate(&models.VM{})
	suite.Require().NoError(err)

	// Initialize components
	suite.vmRepo = repositories.NewVMRepository(suite.db)
	suite.vmService = services.NewVMService(suite.vmRepo, suite.cfg, suite.logger)
	suite.vmHandler = handlers.NewVMHandler(suite.vmService, suite.logger)

	// Setup router
	gin.SetMode(gin.TestMode)
	suite.router = gin.New()
	middlewareManager := middleware.NewMiddlewareManager(suite.cfg, suite.logger)
	router := routes.NewRouter(suite.cfg, suite.logger, suite.vmHandler, middlewareManager)
	router.SetupRoutes(suite.router)
}

// TearDownTest runs after each test
func (suite *VMHandlerTestSuite) TearDownTest() {
	sqlDB, _ := suite.db.DB()
	sqlDB.Close()
}

// Helper methods

func (suite *VMHandlerTestSuite) createTestVM() *models.VM {
	vm := &models.VM{
		ID:          uuid.New(),
		Name:        "test-vm",
		Description: "Test virtual machine",
		Spec: models.VMSpec{
			CPUCores:    2,
			RAMMb:       2048,
			DiskGb:      50,
			ImageName:   "ubuntu:22.04",
			NetworkType: models.NetworkTypeNAT,
		},
		Status:    models.VMStatusStopped,
		NodeID:    "node-01",
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}

	err := suite.vmRepo.Create(context.Background(), vm)
	suite.Require().NoError(err)

	return vm
}

func (suite *VMHandlerTestSuite) makeRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(jsonBody)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	return w
}

// Test cases

func (suite *VMHandlerTestSuite) TestCreateVM_Success() {
	request := models.VMCreateRequest{
		Name:        "new-vm",
		Description: "New virtual machine",
		CPUCores:    4,
		RAMMb:       8192,
		DiskGb:      100,
		ImageName:   "ubuntu:22.04",
		NetworkType: models.NetworkTypeNAT,
		CreatedBy:   "test-user",
	}

	w := suite.makeRequest("POST", "/api/v1/vms", request)

	assert.Equal(suite.T(), http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "VM created successfully", response["message"])
	assert.NotNil(suite.T(), response["data"])
}

func (suite *VMHandlerTestSuite) TestCreateVM_ValidationError() {
	request := models.VMCreateRequest{
		Name:      "vm", // Too short
		CPUCores:  0,    // Invalid
		RAMMb:     100,  // Too small
		DiskGb:    1,    // Too small
		ImageName: "",   // Empty
		CreatedBy: "test-user",
	}

	w := suite.makeRequest("POST", "/api/v1/vms", request)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), response["error"])
}

func (suite *VMHandlerTestSuite) TestCreateVM_DuplicateName() {
	// Create first VM
	suite.createTestVM()

	request := models.VMCreateRequest{
		Name:        "test-vm", // Same name
		Description: "Duplicate VM",
		CPUCores:    2,
		RAMMb:       2048,
		DiskGb:      50,
		ImageName:   "ubuntu:22.04",
		CreatedBy:   "test-user",
	}

	w := suite.makeRequest("POST", "/api/v1/vms", request)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

func (suite *VMHandlerTestSuite) TestGetVM_Success() {
	vm := suite.createTestVM()

	w := suite.makeRequest("GET", "/api/v1/vms/"+vm.ID.String(), nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)

	data := response["data"].(map[string]interface{})
	assert.Equal(suite.T(), vm.Name, data["name"])
	assert.Equal(suite.T(), vm.ID.String(), data["id"])
}

func (suite *VMHandlerTestSuite) TestGetVM_NotFound() {
	randomID := uuid.New()

	w := suite.makeRequest("GET", "/api/v1/vms/"+randomID.String(), nil)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *VMHandlerTestSuite) TestGetVM_InvalidUUID() {
	w := suite.makeRequest("GET", "/api/v1/vms/invalid-uuid", nil)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *VMHandlerTestSuite) TestListVMs_Success() {
	// Create multiple VMs
	suite.createTestVM()

	vm2 := &models.VM{
		Name:        "test-vm-2",
		Description: "Second test VM",
		Spec: models.VMSpec{
			CPUCores:    4,
			RAMMb:       4096,
			DiskGb:      100,
			ImageName:   "centos:8",
			NetworkType: models.NetworkTypeBridge,
		},
		Status:    models.VMStatusRunning,
		NodeID:    "node-02",
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}
	suite.vmRepo.Create(context.Background(), vm2)

	w := suite.makeRequest("GET", "/api/v1/vms", nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)

	data := response["data"].(map[string]interface{})
	vms := data["vms"].([]interface{})
	pagination := data["pagination"].(map[string]interface{})

	assert.Equal(suite.T(), 2, len(vms))
	assert.Equal(suite.T(), float64(2), pagination["total"])
}

func (suite *VMHandlerTestSuite) TestListVMs_WithFilters() {
	vm := suite.createTestVM()

	w := suite.makeRequest("GET", "/api/v1/vms?status=stopped&limit=5", nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)

	data := response["data"].(map[string]interface{})
	vms := data["vms"].([]interface{})

	assert.Equal(suite.T(), 1, len(vms))
	vmData := vms[0].(map[string]interface{})
	assert.Equal(suite.T(), vm.Name, vmData["name"])
	assert.Equal(suite.T(), "stopped", vmData["status"])
}

func (suite *VMHandlerTestSuite) TestUpdateVM_Success() {
	vm := suite.createTestVM()

	request := models.VMUpdateRequest{
		Name:        "updated-vm",
		Description: "Updated description",
		CPUCores:    4,
		RAMMb:       4096,
		UpdatedBy:   "test-user",
	}

	w := suite.makeRequest("PUT", "/api/v1/vms/"+vm.ID.String(), request)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "VM updated successfully", response["message"])

	data := response["data"].(map[string]interface{})
	assert.Equal(suite.T(), "updated-vm", data["name"])
	assert.Equal(suite.T(), "Updated description", data["description"])
}

func (suite *VMHandlerTestSuite) TestUpdateVM_RunningVM() {
	vm := suite.createTestVM()

	// Set VM to running state
	suite.vmRepo.UpdateStatus(context.Background(), vm.ID, models.VMStatusRunning)

	request := models.VMUpdateRequest{
		Name:      "updated-vm",
		UpdatedBy: "test-user",
	}

	w := suite.makeRequest("PUT", "/api/v1/vms/"+vm.ID.String(), request)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

func (suite *VMHandlerTestSuite) TestDeleteVM_Success() {
	vm := suite.createTestVM()

	w := suite.makeRequest("DELETE", "/api/v1/vms/"+vm.ID.String(), nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "VM deleted successfully", response["message"])

	// Verify VM is deleted
	_, err = suite.vmRepo.GetByID(context.Background(), vm.ID)
	assert.Error(suite.T(), err)
}

func (suite *VMHandlerTestSuite) TestDeleteVM_RunningVM() {
	vm := suite.createTestVM()

	// Set VM to running state
	suite.vmRepo.UpdateStatus(context.Background(), vm.ID, models.VMStatusRunning)

	w := suite.makeRequest("DELETE", "/api/v1/vms/"+vm.ID.String(), nil)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

func (suite *VMHandlerTestSuite) TestStartVM_Success() {
	vm := suite.createTestVM()

	w := suite.makeRequest("POST", "/api/v1/vms/"+vm.ID.String()+"/start", nil)

	assert.Equal(suite.T(), http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response["message"], "start operation initiated")
}

func (suite *VMHandlerTestSuite) TestStartVM_AlreadyRunning() {
	vm := suite.createTestVM()

	// Set VM to running state
	suite.vmRepo.UpdateStatus(context.Background(), vm.ID, models.VMStatusRunning)

	w := suite.makeRequest("POST", "/api/v1/vms/"+vm.ID.String()+"/start", nil)

	assert.Equal(suite.T(), http.StatusConflict, w.Code)
}

func (suite *VMHandlerTestSuite) TestStopVM_Success() {
	vm := suite.createTestVM()

	// Set VM to running state
	suite.vmRepo.UpdateStatus(context.Background(), vm.ID, models.VMStatusRunning)

	w := suite.makeRequest("POST", "/api/v1/vms/"+vm.ID.String()+"/stop", nil)

	assert.Equal(suite.T(), http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response["message"], "stop operation initiated")
}

func (suite *VMHandlerTestSuite) TestRestartVM_Success() {
	vm := suite.createTestVM()

	// Set VM to running state
	suite.vmRepo.UpdateStatus(context.Background(), vm.ID, models.VMStatusRunning)

	w := suite.makeRequest("POST", "/api/v1/vms/"+vm.ID.String()+"/restart", nil)

	assert.Equal(suite.T(), http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), response["message"], "restart operation initiated")
}

func (suite *VMHandlerTestSuite) TestGetVMStats_Success() {
	vm := suite.createTestVM()

	// Update VM with some stats
	stats := models.VMStats{
		CPUUsagePercent:  45.2,
		RAMUsagePercent:  67.8,
		DiskUsagePercent: 23.1,
		UptimeSeconds:    3600,
		LastStatsUpdate:  time.Now(),
	}
	suite.vmRepo.UpdateStats(context.Background(), vm.ID, stats)

	w := suite.makeRequest("GET", "/api/v1/vms/"+vm.ID.String()+"/stats", nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)

	data := response["data"].(map[string]interface{})
	assert.Equal(suite.T(), 45.2, data["cpu_usage_percent"])
	assert.Equal(suite.T(), 67.8, data["ram_usage_percent"])
}

func (suite *VMHandlerTestSuite) TestGetResourceSummary_Success() {
	// Create test VMs
	suite.createTestVM()

	vm2 := &models.VM{
		Name:        "running-vm",
		Description: "Running test VM",
		Spec: models.VMSpec{
			CPUCores:    4,
			RAMMb:       8192,
			DiskGb:      100,
			ImageName:   "ubuntu:22.04",
			NetworkType: models.NetworkTypeNAT,
		},
		Status:    models.VMStatusRunning,
		NodeID:    "node-01",
		CreatedBy: "test-user",
		UpdatedBy: "test-user",
	}
	suite.vmRepo.Create(context.Background(), vm2)

	w := suite.makeRequest("GET", "/api/v1/stats/summary", nil)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)

	data := response["data"].(map[string]interface{})
	vms := data["vms"].(map[string]interface{})
	resources := data["resources"].(map[string]interface{})

	assert.Equal(suite.T(), float64(2), vms["total"])
	assert.Equal(suite.T(), float64(1), vms["running"])
	assert.Equal(suite.T(), float64(1), vms["stopped"])

	cpu := resources["cpu"].(map[string]interface{})
	assert.Equal(suite.T(), 6, cpu["total"]) // 2 + 4 cores
	assert.Equal(suite.T(), 4, cpu["used"])  // Only running VM
}

// Run the test suite
func TestVMHandlerSuite(t *testing.T) {
	suite.Run(t, new(VMHandlerTestSuite))
}

// Additional unit tests for individual components

func TestVMStatusTransitions(t *testing.T) {
	vm := &models.VM{Status: models.VMStatusStopped}

	// Valid transitions
	assert.True(t, vm.IsValidStatusTransition(models.VMStatusStarting))
	assert.True(t, vm.IsValidStatusTransition(models.VMStatusPending))

	// Invalid transitions
	assert.False(t, vm.IsValidStatusTransition(models.VMStatusRunning))
	assert.False(t, vm.IsValidStatusTransition(models.VMStatusStopping))

	// Test from running state
	vm.Status = models.VMStatusRunning
	assert.True(t, vm.IsValidStatusTransition(models.VMStatusStopping))
	assert.True(t, vm.IsValidStatusTransition(models.VMStatusSuspended))
	assert.False(t, vm.IsValidStatusTransition(models.VMStatusStarting))
}

func TestVMOperationPermissions(t *testing.T) {
	vm := &models.VM{Status: models.VMStatusStopped}

	assert.True(t, vm.CanPerformOperation("start"))
	assert.True(t, vm.CanPerformOperation("update"))
	assert.True(t, vm.CanPerformOperation("delete"))
	assert.False(t, vm.CanPerformOperation("stop"))
	assert.False(t, vm.CanPerformOperation("restart"))

	vm.Status = models.VMStatusRunning
	assert.False(t, vm.CanPerformOperation("start"))
	assert.False(t, vm.CanPerformOperation("update"))
	assert.False(t, vm.CanPerformOperation("delete"))
	assert.True(t, vm.CanPerformOperation("stop"))
	assert.True(t, vm.CanPerformOperation("restart"))
}

func TestVMUptime(t *testing.T) {
	vm := &models.VM{
		Status:    models.VMStatusStopped,
		StartedAt: nil,
	}

	assert.Equal(t, int64(0), vm.GetUptime())

	now := time.Now()
	vm.Status = models.VMStatusRunning
	vm.StartedAt = &now

	time.Sleep(time.Millisecond * 10)
	uptime := vm.GetUptime()
	assert.Greater(t, uptime, int64(0))
}

func TestVMLabelsAndAnnotations(t *testing.T) {
	vm := &models.VM{}

	// Test adding labels
	err := vm.AddLabel("environment", "test")
	assert.NoError(t, err)

	err = vm.AddLabel("tier", "backend")
	assert.NoError(t, err)

	// Test getting labels
	env, exists := vm.GetLabel("environment")
	assert.True(t, exists)
	assert.Equal(t, "test", env)

	tier, exists := vm.GetLabel("tier")
	assert.True(t, exists)
	assert.Equal(t, "backend", tier)

	_, exists = vm.GetLabel("nonexistent")
	assert.False(t, exists)

	// Test adding annotations
	err = vm.AddAnnotation("created_by", "automated-test")
	assert.NoError(t, err)
}
