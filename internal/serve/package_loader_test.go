package serve

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Test plan for package loader:
// 1. Test extractPackage extracts tar.gz correctly
// 2. Test validatePackageFiles checks for required files
// 3. Test validatePackageFiles validates WASM magic number
// 4. Test loadPackageComponents loads all components correctly
// 5. Test LoadPackage with valid local file
// 6. Test LoadPackage with invalid source URL
// 7. Test LoadPackage with missing file

func createTestPackage(t *testing.T, dir string) string {
	// Create a test package file
	packagePath := filepath.Join(dir, "test.pkg")

	file, err := os.Create(packagePath)
	require.NoError(t, err)
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add test files
	files := map[string][]byte{
		"service.wasm": {0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}, // Valid WASM magic
		"service.description.json": []byte(`{
			"meta": {"namespace": "test", "version": "v1"},
			"services": [{
				"name": "TestService",
				"methods": [{"name": "TestMethod", "inputType": "TestInput", "outputType": "TestOutput"}]
			}]
		}`),
		"okra.json": []byte(`{
			"name": "TestService",
			"version": "1.0.0",
			"language": "go",
			"schema": "service.okra.gql",
			"source": "./",
			"build": {"output": "./build/service.wasm"},
			"dev": {"watch": ["*.go"], "exclude": ["*_test.go"]}
		}`),
		"service.pb.desc": []byte{0x0A, 0x00}, // Minimal protobuf descriptor
	}

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}

		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)

		_, err = tarWriter.Write(content)
		require.NoError(t, err)
	}

	return packagePath
}

func TestExtractPackage(t *testing.T) {
	// Test: extractPackage correctly extracts tar.gz archive

	tempDir := t.TempDir()
	packagePath := createTestPackage(t, tempDir)

	extractDir := filepath.Join(tempDir, "extract")
	err := os.MkdirAll(extractDir, 0755)
	require.NoError(t, err)

	files, err := extractPackage(packagePath, extractDir)
	require.NoError(t, err)

	// Verify all expected files were extracted
	expectedFiles := []string{"service.wasm", "service.description.json", "okra.json", "service.pb.desc"}
	assert.Len(t, files, len(expectedFiles))

	for _, expected := range expectedFiles {
		path, ok := files[expected]
		assert.True(t, ok, "expected file %s not found", expected)

		// Verify file exists
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}
}

func TestValidatePackageFiles(t *testing.T) {
	// Test: validatePackageFiles checks for required files and valid WASM

	tempDir := t.TempDir()

	// Create valid WASM file
	wasmPath := filepath.Join(tempDir, "service.wasm")
	err := os.WriteFile(wasmPath, []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}, 0644)
	require.NoError(t, err)

	// Test with all required files
	validFiles := map[string]string{
		"service.wasm":             wasmPath,
		"service.description.json": filepath.Join(tempDir, "desc.json"),
		"okra.json":                filepath.Join(tempDir, "okra.json"),
		"service.pb.desc":          filepath.Join(tempDir, "pb.desc"),
	}

	// Create empty files for other required files
	for name, path := range validFiles {
		if name != "service.wasm" {
			err := os.WriteFile(path, []byte("{}"), 0644)
			require.NoError(t, err)
		}
	}

	err = validatePackageFiles(validFiles)
	assert.NoError(t, err)

	// Test missing file
	delete(validFiles, "okra.json")
	err = validatePackageFiles(validFiles)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required file: okra.json")

	// Test invalid WASM magic number
	validFiles["okra.json"] = filepath.Join(tempDir, "okra.json")
	err = os.WriteFile(wasmPath, []byte{0xFF, 0xFF, 0xFF, 0xFF}, 0644)
	require.NoError(t, err)

	err = validatePackageFiles(validFiles)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid WASM file")
}

