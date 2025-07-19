package dev

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GoPackageInfo contains information about a Go package
type GoPackageInfo struct {
	PackageName     string
	HasConstructor  bool
	ConstructorName string
}

// GoParser parses Go source files
type GoParser struct {
	packagePattern     *regexp.Regexp
	constructorPattern *regexp.Regexp
}

// NewGoParser creates a new Go parser
func NewGoParser() *GoParser {
	return &GoParser{
		packagePattern:     regexp.MustCompile(`^\s*package\s+(\w+)`),
		constructorPattern: regexp.MustCompile(`^\s*func\s+(New\w*)\s*\(`),
	}
}

// ParsePackageInfo parses Go files in a directory to extract package information
func (p *GoParser) ParsePackageInfo(dir string) (*GoPackageInfo, error) {
	info := &GoPackageInfo{}

	// Find all .go files (excluding test files and generated files)
	goFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("failed to find Go files: %w", err)
	}

	foundPackage := false

	for _, file := range goFiles {
		// Skip test files and generated files
		base := filepath.Base(file)
		if strings.HasSuffix(base, "_test.go") ||
			strings.Contains(base, ".gen.") ||
			strings.Contains(base, "_gen.") ||
			base == "interface.go" {
			continue
		}

		// Parse the file
		fileInfo, err := p.parseGoFile(file)
		if err != nil {
			continue // Skip files we can't parse
		}

		// Set package name from first valid file
		if !foundPackage && fileInfo.PackageName != "" && fileInfo.PackageName != "main" {
			info.PackageName = fileInfo.PackageName
			foundPackage = true
		}

		// Look for constructor
		if fileInfo.HasConstructor && !info.HasConstructor {
			info.HasConstructor = true
			info.ConstructorName = fileInfo.ConstructorName
		}
	}

	if !foundPackage {
		return nil, fmt.Errorf("no valid Go package found in %s", dir)
	}

	// Default constructor name if not found
	if !info.HasConstructor {
		info.ConstructorName = "NewService"
	}

	return info, nil
}

// parseGoFile parses a single Go file
func (p *GoParser) parseGoFile(filepath string) (*GoPackageInfo, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &GoPackageInfo{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Look for package declaration
		if matches := p.packagePattern.FindStringSubmatch(line); len(matches) > 1 {
			info.PackageName = matches[1]
		}

		// Look for constructor functions (NewService, NewXService, etc.)
		if matches := p.constructorPattern.FindStringSubmatch(line); len(matches) > 1 {
			constructorName := matches[1]
			// Prefer NewService or New{ServiceName} patterns
			if constructorName == "NewService" ||
				strings.HasPrefix(constructorName, "New") && strings.Contains(constructorName, "Service") {
				info.HasConstructor = true
				info.ConstructorName = constructorName
				break // Found preferred constructor
			} else if !info.HasConstructor {
				// Use first constructor found as fallback
				info.HasConstructor = true
				info.ConstructorName = constructorName
			}
		}
	}

	return info, scanner.Err()
}

// ExtractModulePath extracts the module path from go.mod
func (p *GoParser) ExtractModulePath(projectRoot string) (string, error) {
	goModPath := filepath.Join(projectRoot, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	modulePattern := regexp.MustCompile(`^\s*module\s+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := modulePattern.FindStringSubmatch(line); len(matches) > 1 {
			return strings.TrimSpace(matches[1]), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}
