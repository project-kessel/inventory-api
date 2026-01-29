package middleware

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
		expectedMsg  string
	}{
		// Application-layer errors (from usecase)
		{
			name:         "ErrMetaAuthzContextMissing maps to Unauthenticated",
			err:          resources.ErrMetaAuthzContextMissing,
			expectedCode: codes.Unauthenticated,
			expectedMsg:  "authz context missing",
		},
		{
			name:         "ErrSelfSubjectMissing maps to Unauthenticated",
			err:          resources.ErrSelfSubjectMissing,
			expectedCode: codes.Unauthenticated,
			expectedMsg:  "self subject missing",
		},
		{
			name:         "ErrMetaAuthorizerUnavailable maps to Internal",
			err:          resources.ErrMetaAuthorizerUnavailable,
			expectedCode: codes.Internal,
			expectedMsg:  "meta authorizer unavailable",
		},
		{
			name:         "ErrMetaAuthorizationDenied maps to PermissionDenied",
			err:          resources.ErrMetaAuthorizationDenied,
			expectedCode: codes.PermissionDenied,
			expectedMsg:  "meta authorization denied",
		},
		// Domain errors (from model)
		{
			name:         "ErrResourceNotFound maps to NotFound",
			err:          model.ErrResourceNotFound,
			expectedCode: codes.NotFound,
			expectedMsg:  "resource not found",
		},
		{
			name:         "ErrResourceAlreadyExists maps to AlreadyExists",
			err:          model.ErrResourceAlreadyExists,
			expectedCode: codes.AlreadyExists,
			expectedMsg:  "resource already exists",
		},
		{
			name:         "ErrInventoryIdMismatch maps to FailedPrecondition",
			err:          model.ErrInventoryIdMismatch,
			expectedCode: codes.FailedPrecondition,
			expectedMsg:  "resource inventory id mismatch",
		},
		{
			name:         "ErrDatabaseError maps to Internal",
			err:          model.ErrDatabaseError,
			expectedCode: codes.Internal,
			expectedMsg:  "internal error",
		},
		// Validation errors
		{
			name:         "ErrEmpty maps to InvalidArgument",
			err:          model.ErrEmpty,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "required field is empty",
		},
		{
			name:         "ErrTooLong maps to InvalidArgument",
			err:          model.ErrTooLong,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "field exceeds maximum length",
		},
		{
			name:         "ErrTooSmall maps to InvalidArgument",
			err:          model.ErrTooSmall,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "field below minimum value",
		},
		{
			name:         "ErrInvalidURL maps to InvalidArgument",
			err:          model.ErrInvalidURL,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid URL",
		},
		{
			name:         "ErrInvalidUUID maps to InvalidArgument",
			err:          model.ErrInvalidUUID,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid UUID",
		},
		{
			name:         "ErrInvalidData maps to InvalidArgument",
			err:          model.ErrInvalidData,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid data structure",
		},
		// Context errors
		{
			name:         "context.Canceled maps to Canceled",
			err:          context.Canceled,
			expectedCode: codes.Canceled,
			expectedMsg:  "request canceled",
		},
		{
			name:         "context.DeadlineExceeded maps to DeadlineExceeded",
			err:          context.DeadlineExceeded,
			expectedCode: codes.DeadlineExceeded,
			expectedMsg:  "request deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapError(tt.err)
			st, ok := status.FromError(result)
			assert.True(t, ok, "expected gRPC status error")
			assert.Equal(t, tt.expectedCode, st.Code())
			assert.Equal(t, tt.expectedMsg, st.Message())
		})
	}
}

func TestMapError_WrappedErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
	}{
		{
			name:         "wrapped ErrResourceNotFound maps to NotFound",
			err:          fmt.Errorf("failed to find: %w", model.ErrResourceNotFound),
			expectedCode: codes.NotFound,
		},
		{
			name:         "wrapped ErrMetaAuthorizationDenied maps to PermissionDenied",
			err:          fmt.Errorf("authorization check failed: %w", resources.ErrMetaAuthorizationDenied),
			expectedCode: codes.PermissionDenied,
		},
		{
			name:         "wrapped ErrEmpty maps to InvalidArgument",
			err:          fmt.Errorf("validation failed: %w", model.ErrEmpty),
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "wrapped validation error with field context",
			err:          fmt.Errorf("%w: resourceId", model.ErrEmpty),
			expectedCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapError(tt.err)
			st, ok := status.FromError(result)
			assert.True(t, ok, "expected gRPC status error")
			assert.Equal(t, tt.expectedCode, st.Code())
		})
	}
}

func TestMapError_PreservesExistingStatusCodes(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
		expectedMsg  string
	}{
		{
			name:         "NotFound status passes through",
			err:          status.Error(codes.NotFound, "custom not found message"),
			expectedCode: codes.NotFound,
			expectedMsg:  "custom not found message",
		},
		{
			name:         "PermissionDenied status passes through",
			err:          status.Error(codes.PermissionDenied, "custom permission denied"),
			expectedCode: codes.PermissionDenied,
			expectedMsg:  "custom permission denied",
		},
		{
			name:         "InvalidArgument status passes through",
			err:          status.Error(codes.InvalidArgument, "custom invalid argument"),
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "custom invalid argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapError(tt.err)
			st, ok := status.FromError(result)
			assert.True(t, ok, "expected gRPC status error")
			assert.Equal(t, tt.expectedCode, st.Code())
			assert.Equal(t, tt.expectedMsg, st.Message())
		})
	}
}

func TestMapError_UnknownErrorsPassThrough(t *testing.T) {
	unknownErr := errors.New("some unknown error")
	result := MapError(unknownErr)
	// Unknown errors pass through unchanged
	assert.Equal(t, unknownErr, result)
}

func TestMapError_NilReturnsNil(t *testing.T) {
	result := MapError(nil)
	assert.Nil(t, result)
}

// TestMapError_ResourcesErrResourceNotFound verifies that the re-exported
// resources.ErrResourceNotFound (which equals model.ErrResourceNotFound) is handled correctly
func TestMapError_ResourcesErrResourceNotFound(t *testing.T) {
	// resources.ErrResourceNotFound is a re-export of model.ErrResourceNotFound
	result := MapError(resources.ErrResourceNotFound)
	st, ok := status.FromError(result)
	assert.True(t, ok, "expected gRPC status error")
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "resource not found", st.Message())
}