func TestLoadPackageComponents(t *testing.T) {
	// Test: loadPackageComponents loads and validates all components

	tempDir := t.TempDir()

	// Create test config
	cfg := config.Config{
		Name:     "TestService",
		Version:  "1.0.0",
		Language: "go",
	}
	configData, _ := json.Marshal(cfg)
	configPath := filepath.Join(tempDir, "okra.json")
	err := os.WriteFile(configPath, configData, 0644)
	require.NoError(t, err)

	// Create test schema
	sch := schema.Schema{
		Meta: schema.Metadata{
			Namespace: "test",
			Version:   "v1",
		},
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{Name: "TestMethod", InputType: "TestInput", OutputType: "TestOutput"},
				},
			},
		},
	}
	schemaData, _ := json.Marshal(sch)
	schemaPath := filepath.Join(tempDir, "service.description.json")
	err = os.WriteFile(schemaPath, schemaData, 0644)
	require.NoError(t, err)

	// Create valid WASM file
	wasmPath := filepath.Join(tempDir, "service.wasm")
	// This is a minimal valid WASM module (empty module)
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6D, // Magic
		0x01, 0x00, 0x00, 0x00, // Version
	}
	err = os.WriteFile(wasmPath, wasmBytes, 0644)
	require.NoError(t, err)

	// Create protobuf descriptor (minimal valid descriptor)
	pbPath := filepath.Join(tempDir, "service.pb.desc")
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{},
	}
	pbData, _ := proto.Marshal(fds)
	err = os.WriteFile(pbPath, pbData, 0644)
	require.NoError(t, err)

	files := map[string]string{
		"okra.json":                configPath,
		"service.description.json": schemaPath,
		"service.wasm":             wasmPath,
		"service.pb.desc":          pbPath,
	}

	pkg, err := loadPackageComponents(files)
	require.NoError(t, err)

	assert.NotNil(t, pkg)
	assert.Equal(t, "TestService", pkg.ServiceName)
	assert.Len(t, pkg.Methods, 1)
	assert.NotNil(t, pkg.FileDescriptors)
}

func TestLoadPackage_LocalFile(t *testing.T) {
	// Test: LoadPackage successfully loads a local package file

	tempDir := t.TempDir()
	packagePath := createTestPackage(t, tempDir)

	ctx := context.Background()
	pkg, err := LoadPackage(ctx, "file://"+packagePath)

	// The test package has minimal valid WASM, so it might actually succeed
	if err != nil {
		// If it fails, it should be at WASM compilation
		assert.Contains(t, err.Error(), "failed to compile WASM module")
		assert.Nil(t, pkg)
	} else {
		// If it succeeds, verify the package
		assert.NotNil(t, pkg)
		assert.Equal(t, "TestService", pkg.ServiceName)
		assert.Len(t, pkg.Methods, 1)
		assert.NotNil(t, pkg.FileDescriptors)
	}
}

func TestLoadPackage_InvalidURL(t *testing.T) {
	// Test: LoadPackage rejects invalid URLs

	ctx := context.Background()

	tests := []struct {
		name   string
		source string
		errMsg string
	}{
		{
			name:   "invalid URL",
			source: "not a url",
			errMsg: "unsupported source scheme",
		},
		{
			name:   "unsupported scheme",
			source: "http://example.com/package.pkg",
			errMsg: "unsupported source scheme: http",
		},
		{
			name:   "missing file",
			source: "file:///does/not/exist.pkg",
			errMsg: "package file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := LoadPackage(ctx, tt.source)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, pkg)
		})
	}
}

func TestLoadPackage_CorruptedPackage(t *testing.T) {
	// Test: LoadPackage handles corrupted package files gracefully
	ctx := context.Background()
	tempDir := t.TempDir()
	
	tests := []struct {
		name     string
		content  []byte
		errMsg   string
	}{
		{
			name:    "empty file",
			content: []byte{},
			errMsg:  "failed to extract package",
		},
		{
			name:    "random data",
			content: []byte("this is not a valid gzip file at all"),
			errMsg:  "failed to extract package",
		},
		{
			name:    "truncated gzip",
			content: []byte{0x1f, 0x8b, 0x08, 0x00}, // Gzip header only
			errMsg:  "failed to extract package",
		},
		{
			name:    "valid gzip but not tar",
			content: compressData([]byte("not a tar file")),
			errMsg:  "failed to extract package",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packagePath := filepath.Join(tempDir, tt.name+".pkg")
			err := os.WriteFile(packagePath, tt.content, 0644)
			require.NoError(t, err)
			
			pkg, err := LoadPackage(ctx, "file://"+packagePath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, pkg)
		})
	}
}

