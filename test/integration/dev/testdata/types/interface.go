package types

type GreetRequest struct {
	Name string `json:"name"`
}

type GreetResponse struct {
	Message string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// Service defines the service interface
type Service interface {
	Greet(input *GreetRequest) (*GreetResponse, error)
}

