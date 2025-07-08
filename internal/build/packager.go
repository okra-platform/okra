package build

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/okra-platform/okra/internal/config"
)

// ServiceMetadata contains metadata about the service for okra.service.json
type ServiceMetadata struct {
	// Name of the service
	Name string `json:"name"`
	
	// Version of the service
	Version string `json:"version"`
	
	// Language the service was written in
	Language string `json:"language"`
	
	// Namespace from the schema
	Namespace string `json:"namespace"`
	
	// Methods supported by the service
	Methods []MethodInfo `json:"methods"`
	
	// BuildInfo contains build metadata
	BuildInfo BuildInfo `json:"buildInfo"`
	
	// RequiredHostAPIs lists any host APIs this service requires
	RequiredHostAPIs []string `json:"requiredHostAPIs,omitempty"`
}

// MethodInfo contains information about a service method
type MethodInfo struct {
	Name       string `json:"name"`
	InputType  string `json:"inputType"`
	OutputType string `json:"outputType"`
}

// Packager creates .pkg files from build artifacts
type Packager struct {
	config  *config.Config
	logger  interface{} // Use interface to avoid circular dependency
}

// NewPackager creates a new packager
func NewPackager(cfg *config.Config) *Packager {
	return &Packager{
		config: cfg,
	}
}

// CreatePackage creates a .pkg file from build artifacts
func (p *Packager) CreatePackage(artifacts *BuildArtifacts, outputPath string) error {
	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Create the package file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create package file: %w", err)
	}
	defer file.Close()
	
	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	
	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	
	// Add service.wasm
	if err := p.addFileToTar(tarWriter, artifacts.WASMPath, "service.wasm"); err != nil {
		return fmt.Errorf("failed to add WASM to package: %w", err)
	}
	
	// Create and add service.description.json
	schemaJSON, err := json.MarshalIndent(artifacts.Schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	if err := p.addDataToTar(tarWriter, schemaJSON, "service.description.json"); err != nil {
		return fmt.Errorf("failed to add schema to package: %w", err)
	}
	
	// Create and add okra.service.json
	metadata := p.createServiceMetadata(artifacts)
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := p.addDataToTar(tarWriter, metadataJSON, "okra.service.json"); err != nil {
		return fmt.Errorf("failed to add metadata to package: %w", err)
	}
	
	// Add protobuf descriptor if available
	if _, err := os.Stat(artifacts.ProtobufDescriptorPath); err == nil {
		if err := p.addFileToTar(tarWriter, artifacts.ProtobufDescriptorPath, "service.pb.desc"); err != nil {
			return fmt.Errorf("failed to add protobuf descriptor to package: %w", err)
		}
	}
	
	return nil
}

// createServiceMetadata creates service metadata from artifacts
func (p *Packager) createServiceMetadata(artifacts *BuildArtifacts) *ServiceMetadata {
	// Extract methods from schema
	var methods []MethodInfo
	if len(artifacts.Schema.Services) > 0 {
		service := artifacts.Schema.Services[0]
		for _, method := range service.Methods {
			methods = append(methods, MethodInfo{
				Name:       method.Name,
				InputType:  method.InputType,
				OutputType: method.OutputType,
			})
		}
	}
	
	// Get namespace
	namespace := artifacts.Schema.Meta.Namespace
	if namespace == "" {
		namespace = "default"
	}
	
	return &ServiceMetadata{
		Name:      p.config.Name,
		Version:   p.config.Version,
		Language:  p.config.Language,
		Namespace: namespace,
		Methods:   methods,
		BuildInfo: artifacts.BuildInfo,
		// TODO: Extract required host APIs from schema directives
		RequiredHostAPIs: []string{},
	}
}

// addFileToTar adds a file to the tar archive
func (p *Packager) addFileToTar(tw *tar.Writer, sourcePath, destName string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return err
	}
	
	header := &tar.Header{
		Name:    destName,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	
	_, err = io.Copy(tw, file)
	return err
}

// addDataToTar adds raw data to the tar archive
func (p *Packager) addDataToTar(tw *tar.Writer, data []byte, name string) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	
	_, err := tw.Write(data)
	return err
}