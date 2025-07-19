package serve

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
)

// LoadPackage loads a package from a file:// or s3:// URL
func LoadPackage(ctx context.Context, source string) (*runtime.ServicePackage, error) {
	// Parse source URL
	sourceURL, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid source URL: %w", err)
	}

	// Create temp directory for extraction
	tempDir, err := os.MkdirTemp("", "okra-package-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download or copy package to temp file
	var packagePath string
	switch sourceURL.Scheme {
	case "file":
		// Local file path
		packagePath = sourceURL.Path
		if _, err := os.Stat(packagePath); err != nil {
			return nil, fmt.Errorf("package file not found: %w", err)
		}
	case "s3":
		// Download from S3
		packagePath, err = downloadFromS3(ctx, sourceURL, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to download from S3: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported source scheme: %s", sourceURL.Scheme)
	}

	// Extract package
	extractedFiles, err := extractPackage(packagePath, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract package: %w", err)
	}

	// Validate required files
	if err := validatePackageFiles(extractedFiles); err != nil {
		return nil, fmt.Errorf("invalid package: %w", err)
	}

	// Load package components
	return loadPackageComponents(extractedFiles)
}

// extractPackage extracts a tar.gz package to the specified directory
func extractPackage(packagePath, destDir string) (map[string]string, error) {
	file, err := os.Open(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	extractedFiles := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Sanitize file name
		name := filepath.Clean(header.Name)
		if strings.Contains(name, "..") {
			return nil, fmt.Errorf("invalid file name in package: %s", header.Name)
		}

		// Create destination file
		destPath := filepath.Join(destDir, name)
		destFile, err := os.Create(destPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", name, err)
		}

		// Copy file contents
		if _, err := io.Copy(destFile, tarReader); err != nil {
			destFile.Close()
			return nil, fmt.Errorf("failed to extract file %s: %w", name, err)
		}
		destFile.Close()

		extractedFiles[name] = destPath
	}

	return extractedFiles, nil
}

// validatePackageFiles ensures all required files are present
func validatePackageFiles(files map[string]string) error {
	requiredFiles := []string{
		"service.wasm",
		"service.description.json",
		"okra.json",
		"service.pb.desc",
	}

	for _, required := range requiredFiles {
		if _, ok := files[required]; !ok {
			return fmt.Errorf("missing required file: %s", required)
		}
	}

	// Validate WASM file has correct magic number
	wasmPath := files["service.wasm"]
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to read WASM file: %w", err)
	}

	// WASM magic number: 0x00 0x61 0x73 0x6D (asm)
	if len(wasmData) < 4 || wasmData[0] != 0x00 || wasmData[1] != 0x61 ||
		wasmData[2] != 0x73 || wasmData[3] != 0x6D {
		return fmt.Errorf("invalid WASM file: incorrect magic number")
	}

	return nil
}

// loadPackageComponents loads all components from extracted files
func loadPackageComponents(files map[string]string) (*runtime.ServicePackage, error) {
	// Load config
	configData, err := os.ReadFile(files["okra.json"])
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg config.Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Load schema
	schemaData, err := os.ReadFile(files["service.description.json"])
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	var sch schema.Schema
	if err := json.Unmarshal(schemaData, &sch); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Validate config matches schema
	if len(sch.Services) > 0 && sch.Services[0].Name != cfg.Name {
		return nil, fmt.Errorf("service name mismatch: config has '%s', schema has '%s'",
			cfg.Name, sch.Services[0].Name)
	}

	// Load WASM module
	wasmBytes, err := os.ReadFile(files["service.wasm"])
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM file: %w", err)
	}

	wasmModule, err := wasm.NewWASMCompiledModule(context.Background(), wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Create service package
	pkg, err := runtime.NewServicePackage(wasmModule, &sch, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create service package: %w", err)
	}

	// Load protobuf descriptors
	fds, err := runtime.LoadFileDescriptors(files["service.pb.desc"])
	if err != nil {
		return nil, fmt.Errorf("failed to load protobuf descriptors: %w", err)
	}

	return pkg.WithFileDescriptors(fds), nil
}

// downloadFromS3 downloads a package from S3 to a temp file
func downloadFromS3(ctx context.Context, s3URL *url.URL, tempDir string) (string, error) {
	// For now, we'll use a simple HTTP GET with presigned URLs
	// In production, this should use the AWS SDK

	// Convert s3:// URL to https:// presigned URL
	// This is a simplified implementation - real implementation would use AWS SDK
	httpsURL := fmt.Sprintf("https://%s.s3.amazonaws.com%s", s3URL.Host, s3URL.Path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temp file
	tempFile, err := os.CreateTemp(tempDir, "package-*.pkg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy response to file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save package: %w", err)
	}

	return tempFile.Name(), nil
}
