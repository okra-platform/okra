package service

import (
	"fmt"
	"time"
	
	"test-service/types"
)

// service implements the generated service interface
type service struct{}

// NewService creates a new instance of the service
// This is the constructor that OKRA will call to initialize your service
func NewService() types.Service {
	return &service{}
}

// Greet returns a greeting message
func (s *service) Greet(input *types.GreetRequest) (*types.GreetResponse, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	
	return &types.GreetResponse{
		Message:   fmt.Sprintf("Hello, %s! Welcome to OKRA.", input.Name),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}