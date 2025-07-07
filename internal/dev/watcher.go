package dev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches files for changes based on patterns
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	patterns []string
	exclude  []string
	onChange func(path string, op fsnotify.Op)
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(patterns []string, exclude []string, onChange func(path string, op fsnotify.Op)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &FileWatcher{
		watcher:  watcher,
		patterns: patterns,
		exclude:  exclude,
		onChange: onChange,
	}, nil
}

// AddDirectory recursively adds a directory to the watcher
func (fw *FileWatcher) AddDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded paths
		for _, pattern := range fw.exclude {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Only watch directories
		if info.IsDir() {
			if err := fw.watcher.Add(path); err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", path, err)
			}
		}

		return nil
	})
}

// Start begins watching for file changes
func (fw *FileWatcher) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}

			// Check if file matches our patterns
			if fw.shouldWatch(event.Name) {
				fw.onChange(event.Name, event.Op)
			}

			// If a new directory is created, add it to the watcher
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					fw.AddDirectory(event.Name)
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher error channel closed")
			}
			if err != nil {
				// Log error but continue watching
				fmt.Printf("Watcher error: %v\n", err)
			}
		}
	}
}

// shouldWatch checks if a file should trigger a change event based on patterns
func (fw *FileWatcher) shouldWatch(path string) bool {
	base := filepath.Base(path)
	
	// Check excludes first
	for _, pattern := range fw.exclude {
		if matched, _ := filepath.Match(pattern, base); matched {
			return false
		}
	}

	// Check if file matches any watch pattern
	for _, pattern := range fw.patterns {
		// Handle ** for recursive matching
		if strings.Contains(pattern, "**") {
			// Simple implementation: check if the file extension matches
			if strings.HasPrefix(pattern, "**/*.") {
				ext := strings.TrimPrefix(pattern, "**/*")
				if strings.HasSuffix(path, ext) {
					return true
				}
			}
		} else if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}

// Close stops the watcher
func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}