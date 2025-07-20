package dev

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for GoParser:
// 1. Test NewGoParser creates parser with correct patterns
// 2. Test ParsePackageInfo with valid Go package
// 3. Test ParsePackageInfo with multiple Go files
// 4. Test ParsePackageInfo skips test files
// 5. Test ParsePackageInfo skips generated files
// 6. Test ParsePackageInfo finds constructor
// 7. Test ParsePackageInfo with no Go files
// 8. Test ParsePackageInfo with main package
// 9. Test parseGoFile with valid file
// 10. Test parseGoFile with comments
// 11. Test ExtractModulePath with valid go.mod
// 12. Test ExtractModulePath with missing go.mod

func TestNewGoParser(t *testing.T) {
	// Test: NewGoParser creates parser with correct patterns
	parser := NewGoParser()
	
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.packagePattern)
	assert.NotNil(t, parser.constructorPattern)
	
	// Test package pattern
	matches := parser.packagePattern.FindStringSubmatch("package mypackage")
	assert.Equal(t, []string{"package mypackage", "mypackage"}, matches)
	
	// Test constructor pattern
	matches = parser.constructorPattern.FindStringSubmatch("func NewService() *Service {")
	assert.Equal(t, []string{"func NewService(", "NewService"}, matches)
}

func TestGoParser_ParsePackageInfo_ValidPackage(t *testing.T) {
	// Test: ParsePackageInfo with valid Go package
	
	tmpDir := t.TempDir()
	
	// Create a simple Go file
	serviceContent := `package myservice

import "fmt"

type Service struct {
	name string
}

func NewService() *Service {
	return &Service{name: "test"}
}

func (s *Service) Greet() {
	fmt.Println("Hello from", s.name)
}`
	
	err := os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(serviceContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "myservice", info.PackageName)
	assert.True(t, info.HasConstructor)
	assert.Equal(t, "NewService", info.ConstructorName)
}

func TestGoParser_ParsePackageInfo_MultipleFiles(t *testing.T) {
	// Test: ParsePackageInfo with multiple Go files
	
	tmpDir := t.TempDir()
	
	// Create first file without constructor
	file1Content := `package mypackage

type User struct {
	Name string
}`
	err := os.WriteFile(filepath.Join(tmpDir, "user.go"), []byte(file1Content), 0644)
	require.NoError(t, err)
	
	// Create second file with constructor
	file2Content := `package mypackage

func NewUserService() *UserService {
	return &UserService{}
}

type UserService struct {
	users []User
}`
	err = os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(file2Content), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.NoError(t, err)
	assert.Equal(t, "mypackage", info.PackageName)
	assert.True(t, info.HasConstructor)
	assert.Equal(t, "NewUserService", info.ConstructorName)
}

