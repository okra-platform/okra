package build

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for Packager:
// 1. Test NewPackager creates packager correctly
// 2. Test CreatePackage creates valid tar.gz file
// 3. Test package contains all required files
// 4. Test package metadata is correct
// 5. Test addFileToTar works correctly
// 6. Test addDataToTar works correctly

func TestNewPackager(t *testing.T) {
	// Test: NewPackager creates packager with config
	
	cfg := &config.Config{
		Name:    "test-service",
		Version: "1.0.0",
	}
	
	packager := NewPackager(cfg)
	
	assert.NotNil(t, packager)
	assert.Equal(t, cfg, packager.config)
}

func TestPackager_CreatePackage(t *testing.T) {
	// Test: CreatePackage creates a valid .pkg file
	
	tempDir := t.TempDir()
	
	// Create test WASM file
	wasmPath := filepath.Join(tempDir, "service.wasm")
	wasmContent := []byte("fake wasm content")
	err := os.WriteFile(wasmPath, wasmContent, 0644)
	require.NoError(t, err)
	
	// Create test protobuf descriptor
	pbPath := filepath.Join(tempDir, "service.pb.desc")
	pbContent := []byte("fake protobuf descriptor")
	err = os.WriteFile(pbPath, pbContent, 0644)
	require.NoError(t, err)
	
	// Create artifacts
	artifacts := &BuildArtifacts{
		WASMPath:               wasmPath,
		ProtobufDescriptorPath: pbPath,
		InterfacePath:          filepath.Join(tempDir, "interface.go"),
		Schema: &schema.Schema{
			Meta: schema.Metadata{
				Namespace: "test",
				Version:   "v1",
			},
			Services: []schema.Service{
				{
					Name: "TestService",
					Methods: []schema.Method{
						{
							Name:       "testMethod",
							InputType:  "TestInput",
							OutputType: "TestOutput",
						},
					},
				},
			},
		},
		BuildInfo: BuildInfo{
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Language:  "go",
		},
	}
	
	// Create packager
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "1.0.0",
		Language: "go",
	}
	packager := NewPackager(cfg)
	
	// Create package
	packagePath := filepath.Join(tempDir, "test.pkg")
	err = packager.CreatePackage(artifacts, packagePath)
	require.NoError(t, err)
	
	// Verify package exists
	_, err = os.Stat(packagePath)
	assert.NoError(t, err)
	
	// Verify package contents
	verifyPackageContents(t, packagePath, []string{
		"service.wasm",
		"service.description.json",
		"okra.service.json",
		"service.pb.desc",
	})
}

func TestCreateServiceMetadata(t *testing.T) {
	// Test: createServiceMetadata generates correct metadata
	
	cfg := &config.Config{
		Name:     "test-service",
		Version:  "2.0.0",
		Language: "typescript",
	}
	
	artifacts := &BuildArtifacts{
		Schema: &schema.Schema{
			Meta: schema.Metadata{
				Namespace: "myapp",
				Version:   "v2",
			},
			Services: []schema.Service{
				{
					Name: "GreeterService",
					Methods: []schema.Method{
						{
							Name:       "greet",
							InputType:  "GreetRequest",
							OutputType: "GreetResponse",
						},
						{
							Name:       "farewell",
							InputType:  "FarewellRequest",
							OutputType: "FarewellResponse",
						},
					},
				},
			},
		},
		BuildInfo: BuildInfo{
			Timestamp: time.Now(),
			Version:   "2.0.0",
			Language:  "typescript",
		},
	}
	
	packager := NewPackager(cfg)
	metadata := packager.createServiceMetadata(artifacts)
	
	assert.Equal(t, "test-service", metadata.Name)
	assert.Equal(t, "2.0.0", metadata.Version)
	assert.Equal(t, "typescript", metadata.Language)
	assert.Equal(t, "myapp", metadata.Namespace)
	assert.Len(t, metadata.Methods, 2)
	assert.Equal(t, "greet", metadata.Methods[0].Name)
	assert.Equal(t, "farewell", metadata.Methods[1].Name)
}

// verifyPackageContents extracts and verifies the contents of a package
func verifyPackageContents(t *testing.T, packagePath string, expectedFiles []string) {
	file, err := os.Open(packagePath)
	require.NoError(t, err)
	defer file.Close()
	
	gzReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer gzReader.Close()
	
	tarReader := tar.NewReader(gzReader)
	
	foundFiles := make(map[string]bool)
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		
		foundFiles[header.Name] = true
		
		// Verify file has content
		assert.Greater(t, header.Size, int64(0), "file %s should have content", header.Name)
	}
	
	// Verify all expected files are present
	for _, expectedFile := range expectedFiles {
		assert.True(t, foundFiles[expectedFile], "expected file %s not found in package", expectedFile)
	}
}