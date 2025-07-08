package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/okra-platform/okra/internal/runtime"
)

// AdminServer provides an HTTP API for managing deployed services
type AdminServer interface {
	Start(ctx context.Context, port int) error
}

// adminServer is the internal implementation of AdminServer
type adminServer struct {
	runtime runtime.Runtime
	gateway runtime.ConnectGateway
	
	// Track deployed services and their sources
	deployedServices map[string]*DeployedService
	servicesMu       sync.RWMutex
	
	server *http.Server
}

// DeployedService tracks a deployed service
type DeployedService struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	DeployedAt time.Time `json:"deployed_at"`
}

// DeployRequest represents a package deployment request
type DeployRequest struct {
	Source   string `json:"source"`   // file:// or s3:// URL
	Override bool   `json:"override"` // Allow redeploying same service
}

// DeployResponse represents a deployment response
type DeployResponse struct {
	ServiceID string   `json:"service_id"`
	Status    string   `json:"status"`
	Endpoints []string `json:"endpoints,omitempty"`
}

// ListServicesResponse represents the list of deployed services
type ListServicesResponse struct {
	Services []*DeployedService `json:"services"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewAdminServer creates a new admin server
func NewAdminServer(runtime runtime.Runtime, gateway runtime.ConnectGateway) AdminServer {
	return &adminServer{
		runtime:          runtime,
		gateway:          gateway,
		deployedServices: make(map[string]*DeployedService),
	}
}

// Start starts the admin server on the specified port
func (s *adminServer) Start(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/packages/deploy", s.handleDeploy)
	mux.HandleFunc("/api/v1/packages/", s.handleUndeploy)  // Note the trailing slash for path prefix
	mux.HandleFunc("/api/v1/packages", s.handleListServices)
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	
	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// handleHealth handles health check requests
func (s *adminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleDeploy handles package deployment requests
func (s *adminServer) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	// Validate source
	if req.Source == "" {
		s.sendError(w, http.StatusBadRequest, "source is required")
		return
	}
	
	// Deploy the package
	serviceID, endpoints, err := s.deployPackage(r.Context(), req.Source, req.Override)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// Track the deployment
	s.servicesMu.Lock()
	s.deployedServices[serviceID] = &DeployedService{
		ID:         serviceID,
		Source:     req.Source,
		DeployedAt: time.Now(),
	}
	s.servicesMu.Unlock()
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&DeployResponse{
		ServiceID: serviceID,
		Status:    "deployed",
		Endpoints: endpoints,
	})
}


// handleListServices handles listing deployed services
func (s *adminServer) handleListServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.servicesMu.RLock()
	services := make([]*DeployedService, 0, len(s.deployedServices))
	for _, svc := range s.deployedServices {
		services = append(services, svc)
	}
	s.servicesMu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&ListServicesResponse{
		Services: services,
	})
}

// handleUndeploy handles service undeployment
func (s *adminServer) handleUndeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract service ID from URL path
	// Expected format: /api/v1/packages/{id}
	path := r.URL.Path
	prefix := "/api/v1/packages/"
	if len(path) <= len(prefix) {
		s.sendError(w, http.StatusBadRequest, "service ID required")
		return
	}
	
	serviceID := path[len(prefix):]
	if serviceID == "" {
		s.sendError(w, http.StatusBadRequest, "service ID required")
		return
	}
	
	// Check if service exists
	s.servicesMu.RLock()
	_, exists := s.deployedServices[serviceID]
	s.servicesMu.RUnlock()
	
	if !exists {
		s.sendError(w, http.StatusNotFound, "service not found")
		return
	}
	
	// Undeploy from runtime
	if err := s.runtime.Undeploy(r.Context(), serviceID); err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// Gateway will be updated on next deployment with new service
	
	// Remove from tracking
	s.servicesMu.Lock()
	delete(s.deployedServices, serviceID)
	s.servicesMu.Unlock()
	
	w.WriteHeader(http.StatusNoContent)
}

// deployPackage deploys a package from the given source
func (s *adminServer) deployPackage(ctx context.Context, source string, override bool) (string, []string, error) {
	// Load the package
	pkg, err := LoadPackage(ctx, source)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load package: %w", err)
	}
	
	// Deploy to runtime
	actorID, err := s.runtime.Deploy(ctx, pkg)
	if err != nil {
		return "", nil, fmt.Errorf("failed to deploy to runtime: %w", err)
	}
	
	// Update gateway with protobuf descriptors if available
	var endpoints []string
	if pkg.FileDescriptors != nil {
		fmt.Printf("Debug: Updating gateway for service %s (actor ID: %s)\n", pkg.ServiceName, actorID)
		
		// Get actor PID from runtime (same approach as dev server)
		okraRuntime, ok := s.runtime.(*runtime.OkraRuntime)
		if !ok {
			fmt.Printf("Warning: runtime is not OkraRuntime type, cannot update gateway\n")
		} else {
			actorPID := okraRuntime.GetActorPID(actorID)
			if actorPID == nil {
				fmt.Printf("Warning: failed to get actor PID for service %s (actor ID: %s)\n", pkg.ServiceName, actorID)
			} else {
			fmt.Printf("Debug: Got actor PID for service %s\n", pkg.ServiceName)
			if err := s.gateway.UpdateService(ctx, pkg.ServiceName, pkg.FileDescriptors, actorPID); err != nil {
				// Log error but don't fail deployment
				fmt.Printf("Warning: failed to update gateway with service: %v\n", err)
			} else {
				fmt.Printf("✅ Service %s deployed and exposed via ConnectRPC\n", pkg.ServiceName)
				// Generate endpoint URLs based on service methods
				for methodName := range pkg.Methods {
					endpoint := fmt.Sprintf("/%s.%s/%s", 
						pkg.Schema.Meta.Namespace, 
						pkg.ServiceName, 
						methodName)
					endpoints = append(endpoints, endpoint)
				}
			}
		}
		}
	} else {
		fmt.Printf("Warning: No FileDescriptors for service %s\n", pkg.ServiceName)
	}
	
	return actorID, endpoints, nil
}

// sendError sends an error response
func (s *adminServer) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&ErrorResponse{
		Error: message,
	})
}