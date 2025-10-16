package errors

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
)

// AppError represents an application error with additional context
type AppError struct {
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Details  string            `json:"details,omitempty"`
	HTTPCode int               `json:"-"`
	Internal error             `json:"-"`
	Context  map[string]string `json:"context,omitempty"`
	File     string            `json:"-"`
	Line     int               `json:"-"`
	Function string            `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Internal
}

// WithContext adds context to the error
func (e *AppError) WithContext(key, value string) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]string)
	}
	e.Context[key] = value
	return e
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// Error types
var (
	// Validation errors
	ErrInvalidInput     = &AppError{Code: "INVALID_INPUT", Message: "Invalid input provided", HTTPCode: http.StatusBadRequest}
	ErrValidationFailed = &AppError{Code: "VALIDATION_FAILED", Message: "Validation failed", HTTPCode: http.StatusBadRequest}
	ErrMissingField     = &AppError{Code: "MISSING_FIELD", Message: "Required field is missing", HTTPCode: http.StatusBadRequest}

	// Authentication errors
	ErrUnauthorized     = &AppError{Code: "UNAUTHORIZED", Message: "Authentication required", HTTPCode: http.StatusUnauthorized}
	ErrInvalidToken     = &AppError{Code: "INVALID_TOKEN", Message: "Invalid or expired token", HTTPCode: http.StatusUnauthorized}
	ErrInsufficientPerm = &AppError{Code: "INSUFFICIENT_PERMISSIONS", Message: "Insufficient permissions", HTTPCode: http.StatusForbidden}

	// Resource errors
	ErrNotFound       = &AppError{Code: "NOT_FOUND", Message: "Resource not found", HTTPCode: http.StatusNotFound}
	ErrAlreadyExists  = &AppError{Code: "ALREADY_EXISTS", Message: "Resource already exists", HTTPCode: http.StatusConflict}
	ErrResourceLocked = &AppError{Code: "RESOURCE_LOCKED", Message: "Resource is locked", HTTPCode: http.StatusConflict}

	// VM specific errors
	ErrVMNotFound       = &AppError{Code: "VM_NOT_FOUND", Message: "Virtual machine not found", HTTPCode: http.StatusNotFound}
	ErrVMAlreadyRunning = &AppError{Code: "VM_ALREADY_RUNNING", Message: "Virtual machine is already running", HTTPCode: http.StatusConflict}
	ErrVMNotRunning     = &AppError{Code: "VM_NOT_RUNNING", Message: "Virtual machine is not running", HTTPCode: http.StatusConflict}
	ErrInvalidVMState   = &AppError{Code: "INVALID_VM_STATE", Message: "Invalid virtual machine state for this operation", HTTPCode: http.StatusConflict}
	ErrResourceExceeded = &AppError{Code: "RESOURCE_EXCEEDED", Message: "Resource limits exceeded", HTTPCode: http.StatusConflict}

	// System errors
	ErrInternalServer     = &AppError{Code: "INTERNAL_SERVER_ERROR", Message: "Internal server error", HTTPCode: http.StatusInternalServerError}
	ErrDatabaseError      = &AppError{Code: "DATABASE_ERROR", Message: "Database error occurred", HTTPCode: http.StatusInternalServerError}
	ErrServiceUnavailable = &AppError{Code: "SERVICE_UNAVAILABLE", Message: "Service temporarily unavailable", HTTPCode: http.StatusServiceUnavailable}

	// Rate limiting errors
	ErrRateLimitExceeded = &AppError{Code: "RATE_LIMIT_EXCEEDED", Message: "Rate limit exceeded", HTTPCode: http.StatusTooManyRequests}
)

// New creates a new AppError with stack trace
func New(code, message string, httpCode int) *AppError {
	pc, file, line, _ := runtime.Caller(1)
	function := runtime.FuncForPC(pc).Name()

	return &AppError{
		Code:     code,
		Message:  message,
		HTTPCode: httpCode,
		File:     file,
		Line:     line,
		Function: function,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code, message string, httpCode int) *AppError {
	if err == nil {
		return nil
	}

	pc, file, line, _ := runtime.Caller(1)
	function := runtime.FuncForPC(pc).Name()

	return &AppError{
		Code:     code,
		Message:  message,
		HTTPCode: httpCode,
		Internal: err,
		File:     file,
		Line:     line,
		Function: function,
	}
}

// WrapWithCode wraps an error with a predefined error code
func WrapWithCode(err error, appErr *AppError) *AppError {
	if err == nil {
		return nil
	}

	pc, file, line, _ := runtime.Caller(1)
	function := runtime.FuncForPC(pc).Name()

	return &AppError{
		Code:     appErr.Code,
		Message:  appErr.Message,
		HTTPCode: appErr.HTTPCode,
		Internal: err,
		File:     file,
		Line:     line,
		Function: function,
	}
}

// Is checks if an error matches a specific AppError type
func Is(err error, target *AppError) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == target.Code
	}
	return false
}

// GetHTTPCode extracts HTTP status code from error
func GetHTTPCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPCode
	}
	return http.StatusInternalServerError
}

// GetCode extracts error code from error
func GetCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return "UNKNOWN_ERROR"
}

// ToAppError converts any error to AppError
func ToAppError(err error) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	pc, file, line, _ := runtime.Caller(1)
	function := runtime.FuncForPC(pc).Name()

	return &AppError{
		Code:     "UNKNOWN_ERROR",
		Message:  "An unexpected error occurred",
		HTTPCode: http.StatusInternalServerError,
		Internal: err,
		File:     file,
		Line:     line,
		Function: function,
	}
}

// ValidationError creates a validation error with field details
func ValidationError(field, message string) *AppError {
	return ErrValidationFailed.WithContext("field", field).WithDetails(message)
}

// NotFoundError creates a not found error for a specific resource
func NotFoundError(resourceType, resourceID string) *AppError {
	return ErrNotFound.
		WithContext("resource_type", resourceType).
		WithContext("resource_id", resourceID).
		WithDetails(fmt.Sprintf("%s with ID %s not found", resourceType, resourceID))
}

// AlreadyExistsError creates an already exists error for a specific resource
func AlreadyExistsError(resourceType, resourceID string) *AppError {
	return ErrAlreadyExists.
		WithContext("resource_type", resourceType).
		WithContext("resource_id", resourceID).
		WithDetails(fmt.Sprintf("%s with ID %s already exists", resourceType, resourceID))
}

// VMStateError creates a VM state error
func VMStateError(vmID, currentState, requiredState string) *AppError {
	return ErrInvalidVMState.
		WithContext("vm_id", vmID).
		WithContext("current_state", currentState).
		WithContext("required_state", requiredState).
		WithDetails(fmt.Sprintf("VM %s is in state %s, but operation requires %s", vmID, currentState, requiredState))
}

// ResourceLimitError creates a resource limit error
func ResourceLimitError(resourceType string, requested, limit int) *AppError {
	return ErrResourceExceeded.
		WithContext("resource_type", resourceType).
		WithContext("requested", fmt.Sprintf("%d", requested)).
		WithContext("limit", fmt.Sprintf("%d", limit)).
		WithDetails(fmt.Sprintf("Requested %s (%d) exceeds limit (%d)", resourceType, requested, limit))
}

// DatabaseError creates a database error
func DatabaseError(operation string, err error) *AppError {
	return Wrap(err, "DATABASE_ERROR", fmt.Sprintf("Database error during %s", operation), http.StatusInternalServerError)
}

// InternalError creates an internal server error
func InternalError(message string, err error) *AppError {
	return Wrap(err, "INTERNAL_SERVER_ERROR", message, http.StatusInternalServerError)
}
