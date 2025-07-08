package service

import (
	"{{project_name}}/types"
)

// service implements the generated service interface
type service struct {
	// Add any service dependencies here
}

// NewService creates a new instance of the service
// This is the constructor that OKRA will call to initialize your service
func NewService() types.Service {
	return &service{}
}

// Example method implementation
// Replace this with your actual service methods as defined in service.okra.gql
func (s *service) Hello(input *types.HelloInput) (*types.HelloOutput, error) {
	return &types.HelloOutput{
		Message: "Hello, " + input.Name + "!",
	}, nil
}