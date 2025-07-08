package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// NewConnectGateway creates a new ConnectRPC gateway
func NewConnectGateway() ConnectGateway {
	return &connectGateway{
		mux:      http.NewServeMux(),
		services: make(map[string]*serviceHandler),
	}
}

type connectGateway struct {
	mux      *http.ServeMux
	mu       sync.RWMutex
	services map[string]*serviceHandler
}

type serviceHandler struct {
	serviceName string
	actorPID    *actors.PID
	files       *protoregistry.Files
	handler     http.Handler
}

func (g *connectGateway) Handler() http.Handler {
	return g.mux
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
			
			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer r.Body.Close()
			
			// Check content type to determine format
			contentType := r.Header.Get("Content-Type")
			
			// Unmarshal the request based on content type
			if contentType == "application/json" || contentType == "application/connect+json" {
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
			
			// Send request to actor and wait for response
			reply, err := actors.Ask(r.Context(), capturedActorPID, serviceRequest, 30*time.Second)
			if err != nil {
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