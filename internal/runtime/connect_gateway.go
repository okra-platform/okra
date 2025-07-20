package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/tochemey/goakt/v2/actors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ConnectGateway provides HTTP connectivity to OKRA services via ConnectRPC
type ConnectGateway interface {
	// Handler returns the HTTP handler for the gateway
	Handler() http.Handler

	// UpdateService updates the service configuration with new descriptors
	UpdateService(ctx context.Context, serviceName string, fds *descriptorpb.FileDescriptorSet, actorPID *actors.PID) error

	// Shutdown gracefully shuts down the gateway
	Shutdown(ctx context.Context) error
}

// ConnectGatewayOption configures a ConnectGateway
type ConnectGatewayOption func(*connectGateway)

// WithRequestTimeout sets the timeout for actor requests
func WithRequestTimeout(timeout time.Duration) ConnectGatewayOption {
	return func(cg *connectGateway) {
		cg.requestTimeout = timeout
	}
}

// NewConnectGateway creates a new ConnectRPC gateway
func NewConnectGateway(opts ...ConnectGatewayOption) ConnectGateway {
	cg := &connectGateway{
		mux:            http.NewServeMux(),
		services:       make(map[string]*serviceHandler),
		requestTimeout: 30 * time.Second, // Default timeout
	}
	
	for _, opt := range opts {
		opt(cg)
	}
	
	return cg
}

type connectGateway struct {
	mux            *http.ServeMux
	mu             sync.RWMutex
	services       map[string]*serviceHandler
	requestTimeout time.Duration
}

type serviceHandler struct {
	serviceName string
	actorPID    *actors.PID
	files       *protoregistry.Files
	handler     http.Handler
}

func (g *connectGateway) Handler() http.Handler {
	// Wrap the mux to handle /connect prefix
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /connect prefix if present
		if strings.HasPrefix(r.URL.Path, "/connect") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/connect")
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		g.mux.ServeHTTP(w, r)
	})
}

func (g *connectGateway) UpdateService(ctx context.Context, serviceName string, fds *descriptorpb.FileDescriptorSet, actorPID *actors.PID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Create a new protoregistry.Files from the FileDescriptorSet
	files := new(protoregistry.Files)
	for _, fdProto := range fds.File {
		file, err := protodesc.NewFile(fdProto, files)
		if err != nil {
			return fmt.Errorf("failed to create file descriptor: %w", err)
		}
		if err := files.RegisterFile(file); err != nil {
			return fmt.Errorf("failed to register file: %w", err)
		}
	}

	// Find the service descriptor
	var serviceDesc protoreflect.ServiceDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			sd := fd.Services().Get(i)
			if string(sd.Name()) == serviceName {
				serviceDesc = sd
				return false
			}
		}
		return true
	})

	if serviceDesc == nil {
		return fmt.Errorf("service %s not found in descriptors", serviceName)
	}

	// Create dynamic handler for the service
	handler := g.createDynamicHandler(serviceDesc, actorPID, files)

	// Store the handler
	g.services[serviceName] = &serviceHandler{
		serviceName: serviceName,
		actorPID:    actorPID,
		files:       files,
		handler:     handler,
	}

	// Register the handler with the mux
	// ConnectRPC uses the pattern /package.Service/Method
	pattern := fmt.Sprintf("/%s.%s/", serviceDesc.ParentFile().Package(), serviceName)
	fmt.Printf("ConnectGateway: Registering service handler for pattern: %s\n", pattern)
	g.mux.Handle(pattern, handler)

	return nil
}

