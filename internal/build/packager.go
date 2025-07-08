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

// Packager creates .pkg files from build artifacts

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
	
	// Create and add okra.json (the original config file)
	configJSON, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := p.addDataToTar(tarWriter, configJSON, "okra.json"); err != nil {
		return fmt.Errorf("failed to add config to package: %w", err)
	}
	
	// Add protobuf descriptor if available
	if _, err := os.Stat(artifacts.ProtobufDescriptorPath); err == nil {
		if err := p.addFileToTar(tarWriter, artifacts.ProtobufDescriptorPath, "service.pb.desc"); err != nil {
			return fmt.Errorf("failed to add protobuf descriptor to package: %w", err)
		}
	}
	
	return nil
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