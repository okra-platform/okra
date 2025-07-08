package dev

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileWatcher_shouldWatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		exclude  []string
		path     string
		want     bool
	}{
		{
			name:     "match go file",
			patterns: []string{"*.go"},
			exclude:  []string{},
			path:     "/project/main.go",
			want:     true,
		},
		{
			name:     "match nested go file with ** pattern",
			patterns: []string{"**/*.go"},
			exclude:  []string{},
			path:     "/project/internal/pkg/file.go",
			want:     true,
		},
		{
			name:     "exclude test file",
			patterns: []string{"*.go"},
			exclude:  []string{"*_test.go"},
			path:     "/project/main_test.go",
			want:     false,
		},
		{
			name:     "match graphql file",
			patterns: []string{"*.okra.gql", "**/*.okra.gql"},
			exclude:  []string{},
			path:     "/project/schema/service.okra.gql",
			want:     true,
		},
		{
			name:     "no match",
			patterns: []string{"*.go", "*.ts"},
			exclude:  []string{},
			path:     "/project/readme.md",
			want:     false,
		},
		{
			name:     "exclude overrides pattern",
			patterns: []string{"*.go"},
			exclude:  []string{"vendor.go"},
			path:     "/project/vendor.go",
			want:     false,
		},
		{
			name:     "match typescript file",
			patterns: []string{"*.ts", "**/*.ts"},
			exclude:  []string{"*.test.ts"},
			path:     "/project/src/index.ts",
			want:     true,
		},
		{
			name:     "exclude typescript test",
			patterns: []string{"*.ts", "**/*.ts"},
			exclude:  []string{"*.test.ts"},
			path:     "/project/src/index.test.ts",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw := &FileWatcher{
				patterns: tt.patterns,
				exclude:  tt.exclude,
			}
			
			got := fw.shouldWatch(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileWatcher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	
	// Create a test directory structure
	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)
	
	// Track events
	var events []struct {
		path string
		op   fsnotify.Op
	}
	var eventsMu sync.Mutex
	
	onChange := func(path string, op fsnotify.Op) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, struct {
			path string
			op   fsnotify.Op
		}{path: path, op: op})
	}
	
	// Create watcher
	fw, err := NewFileWatcher(
		[]string{"*.go", "**/*.go", "*.okra.gql"},
		[]string{"*_test.go", "vendor/"},
		onChange,
	)
	require.NoError(t, err)
	defer fw.Close()
	
	// Add directory to watch
	err = fw.AddDirectory(tmpDir)
	require.NoError(t, err)
	
	// Start watching in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	errChan := make(chan error, 1)
	go func() {
		errChan <- fw.Start(ctx)
	}()
	
	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test 1: Create a Go file (should trigger)
	goFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main"), 0644)
	require.NoError(t, err)
	
	// Test 2: Create a test file (should not trigger)
	testFile := filepath.Join(tmpDir, "main_test.go")
	err = os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)
	
	// Test 3: Create a GraphQL file (should trigger)
	graphqlFile := filepath.Join(tmpDir, "service.okra.gql")
	err = os.WriteFile(graphqlFile, []byte("type Query { hello: String }"), 0644)
	require.NoError(t, err)
	
	// Test 4: Create a file in src directory (should trigger)
	srcFile := filepath.Join(srcDir, "handler.go")
	err = os.WriteFile(srcFile, []byte("package src"), 0644)
	require.NoError(t, err)
	
	// Test 5: Create a vendor directory and file (should not trigger)
	vendorDir := filepath.Join(tmpDir, "vendor")
	err = os.MkdirAll(vendorDir, 0755)
	require.NoError(t, err)
	
	vendorFile := filepath.Join(vendorDir, "lib.go")
	err = os.WriteFile(vendorFile, []byte("package vendor"), 0644)
	require.NoError(t, err)
	
	// Give time for events to be processed
	time.Sleep(200 * time.Millisecond)
	
	// Check events
	eventsMu.Lock()
	defer eventsMu.Unlock()
	
	// Should have events for: main.go, service.okra.gql, handler.go
	assert.GreaterOrEqual(t, len(events), 3, "Expected at least 3 events")
	
	// Verify expected files triggered events
	fileNames := make(map[string]bool)
	for _, e := range events {
		fileNames[filepath.Base(e.path)] = true
	}
	
	assert.True(t, fileNames["main.go"], "Expected event for main.go")
	assert.True(t, fileNames["service.okra.gql"], "Expected event for service.okra.gql")
	assert.True(t, fileNames["handler.go"], "Expected event for handler.go")
	assert.False(t, fileNames["main_test.go"], "Should not have event for main_test.go")
	assert.False(t, fileNames["lib.go"], "Should not have event for vendor/lib.go")
}

func TestFileWatcher_AddDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create directory structure with excludes
	dirs := []string{
		"src",
		"src/internal",
		"vendor",
		"node_modules",
		".git",
	}
	
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		require.NoError(t, err)
	}
	
	fw, err := NewFileWatcher(
		[]string{"*.go"},
		[]string{"vendor", "node_modules", ".git"},
		func(string, fsnotify.Op) {},
	)
	require.NoError(t, err)
	defer fw.Close()
	
	// Add root directory
	err = fw.AddDirectory(tmpDir)
	require.NoError(t, err)
	
	// Verify that excluded directories were not added
	// This is a bit tricky to test directly, but we can verify
	// by checking that creating files in excluded dirs doesn't trigger events
}

func TestFileWatcher_Close(t *testing.T) {
	fw, err := NewFileWatcher(
		[]string{"*.go"},
		[]string{},
		func(string, fsnotify.Op) {},
	)
	require.NoError(t, err)
	
	// Close should not error
	err = fw.Close()
	assert.NoError(t, err)
	
	// Double close should also be safe
	err = fw.Close()
	assert.NoError(t, err)
}