func (g *connectGateway) createDynamicHandler(serviceDesc protoreflect.ServiceDescriptor, actorPID *actors.PID, files *protoregistry.Files) http.Handler {
	// Create a new mux for this service
	serviceMux := http.NewServeMux()

	// Register handlers for each method
	for i := 0; i < serviceDesc.Methods().Len(); i++ {
		method := serviceDesc.Methods().Get(i)
		methodPath := fmt.Sprintf("/%s.%s/%s",
			serviceDesc.ParentFile().Package(),
			serviceDesc.Name(),
			method.Name())

		// Capture method in closure
		capturedMethod := method
		capturedActorPID := actorPID

		// Create handler
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the input/output descriptors
			inputDesc := capturedMethod.Input()
			outputDesc := capturedMethod.Output()

			// Create dynamic messages
			inputMsg := dynamicpb.NewMessage(inputDesc)
			outputMsg := dynamicpb.NewMessage(outputDesc)

			// Check content type first
			contentType := r.Header.Get("Content-Type")
			
			// Parse content type to get the base type (ignore parameters)
			baseContentType := contentType
			if idx := strings.Index(contentType, ";"); idx != -1 {
				baseContentType = strings.TrimSpace(contentType[:idx])
			}
			
			// Validate base content type
			if baseContentType != "" && 
				baseContentType != "application/json" && 
				baseContentType != "application/connect+json" && 
				baseContentType != "application/proto" &&
				baseContentType != "application/x-protobuf" {
				http.Error(w, "unsupported content type", http.StatusBadRequest)
				return
			}

			// Set request size limit (10MB)
			const maxRequestSize = 10 * 1024 * 1024
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				if err.Error() == "http: request body too large" {
					http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			// Unmarshal the request based on content type
			if baseContentType == "application/json" || baseContentType == "application/connect+json" {
				// JSON format
				if err := protojson.Unmarshal(body, inputMsg); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			} else {
				// Default to protobuf format
				if err := proto.Unmarshal(body, inputMsg); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}

			// Convert to JSON for actor messaging
			jsonBytes, err := protojson.Marshal(inputMsg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Parse JSON to map
			var jsonMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Create service request for actor
			serviceRequest := &pb.ServiceRequest{
				Method: string(capturedMethod.Name()),
				Input:  jsonBytes, // Send JSON bytes to actor
			}

			// Determine timeout - use request context deadline if shorter than gateway timeout
			timeout := g.requestTimeout
			if deadline, ok := r.Context().Deadline(); ok {
				if timeRemaining := time.Until(deadline); timeRemaining < timeout {
					timeout = timeRemaining
				}
			}

			// Send request to actor and wait for response
			reply, err := actors.Ask(r.Context(), capturedActorPID, serviceRequest, timeout)
			if err != nil {
				// Check for context cancellation/timeout
				errStr := err.Error()
				
				// Only treat as timeout if we have an actual context deadline that was exceeded
				hasContextDeadline := false
				if deadline, ok := r.Context().Deadline(); ok {
					hasContextDeadline = time.Now().After(deadline)
				}
				
				if (err == context.DeadlineExceeded || err == context.Canceled || 
					strings.Contains(errStr, "context deadline exceeded") ||
					strings.Contains(errStr, "context canceled")) || 
					(errStr == "request timed out" && hasContextDeadline) {
					http.Error(w, "request timeout", http.StatusRequestTimeout)
					return
				}
				http.Error(w, fmt.Sprintf("actor request failed: %v", err), http.StatusInternalServerError)
				return
			}

			// Cast response to ServiceResponse
			serviceResponse, ok := reply.(*pb.ServiceResponse)
			if !ok {
				http.Error(w, "invalid response type from actor", http.StatusInternalServerError)
				return
			}

			// Check for errors in response
			if serviceResponse.Error != nil {
				http.Error(w, serviceResponse.Error.Message, http.StatusInternalServerError)
				return
			}

			// Parse JSON response back to protobuf
			if err := protojson.Unmarshal(serviceResponse.Output, outputMsg); err != nil {
				http.Error(w, fmt.Sprintf("failed to unmarshal response: %v", err), http.StatusInternalServerError)
				return
			}

			// Marshal response based on request content type
			var respBytes []byte
			if contentType == "application/json" || contentType == "application/connect+json" {
				// Return JSON response
				respBytes, err = protojson.Marshal(outputMsg)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", contentType)
			} else {
				// Return protobuf response
				respBytes, err = proto.Marshal(outputMsg)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/proto")
			}

			// Write response
			w.WriteHeader(http.StatusOK)
			w.Write(respBytes)
		})

		serviceMux.Handle(methodPath, handler)
	}

	return serviceMux
}

func (g *connectGateway) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Clear all services
	g.services = make(map[string]*serviceHandler)

	return nil
}
