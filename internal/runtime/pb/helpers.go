package pb

import (
	"google.golang.org/protobuf/types/known/anypb"
)

// NewServiceError creates a new ServiceError with the given code and message
func NewServiceError(code, message string) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
		Details: make(map[string]*anypb.Any),
	}
}

// NewServiceResponse creates a new ServiceResponse with the given ID and success status
func NewServiceResponse(id string, success bool) *ServiceResponse {
	return &ServiceResponse{
		Id:       id,
		Success:  success,
		Metadata: make(map[string]string),
	}
}