func TestLoadPackage_InvalidPackageContents(t *testing.T) {
	// Test: LoadPackage validates package contents thoroughly
	ctx := context.Background()
	tempDir := t.TempDir()
	
	tests := []struct {
		name         string
		modifyFiles  func(files map[string][]byte)
		errMsg       string
	}{
		{
			name: "invalid JSON in okra.json",
			modifyFiles: func(files map[string][]byte) {
				files["okra.json"] = []byte(`{invalid json`)
			},
			errMsg: "failed to parse config",
		},
		{
			name: "invalid JSON in service description",
			modifyFiles: func(files map[string][]byte) {
				files["service.description.json"] = []byte(`{invalid json`)
			},
			errMsg: "failed to parse schema",
		},
		{
			name: "truncated WASM file",
			modifyFiles: func(files map[string][]byte) {
				files["service.wasm"] = []byte{0x00, 0x61} // Incomplete magic number
			},
			errMsg: "invalid WASM file",
		},
		{
			name: "invalid protobuf descriptor",
			modifyFiles: func(files map[string][]byte) {
				files["service.pb.desc"] = []byte{0xFF, 0xFF, 0xFF, 0xFF} // Invalid protobuf
			},
			errMsg: "failed to load protobuf descriptors",
		},
		{
			name: "missing service in schema",
			modifyFiles: func(files map[string][]byte) {
				files["service.description.json"] = []byte(`{
					"meta": {"namespace": "test", "version": "v1"},
					"services": []
				}`)
			},
			errMsg: "failed to create service package", // NewServicePackage likely checks this
		},
		{
			name: "mismatched service name",
			modifyFiles: func(files map[string][]byte) {
				files["okra.json"] = []byte(`{
					"name": "ServiceA",
					"version": "1.0.0",
					"language": "go"
				}`)
				files["service.description.json"] = []byte(`{
					"meta": {"namespace": "test", "version": "v1"},
					"services": [{
						"name": "ServiceB",
						"methods": []
					}]
				}`)
			},
			errMsg: "service name mismatch",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create base files
			files := map[string][]byte{
				"service.wasm": {0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00},
				"service.description.json": []byte(`{
					"meta": {"namespace": "test", "version": "v1"},
					"services": [{
						"name": "TestService",
						"methods": [{"name": "TestMethod", "inputType": "TestInput", "outputType": "TestOutput"}]
					}]
				}`),
				"okra.json": []byte(`{
					"name": "TestService",
					"version": "1.0.0",
					"language": "go"
				}`),
				"service.pb.desc": []byte{0x0A, 0x00},
			}
			
			// Apply modifications
			tt.modifyFiles(files)
			
			// Create package
			packagePath := createCorruptedPackage(t, tempDir, tt.name, files)
			
			pkg, err := LoadPackage(ctx, "file://"+packagePath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Nil(t, pkg)
		})
	}
}

func TestLoadPackage_LargeFiles(t *testing.T) {
	// Test: LoadPackage handles large files appropriately
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}
	
	ctx := context.Background()
	tempDir := t.TempDir()
	
	// Create a large WASM file (10MB)
	largeWASM := make([]byte, 10*1024*1024)
	copy(largeWASM, []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00})
	
	files := map[string][]byte{
		"service.wasm": largeWASM,
		"service.description.json": []byte(`{
			"meta": {"namespace": "test", "version": "v1"},
			"services": [{
				"name": "TestService",
				"methods": [{"name": "TestMethod", "inputType": "TestInput", "outputType": "TestOutput"}]
			}]
		}`),
		"okra.json": []byte(`{
			"name": "TestService",
			"version": "1.0.0",
			"language": "go"
		}`),
		"service.pb.desc": []byte{0x0A, 0x00},
	}
	
	packagePath := createCorruptedPackage(t, tempDir, "large", files)
	
	// Should handle large files without panicking
	// May fail at WASM compilation, but should not crash
	pkg, err := LoadPackage(ctx, "file://"+packagePath)
	if err != nil {
		// Expected to fail at WASM compilation
		assert.Contains(t, err.Error(), "failed to compile WASM module")
	}
	_ = pkg
}

