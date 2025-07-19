package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// WASMCache provides cached WASM compilation for tests
type WASMCache struct {
	mu       sync.RWMutex
	cacheDir string
	cache    map[string][]byte
}

// NewWASMCache creates a new WASM cache
func NewWASMCache(t *testing.T) *WASMCache {
	t.Helper()
	
	cacheDir := filepath.Join(t.TempDir(), "wasm-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	
	return &WASMCache{
		cacheDir: cacheDir,
		cache:    make(map[string][]byte),
	}
}

// GetOrCompile returns cached WASM bytes or compiles if not cached
func (c *WASMCache) GetOrCompile(t *testing.T, sourceDir string, compileFunc func() ([]byte, error)) ([]byte, error) {
	t.Helper()
	
	// Generate cache key from source directory contents
	key, err := c.generateCacheKey(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cache key: %w", err)
	}
	
	// Check memory cache first
	c.mu.RLock()
	if data, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()
	
	// Check disk cache
	cachePath := filepath.Join(c.cacheDir, key+".wasm")
	if data, err := os.ReadFile(cachePath); err == nil {
		c.mu.Lock()
		c.cache[key] = data
		c.mu.Unlock()
		return data, nil
	}
	
	// Compile and cache
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Double-check in case another goroutine compiled while we waited
	if data, ok := c.cache[key]; ok {
		return data, nil
	}
	
	data, err := compileFunc()
	if err != nil {
		return nil, err
	}
	
	// Save to cache
	c.cache[key] = data
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Logf("Warning: failed to write to disk cache: %v", err)
	}
	
	return data, nil
}

// generateCacheKey creates a hash of all source files in a directory
func (c *WASMCache) generateCacheKey(sourceDir string) (string, error) {
	h := sha256.New()
	
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories and non-source files
		if info.IsDir() || !isSourceFile(path) {
			return nil
		}
		
		// Hash file path and contents
		fmt.Fprintf(h, "%s\n", path)
		
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		
		return nil
	})
	
	if err != nil {
		return "", err
	}
	
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".gql"
}