func TestGoParser_ParsePackageInfo_SkipsTestFiles(t *testing.T) {
	// Test: ParsePackageInfo skips test files
	
	tmpDir := t.TempDir()
	
	// Create test file
	testContent := `package mypackage

import "testing"

func TestSomething(t *testing.T) {
	// test code
}`
	err := os.WriteFile(filepath.Join(tmpDir, "service_test.go"), []byte(testContent), 0644)
	require.NoError(t, err)
	
	// Create regular file
	serviceContent := `package mypackage

type Service struct{}`
	err = os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(serviceContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.NoError(t, err)
	assert.Equal(t, "mypackage", info.PackageName)
	assert.False(t, info.HasConstructor)
	assert.Equal(t, "NewService", info.ConstructorName) // Default
}

func TestGoParser_ParsePackageInfo_SkipsGeneratedFiles(t *testing.T) {
	// Test: ParsePackageInfo skips generated files
	
	tmpDir := t.TempDir()
	
	// Create generated files that should be skipped
	genContent := `package mypackage

func NewGenerated() *Generated {
	return &Generated{}
}`
	err := os.WriteFile(filepath.Join(tmpDir, "service.gen.go"), []byte(genContent), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(tmpDir, "service_gen.go"), []byte(genContent), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(tmpDir, "interface.go"), []byte(genContent), 0644)
	require.NoError(t, err)
	
	// Create regular file
	serviceContent := `package mypackage

type Service struct{}`
	err = os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(serviceContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.NoError(t, err)
	assert.Equal(t, "mypackage", info.PackageName)
	assert.False(t, info.HasConstructor) // Should not find constructor in generated files
}

func TestGoParser_ParsePackageInfo_NoGoFiles(t *testing.T) {
	// Test: ParsePackageInfo with no Go files
	
	tmpDir := t.TempDir()
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "no valid Go package found")
}

func TestGoParser_ParsePackageInfo_MainPackage(t *testing.T) {
	// Test: ParsePackageInfo skips main package
	
	tmpDir := t.TempDir()
	
	// Create main package file
	mainContent := `package main

func main() {
	println("Hello")
}`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	_, err = parser.ParsePackageInfo(tmpDir)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid Go package found")
}

func TestGoParser_ParsePackageInfo_PreferredConstructor(t *testing.T) {
	// Test: ParsePackageInfo prefers NewService over other constructors
	
	tmpDir := t.TempDir()
	
	// Create file with multiple constructors
	content := `package myservice

func NewClient() *Client {
	return &Client{}
}

func NewService() *Service {
	return &Service{}
}

func NewMyService() *Service {
	return &Service{}
}`
	err := os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(content), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.ParsePackageInfo(tmpDir)
	
	assert.NoError(t, err)
	assert.True(t, info.HasConstructor)
	assert.Equal(t, "NewService", info.ConstructorName) // Should prefer NewService
}

func TestGoParser_parseGoFile_WithComments(t *testing.T) {
	// Test: parseGoFile handles comments correctly
	
	tmpDir := t.TempDir()
	
	content := `// Package myservice provides a service
package myservice

// This is a comment
/* This is a 
   multiline comment */

// NewService creates a new service
func NewService() *Service {
	return &Service{}
}`
	
	filepath := filepath.Join(tmpDir, "service.go")
	err := os.WriteFile(filepath, []byte(content), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	info, err := parser.parseGoFile(filepath)
	
	assert.NoError(t, err)
	assert.Equal(t, "myservice", info.PackageName)
	assert.True(t, info.HasConstructor)
	assert.Equal(t, "NewService", info.ConstructorName)
}

func TestGoParser_parseGoFile_InvalidFile(t *testing.T) {
	// Test: parseGoFile with non-existent file
	
	parser := NewGoParser()
	info, err := parser.parseGoFile("/non/existent/file.go")
	
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestGoParser_ExtractModulePath_Valid(t *testing.T) {
	// Test: ExtractModulePath with valid go.mod
	
	tmpDir := t.TempDir()
	
	goModContent := `module github.com/example/myproject

go 1.21

require (
	github.com/stretchr/testify v1.8.0
)`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	modulePath, err := parser.ExtractModulePath(tmpDir)
	
	assert.NoError(t, err)
	assert.Equal(t, "github.com/example/myproject", modulePath)
}

func TestGoParser_ExtractModulePath_WithWhitespace(t *testing.T) {
	// Test: ExtractModulePath handles whitespace
	
	tmpDir := t.TempDir()
	
	goModContent := `   module   github.com/example/myproject   

go 1.21`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	modulePath, err := parser.ExtractModulePath(tmpDir)
	
	assert.NoError(t, err)
	assert.Equal(t, "github.com/example/myproject", modulePath)
}

func TestGoParser_ExtractModulePath_MissingGoMod(t *testing.T) {
	// Test: ExtractModulePath with missing go.mod
	
	tmpDir := t.TempDir()
	
	parser := NewGoParser()
	modulePath, err := parser.ExtractModulePath(tmpDir)
	
	assert.Error(t, err)
	assert.Empty(t, modulePath)
	assert.Contains(t, err.Error(), "failed to open go.mod")
}

func TestGoParser_ExtractModulePath_NoModuleDeclaration(t *testing.T) {
	// Test: ExtractModulePath with go.mod without module declaration
	
	tmpDir := t.TempDir()
	
	goModContent := `go 1.21

require (
	github.com/stretchr/testify v1.8.0
)`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)
	
	parser := NewGoParser()
	modulePath, err := parser.ExtractModulePath(tmpDir)
	
	assert.Error(t, err)
	assert.Empty(t, modulePath)
	assert.Contains(t, err.Error(), "module declaration not found")
}