func TestExtractPackage_MaliciousArchive(t *testing.T) {
	// Test: extractPackage protects against path traversal attacks
	tempDir := t.TempDir()
	
	// Create a malicious archive with path traversal
	packagePath := filepath.Join(tempDir, "malicious.pkg")
	file, err := os.Create(packagePath)
	require.NoError(t, err)
	defer file.Close()
	
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	
	// Add file with path traversal attempt
	header := &tar.Header{
		Name: "../../../etc/passwd",
		Size: 10,
		Mode: 0644,
	}
	
	err = tarWriter.WriteHeader(header)
	require.NoError(t, err)
	
	_, err = tarWriter.Write([]byte("malicious"))
	require.NoError(t, err)
	
	// Extract should fail or sanitize the path
	extractDir := filepath.Join(tempDir, "extract")
	err = os.MkdirAll(extractDir, 0755)
	require.NoError(t, err)
	
	files, err := extractPackage(packagePath, extractDir)
	
	// Should either error or sanitize the path
	if err == nil {
		// If no error, verify the file was not extracted outside extractDir
		for _, path := range files {
			assert.True(t, strings.HasPrefix(path, extractDir), 
				"extracted file %s is outside extract directory", path)
		}
	}
}

func TestLoadPackage_ConcurrentLoads(t *testing.T) {
	// Test: Multiple concurrent package loads don't interfere
	ctx := context.Background()
	tempDir := t.TempDir()
	
	// Create multiple test packages
	var packagePaths []string
	for i := 0; i < 5; i++ {
		packagePath := createTestPackageWithName(t, tempDir, fmt.Sprintf("Service%d", i))
		packagePaths = append(packagePaths, packagePath)
	}
	
	// Load packages concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(packagePaths))
	
	for _, path := range packagePaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			_, err := LoadPackage(ctx, "file://"+p)
			if err != nil {
				errors <- err
			}
		}(path)
	}
	
	wg.Wait()
	close(errors)
	
	// All loads should complete without interference
	for err := range errors {
		// Expected to fail at WASM compilation, but not due to concurrency
		if err != nil {
			assert.Contains(t, err.Error(), "failed to compile WASM module")
		}
	}
}

// Helper functions for robustness tests

func compressData(data []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	return buf.Bytes()
}

func createCorruptedPackage(t *testing.T, dir, name string, files map[string][]byte) string {
	packagePath := filepath.Join(dir, name+".pkg")
	
	file, err := os.Create(packagePath)
	require.NoError(t, err)
	defer file.Close()
	
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		
		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)
		
		_, err = tarWriter.Write(content)
		require.NoError(t, err)
	}
	
	return packagePath
}

func createTestPackageWithName(t *testing.T, dir, serviceName string) string {
	packagePath := filepath.Join(dir, serviceName+".pkg")
	
	file, err := os.Create(packagePath)
	require.NoError(t, err)
	defer file.Close()
	
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	
	files := map[string][]byte{
		"service.wasm": {0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00},
		"service.description.json": []byte(fmt.Sprintf(`{
			"meta": {"namespace": "test", "version": "v1"},
			"services": [{
				"name": "%s",
				"methods": [{"name": "TestMethod", "inputType": "TestInput", "outputType": "TestOutput"}]
			}]
		}`, serviceName)),
		"okra.json": []byte(fmt.Sprintf(`{
			"name": "%s",
			"version": "1.0.0",
			"language": "go"
		}`, serviceName)),
		"service.pb.desc": []byte{0x0A, 0x00},
	}
	
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		
		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)
		
		_, err = tarWriter.Write(content)
		require.NoError(t, err)
	}
	
	return packagePath
}
